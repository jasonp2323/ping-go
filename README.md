# ping-go

A small cross-platform CLI for pinging one or more hosts, with optional
concurrency, duration-based runs, file logging, and the ability to detach
itself into the background — no `nohup`, no `Start-Process`, no Windows
service required.

Built primarily for Windows (PowerShell), but works on Linux and macOS too.

## Features

- Ping one host or several, comma-separated
- **Structured, log-friendly output**: one timestamped `key=value` line per echo — no chatty native `ping` text
- Native ICMP via [`pro-bing`](https://github.com/prometheus-community/pro-bing) — no dependency on the OS `ping` binary
- Run hosts sequentially or **concurrently**
- Fixed echo count, or ping continuously for a **duration**
- Per-ping **timeout** so a hung/unreachable host can't block forever
- Optional **screen output**, optional **file logging**, or both
- Optional self-**daemonizing**: detaches into the background and returns your prompt immediately

## Output format

Every echo is emitted as a single line, suitable for dropping straight into a log:

```
2026-07-15T17:30:01Z host=google.com ip=142.250.80.46 status=reply seq=1 rtt=12.3ms ttl=118
2026-07-15T17:30:02Z host=google.com ip=142.250.80.46 status=reply seq=2 rtt=11.8ms ttl=118
2026-07-15T17:30:03Z host=google.com status=timeout seq=3
```

The leading field is a sortable **RFC3339 UTC timestamp**; everything else is
`key=value` (logfmt style), so the output is easy to `grep`, `awk`, or parse.
`status` is one of `reply`, `timeout`, or `error`. `rtt`/`ttl` appear only on
replies; `err="..."` appears on errors.

## Requirements

- Go 1.22+ to build
- No external `ping` binary needed — ICMP is sent directly by the program
- **Privileges (Linux/macOS only):** by default the tool uses *unprivileged*
  UDP-based ping, which needs the `net.ipv4.ping_group_range` sysctl to include
  your group (the common case on modern desktop Linux). If that's not set, run
  with `-privileged` and either `root` or the `CAP_NET_RAW` capability. On
  **Windows** no special privileges are required — Windows has no unprivileged
  UDP ping, so the tool always uses raw ICMP there and the `-privileged` flag is
  ignored.

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
| `-privileged` | `false`    | Use raw ICMP sockets (Linux/macOS: needs root/`CAP_NET_RAW`); default is unprivileged UDP. Ignored on Windows (always raw ICMP) |
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

## Testing

```bash
# Fast, deterministic unit tests (output formatting for every option combo)
go test ./...

# Also run the real loopback ICMP smoke test (catches platform/privilege
# regressions like Windows rejecting unprivileged UDP ping)
PINGGO_INTEGRATION=1 go test ./...
```

CI (`.github/workflows/ci.yml`) runs `gofmt`, `go vet`, `go build`, and the
full test suite — including the loopback integration test — on **Linux,
Windows, and macOS**, so a platform-specific socket error is caught before it
ships. The Linux job enables unprivileged ping via
`net.ipv4.ping_group_range`; Windows uses raw ICMP automatically.

## Project layout

```
cmd/ping-go/        CLI entry point: flag parsing, orchestration, output routing
internal/pinger/      Sends ICMP via pro-bing; emits one structured Event per echo
internal/daemonize/   Self-detach logic; platform-specific bits isolated via build tags
```

The daemonize package splits Windows and Unix process-detach flags into
separate `//go:build` files (`daemonize_windows.go`, `daemonize_unix.go`)
since `syscall.SysProcAttr` has different fields per platform — this is
what lets the same codebase cross-compile cleanly for both.

## Known limitations / ideas for later

- No PID file, so scripting a clean stop requires noting the PID manually
- Output is logfmt-style text only; a JSON-lines mode could be added behind a `-format` flag
- `-duration` mode currently sleeps 1 second between pings; not configurable yet
