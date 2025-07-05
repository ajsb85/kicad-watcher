package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// job represents a file path that needs to be processed.
type job string

// worker reads file paths from a jobs channel and executes the KiCad CLI.
// It receives the base output directory to pass along to the command runner.
func worker(id int, jobs <-chan job, outputDir string) {
	for filePath := range jobs {
		log.Printf("msg=\"worker starting\" worker_id=%d path=\"%s\"", id, filePath)
		runKiCadCLI(string(filePath), outputDir)
	}
}

func main() {
	// Use a structured, key-value format for logs and rely on systemd for timestamps.
	log.SetFlags(0)

	// Check if a folder path is provided
	if len(os.Args) < 2 {
		log.Fatalf("msg=\"initialization error\" error=\"folder path not provided\" usage=\"%s <folder_to_watch>\"", os.Args[0])
	}
	folderToWatch := os.Args[1]

	// --- Concurrency and Debouncing/Cooldown Setup ---
	jobs := make(chan job, 100)
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	// Pass the root watch folder to each worker.
	for w := 1; w <= numWorkers; w++ {
		go worker(w, jobs, folderToWatch)
	}
	log.Printf("msg=\"worker pool started\" workers=%d", numWorkers)

	// State for managing event processing
	const debounceDuration = 250 * time.Millisecond
	const cooldownDuration = 2 * time.Second
	var processingMutex sync.Mutex
	debounceTimers := make(map[string]*time.Timer)
	onCooldown := make(map[string]bool)
	// --- End Setup ---

	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("msg=\"initialization error\" error=\"%v\"", err)
	}
	defer watcher.Close()

	// Start the main event processing loop
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				ext := filepath.Ext(event.Name)
				isSch := ext == ".kicad_sch"
				isSvg := ext == ".svg"

				if isSch || isSvg {
					log.Printf("msg=\"file event\" path=\"%s\" op=\"%s\"", event.Name, event.Op)
					if event.Op&fsnotify.Write == fsnotify.Write && isSch {
						// --- Debouncing and Cooldown Logic ---
						processingMutex.Lock()

						// If file is on cooldown, ignore the event.
						if onCooldown[event.Name] {
							processingMutex.Unlock()
							log.Printf("msg=\"event ignored\" path=\"%s\" reason=\"cooldown\"", event.Name)
							continue
						}

						// If a debounce timer already exists, reset it.
						if timer, exists := debounceTimers[event.Name]; exists {
							timer.Reset(debounceDuration)
						} else {
							// Otherwise, create a new timer.
							debounceTimers[event.Name] = time.AfterFunc(debounceDuration, func() {
								// Timer fired. Put file on cooldown before processing.
								processingMutex.Lock()
								onCooldown[event.Name] = true
								delete(debounceTimers, event.Name)
								processingMutex.Unlock()

								// Schedule the end of the cooldown period.
								time.AfterFunc(cooldownDuration, func() {
									processingMutex.Lock()
									delete(onCooldown, event.Name)
									processingMutex.Unlock()
									log.Printf("msg=\"cooldown ended\" path=\"%s\"", event.Name)
								})

								// Send the job to a worker.
								select {
								case jobs <- job(event.Name):
									// Job successfully sent
								default:
									log.Printf("msg=\"job queue full\" path=\"%s\" error=\"could not queue job for processing\"", event.Name)
									// If queue is full, remove from cooldown immediately so it can be retried.
									processingMutex.Lock()
									delete(onCooldown, event.Name)
									processingMutex.Unlock()
								}
							})
						}
						processingMutex.Unlock()
						// --- End Debouncing and Cooldown Logic ---
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("msg=\"watcher error\" error=\"%v\"", err)
			}
		}
	}()

	// Add the folder to the watcher
	err = watcher.Add(folderToWatch)
	if err != nil {
		log.Fatalf("msg=\"initialization error\" error=\"failed to add folder to watcher: %v\"", err)
	}
	log.Printf("msg=\"service started\" watching=\"%s\"", folderToWatch)

	// --- Graceful Shutdown Setup ---
	// Create a channel to receive OS signals.
	quit := make(chan os.Signal, 1)
	// Notify this channel on SIGINT (Ctrl+C) or SIGTERM (sent by systemd on stop/restart).
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received.
	sig := <-quit
	log.Printf("msg=\"service stopping\" signal=\"%v\"", sig)
	// The deferred watcher.Close() will now be called, and the program will exit cleanly.
	// --- End Graceful Shutdown ---
}

