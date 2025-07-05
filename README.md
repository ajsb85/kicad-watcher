# KiCad Watcher

A simple, efficient file watcher that automatically converts KiCad schematic files (`.kicad_sch`) to SVG format upon modification.

This tool is designed to be run as a background service (e.g., using `systemd` on Linux) to streamline the workflow for KiCad users who need up-to-date SVG exports of their schematics.

## Features

- **Automatic Conversion**: Monitors a specified directory for changes to `.kicad_sch` files.
- **Concurrent Processing**: Each file conversion is handled in a separate goroutine, allowing multiple files to be processed concurrently without blocking the main watcher.
- **Robust Execution**: Includes a timeout for the `kicad-cli` command to prevent stuck processes.
- **Structured Logging**: Outputs logs in a key-value format, ideal for `systemd`'s journal or other log aggregation tools.
- **Detailed Reporting**: Logs execution time, CPU usage, and memory consumption for each conversion task.

## Prerequisites

- Go (version 1.18 or newer)
- KiCad (version 6 or newer) with `kicad-cli` installed and available at `/usr/bin/kicad-cli`.

## Installation

1. **Clone the repository:**

    ```bash
    git clone [https://github.com/ajsb85/kicad-watcher.git](https://github.com/ajsb85/kicad-watcher.git)
    cd kicad-watcher
    ```

2. **Build the binary:**

    ```bash
    go build -o kicad-watcher
    ```

3. **Move the binary to a standard location (optional but recommended):**

    ```bash
    sudo mv kicad-watcher /usr/local/bin/
    ```

## Usage

Run the watcher from your terminal, providing the path to the directory you want to monitor:

```bash
kicad-watcher /path/to/your/kicad/projects
```
