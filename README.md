# pidlot

A full-screen process manager with a command prompt underneath.
Built as a terminal UI (Go + tview) using gopsutil.

Runs on **Windows, macOS and Linux** — the code has no platform-specific
dependencies and cross-compiles cleanly to `windows`, `darwin` and `linux`
(amd64/arm64).

## Build

Use the build script for your shell — it builds the full matrix and exports
the binaries into platform-specific folders in `./build` (gitignored,
created automatically at build time):

```
./build.ps1                # Windows (PowerShell)
./build.sh                 # macOS / Linux / WSL
./build.sh darwin/arm64    # a single target only
```

Resulting layout:

```
build/
├── win/     pidlot-windows-amd64.exe,  pidlot-windows-arm64.exe
├── darwin/  pidlot-darwin-amd64,      pidlot-darwin-arm64
└── linux/   pidlot-linux-amd64,       pidlot-linux-arm64
```

A plain `go build -o pidlot .` for the host platform works too.

> On macOS/Linux, run under `sudo` to see ports and paths of processes
owned by other users.

## Overview

The top of the screen is a live table of every running process, refreshed
every 2 seconds and sorted by CPU usage:

| Column | Meaning                                             |
|--------|-----------------------------------------------------|
| PID    | process id                                          |
| PPID   | parent process id                                   |
| NAME   | executable name                                      |
| CPU %  | percent of a single core (can exceed 100 on multicore) |
| RAM    | resident memory                                      |
| PORTS  | listening TCP / bound UDP ports                      |
| PATH   | full executable path                                 |

Below the table is a status line (view, counts, last command result) and a
command prompt. Type a command and press Enter.

## Commands

| Command                    | Effect                                                        |
|----------------------------|---------------------------------------------------------------|
| `kill <pid> [pid...]`      | terminate process(es)                                         |
| `child <pid>`              | show the child processes of the given pid                    |
| `filter <text>`            | show only processes whose name/path/pid contains the text     |
| `port <n>`                 | show only processes using that port                          |
| `export txt\|csv\md [file]`| write the currently displayed rows to a file (default `processes-<timestamp>.<ext>`) |
| `reset`                    | clear filters / child view, back to all processes             |
| `refresh`                  | refresh immediately                                            |
| `help`                     | show the command list                                          |
| `quit`                     | exit                                                           |

### Keys

- `Enter` executes the command in the prompt
- `Up`/`Down`/`PgUp`/`PgDn`/`Home`/`End` scroll the process table
- `Ctrl+P` / `Ctrl+N` browse command history
- `Ctrl+L` force refresh, `Ctrl+C` quit

## Notes

- Some columns (path, ports) may be unavailable for processes running at a
  higher privilege level unless pidlot itself is run elevated.
- `filter` and `port` are exclusive with the `child` view; issuing one clears
  the other.
