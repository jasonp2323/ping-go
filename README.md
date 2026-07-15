# ping-go

A small cross-platform CLI for pinging one or more hosts, with optional
concurrency, duration-based runs, file logging, and the ability to detach
itself into the background — no `nohup`, no `Start-Process`, no Windows
service required.

Built primarily for Windows (PowerShell), but works on Linux and macOS too.

## Features

- Ping one host or several, comma-separated
- Run hosts sequentially or **concurrently**
- Fixed echo count, or ping continuously for a **duration**
- Per-ping **timeout** so a hung/unreachable host can't block forever
- Optional **screen output**, optional **file logging**, or both
- Optional self-**daemonizing**: detaches into the background and returns your prompt immediately

## Requirements

- Go 1.22+ to build
- The OS-native `ping` binary must be on `PATH` (present by default on Windows, and on Linux via `iputils-ping` / macOS out of the box)

## Building

```powershell
# From the repo root
go build -o ping-go.exe .\cmd\ping-go

# Cross-compile for Windows from Linux/macOS
$env:GOOS="windows"; $env:GOARCH="amd64"; go build -o ping-go.exe .\cmd\ping-go
```

```bash
# Linux/macOS
go build -o ping-go ./cmd/ping-go
```

## Usage

```
ping-go -hosts=<host1,host2,...> [flags]
```

| Flag          | Default    | Description                                                                 |
|---------------|------------|------------------------------------------------------------------------------|
| `-hosts`      | *(required)* | Comma-separated list of hosts to ping                                      |
| `-screen`     | `true`     | Print output to the console                                                  |
| `-log`        | `false`    | Enable logging to a file                                                     |
| `-logfile`    | `ping.log` | Path to the log file (used when `-log=true`)                                 |
| `-concurrent` | `false`    | Ping all hosts at the same time instead of one after another                 |
| `-count`      | `4`        | Number of echoes per host (ignored if `-duration` is set)                    |
| `-timeout`    | `5s`       | Max time to wait for a single ping invocation, e.g. `2s`, `500ms`             |
| `-duration`   | `0`        | Keep pinging each host for this long, e.g. `30s`, `2m` (overrides `-count`)   |
| `-daemon`     | `false`    | Detach and run in the background                                             |

### Examples (PowerShell)

```powershell
# Basic ping, screen output only
.\ping-go.exe -hosts=google.com

# Two hosts, run concurrently, custom timeout
.\ping-go.exe -hosts=google.com,1.1.1.1 -concurrent=true -timeout=2s

# Ping continuously for 30 seconds instead of a fixed count
.\ping-go.exe -hosts=google.com -duration=30s

# Log to a file, no console output
.\ping-go.exe -hosts=google.com -screen=false -log=true -logfile=C:\Logs\ping.log

# Fire-and-forget: detach into the background, log to file, run for 10 minutes
.\ping-go.exe -hosts=google.com,1.1.1.1 -concurrent=true -daemon=true -duration=10m -log=true -logfile=C:\Logs\ping.log

# Confirm it's still running, then stop it
Get-Process ping-go -ErrorAction SilentlyContinue
Stop-Process -Id <PID>
```

### Examples (bash)

```bash
./ping-go -hosts=google.com,1.1.1.1 -concurrent=true -daemon=true -duration=10m -log=true -logfile=/var/log/ping-go.log
ps aux | grep ping-go
kill <PID>
```

## How `-daemon` works

When `-daemon=true` is passed, the process re-executes itself with the same
flags (minus `-daemon`), fully detached from the current console:

- **Windows**: spawned with `CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS | CREATE_NO_WINDOW`, so it has no console window and isn't tied to the parent's session.
- **Linux/macOS**: spawned in a new session (`Setsid`), so it has no controlling terminal and won't receive `SIGHUP` when the parent shell closes.

The original process prints the child's PID and exits immediately. Since
there's no console to write to once detached, pair `-daemon` with
`-log=true -logfile=<path>` to actually capture output — otherwise it's
discarded.

There is currently no built-in PID file or `-stop` flag; track the printed
PID yourself, or use `Get-Process` / `ps` to find it later.

## Project layout

```
cmd/ping-go/        CLI entry point: flag parsing, orchestration, output routing
internal/pinger/      Wraps the OS ping binary; handles single pings and duration-based runs
internal/daemonize/   Self-detach logic; platform-specific bits isolated via build tags
```

The daemonize package splits Windows and Unix process-detach flags into
separate `//go:build` files (`daemonize_windows.go`, `daemonize_unix.go`)
since `syscall.SysProcAttr` has different fields per platform — this is
what lets the same codebase cross-compile cleanly for both.

## Known limitations / ideas for later

- No PID file, so scripting a clean stop requires noting the PID manually
- No structured (JSON/CSV) output mode — currently just wraps raw `ping` output
- `-duration` mode currently sleeps 1 second between pings; not configurable yet