// runKiCadCLI executes the kicad-cli command and logs detailed execution statistics.
func runKiCadCLI(filePath string, outputDir string) {
	startTime := time.Now()

	// Create a new context with a 30-second timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel() // Ensures the context is canceled to free resources.

	// Determine output path. The SVG will have the same base name as the schematic,
	// but will be placed in the root folder being watched.
	baseName := filepath.Base(filePath)
	outputFile := filepath.Join(outputDir, strings.TrimSuffix(baseName, filepath.Ext(baseName))+".svg")

	// Attempt to remove the old SVG file before generating a new one.
	if err := os.Remove(outputFile); err != nil {
		// If the file doesn't exist, that's fine. For other errors, log a warning but continue.
		if !os.IsNotExist(err) {
			log.Printf("msg=\"pre-generation cleanup warning\" path=\"%s\" error=\"failed to remove old svg: %v\"", outputFile, err)
		}
	} else {
		log.Printf("msg=\"pre-generation cleanup\" path=\"%s\" status=\"removed old svg\"", outputFile)
	}

	// The command to execute, with the output directory specified.
	//cmd := exec.CommandContext(ctx, "/usr/bin/kicad-cli", "sch", "export", "svg", filePath, "-b", "-e", "--output", outputDir)
	cmd := exec.CommandContext(ctx, "/usr/bin/kicad-cli", "sch", "export", "svg", filePath, "--output", outputDir)

	// Execute the command and capture the combined output.
	output, runErr := cmd.CombinedOutput()
	outputStr := string(output)

	// Gather execution stats
	duration := time.Since(startTime)
	result := "SUCCESS"
	if runErr != nil {
		// Check for specific, known error messages from the tool's output.
		switch {
		case strings.Contains(outputStr, "Failed to load schematic"):
			result = "FAILED (invalid schematic)"
		case strings.Contains(outputStr, "Unable to load library"):
			result = "FAILED (library error)"
		case strings.Contains(outputStr, "Error loading drawing sheet"):
			result = "FAILED (drawing sheet error)"
		case strings.Contains(outputStr, "schematic has annotation errors"):
			result = "FAILED (annotation error)"
		case strings.Contains(outputStr, "Failed to create output directory"),
			strings.Contains(outputStr, "Unable to open destination"):
			result = "FAILED (filesystem error)"
		default:
			result = "FAILED"
		}
		// Generate an SVG with the error message.
		generateErrorSVG(outputFile, result)
	}

	var userCPUTime, sysCPUTime, maxRSS string
	if cmd.ProcessState != nil {
		userCPUTime = cmd.ProcessState.UserTime().String()
		sysCPUTime = cmd.ProcessState.SystemTime().String()
		if usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage); ok {
			maxRSS = fmt.Sprintf("%d", usage.Maxrss) // in kilobytes
		} else {
			maxRSS = "N/A"
		}
	} else {
		userCPUTime, sysCPUTime, maxRSS = "N/A", "N/A", "N/A"
	}

	// Sanitize output for single-line logging
	sanitizedOutput := strings.TrimSpace(outputStr)
	sanitizedOutput = strings.ReplaceAll(sanitizedOutput, "\n", "\\n")

	// Log the final report as a single structured line.
	log.Printf(
		"msg=\"command finished\" command=\"kicad-cli\" path=\"%s\" result=\"%s\" duration_ms=\"%d\" user_cpu=\"%s\" sys_cpu=\"%s\" mem_rss_kb=\"%s\" error=\"%v\" output=\"%s\"",
		filePath,
		result,
		duration.Milliseconds(),
		userCPUTime,
		sysCPUTime,
		maxRSS,
		runErr,
		sanitizedOutput,
	)
}

// generateErrorSVG creates an SVG file at the given path to display an error message.
func generateErrorSVG(filePath, errorText string) {
	// SVG template with a warning icon and placeholder for text.
	// NOTE: All literal '%' characters must be escaped as '%%'.
	svgTemplate := `
<svg width="600" height="300" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 300">
    <rect width="100%%" height="100%%" fill="#f5f4ef"/>
    <path d="M300,50 L350,150 L250,150 Z" fill="#D9534F"/>
    <path d="M295,100 L305,100 L305,130 L295,130 Z" fill="#FFFFFF"/>
    <path d="M295,140 L305,140 L305,150 L295,150 Z" fill="#FFFFFF"/>
    <text x="50%%" y="200" font-family="Arial, sans-serif" font-size="18" fill="#333" text-anchor="middle">
        %s
    </text>
</svg>
`
	// Simple text wrapping logic to split the error message into multiple lines.
	var wrappedTextBuilder strings.Builder
	words := strings.Split(errorText, " ")
	line := ""
	for _, word := range words {
		// Check if adding the next word exceeds the approximate line length.
		if len(line)+len(word) > 40 {
			wrappedTextBuilder.WriteString(fmt.Sprintf(`<tspan x="50%%" dy="1.2em">%s</tspan>`, line))
			line = ""
		}
		line += word + " "
	}
	// Add the last line.
	wrappedTextBuilder.WriteString(fmt.Sprintf(`<tspan x="50%%" dy="1.2em">%s</tspan>`, strings.TrimSpace(line)))

	// Populate the SVG template with the wrapped text.
	svgContent := fmt.Sprintf(svgTemplate, wrappedTextBuilder.String())

	// Write the content to the specified file.
	err := os.WriteFile(filePath, []byte(svgContent), 0644)
	if err != nil {
		log.Printf("msg=\"failed to generate error svg\" path=\"%s\" error=\"%v\"", filePath, err)
	} else {
		log.Printf("msg=\"generated error svg\" path=\"%s\"", filePath)
	}
}
