# healthsvc — Scheduled Screen-Lock Service

[English](README.md) | [简体中文](README.zh-CN.md)

A small parental-control tool that locks the screen at scheduled times to
curb teen computer overuse. Works on **Windows** and **macOS**; ships as a
single Go binary with no runtime dependencies.

## Download & Install (no development environment needed)

Grab the archive for your platform from the
[Releases](https://github.com/mintalphax/healthsvc/releases) page:

| File | Platform |
|---|---|
| `healthsvc-windows-amd64.zip` | Windows 10/11 (x64) |
| `healthsvc-macos-arm64.zip` | macOS Apple Silicon (M1–M4) |
| `healthsvc-macos-amd64.zip` | macOS Intel |

- **Windows**: extract the archive, right-click `install.bat` → **Run as
  administrator**.
- **macOS**: extract the archive, then run `sudo ./install.sh`. If macOS
  complains that the developer cannot be verified, run
  `sudo xattr -rd com.apple.quarantine <extracted folder>` first.

To verify integrity, download `SHA256SUMS.txt` and compare with `sha256sum`
(on Windows: `certutil -hashfile <file> SHA256`).

See [Installation](#installation) below for details, including uninstalling.

## How it works

```
                    ┌──────────────────────────────────────────────┐
                    │  privileged daemon (checks every 30 s)       │
                    │  1. queries NTP for trusted time (so         │
                    │     changing the system clock cannot         │
                    │     bypass the schedule)                     │
                    │  2. when a lock time is reached and not yet  │
                    │     locked today → writes trigger file       │
                    │     _h.dat and records it in _state.dat      │
                    └───────────────────┬──────────────────────────┘
                                        │ _h.dat
                                        ▼
                    ┌──────────────────────────────────────────────┐
                    │  user-session component (the only context    │
                    │  that can lock the user's screen)            │
                    │  Windows: per-minute scheduled task          │
                    │           monitor.bat → LockWorkStation      │
                    │  macOS  : LaunchAgent (WatchPaths, instant)  │
                    │           → require-password-now + pmset     │
                    │             display sleep = locked           │
                    └──────────────────────────────────────────────┘
```

The daemon runs as SYSTEM / root, which cannot lock an interactive session's
screen. That is why the design splits into a trigger file plus a per-user
component:

- **macOS uses launchd `WatchPaths`**: the agent is launched the instant the
  daemon writes `_h.dat` — lower latency than the Windows per-minute poll.
- **Direct-lock fallback**: if the trigger is not consumed within 90 seconds
  (e.g. the agent was unloaded), the daemon on macOS performs the lock
  directly as root via `pmset displaysleepnow`.

## Repository layout

```
healthsvc/
├── main.go                  # entry point & CLI (-install/-uninstall/-run/-agent/-dry-run)
├── main_darwin.go           # macOS: LaunchDaemon/LaunchAgent install, agent mode
├── main_windows.go          # Windows: service-mode entry, service registration
├── pkg/
│   ├── config/              # config.yaml parsing, validation, hot reload, schedule matching
│   ├── ntp/                 # hand-written SNTP client (multi-server, retries, offset warning)
│   ├── lockscreen/          # FileTrigger protocol + per-platform lock actions
│   ├── logger/              # log rotation & old-log cleanup
│   └── service/             # scheduler loop, _state.dat dedup, Windows service wrapper
├── configs/config.yaml      # default configuration
├── scripts-windows/         # Windows install/uninstall scripts & scheduled-task scripts
├── scripts-darwin/          # macOS install/uninstall scripts
└── build.sh / build.bat     # local & cross builds
```

## Build

Requires Go 1.24+. Build on the host, or cross-compile both artifacts from
any platform:

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o healthsvc .

# Windows
GOOS=windows GOARCH=amd64 go build -o healthsvc.exe .
```

Or run `./build.sh` / `build.bat` to produce both at once.

## Installation

### Windows

1. Put `healthsvc.exe`, `configs\` and `scripts-windows\` in the same folder.
2. Right-click `scripts-windows\install.bat` → **Run as administrator**
   (registers service `KeepHealthService` + the per-minute scheduled task
   `HealthMonitorTask`).
3. Uninstall: run `scripts-windows\uninstall.bat` as administrator.

### macOS

```bash
sudo ./scripts-darwin/install.sh
```

The script deploys the program to `/Library/HealthSvc` and registers:

- `/Library/LaunchDaemons/com.family.healthsvc.plist` — root daemon (starts at
  boot, KeepAlive restarts it if it dies)
- `/Library/LaunchAgents/com.family.healthsvc.agent.plist` — per-user lock
  agent (activated for the current console user immediately; every future
  login loads it automatically)

The installer also sets "require password immediately after sleep" for the
current user — the prerequisite for `pmset` display sleep acting as a lock.

Uninstall:

```bash
sudo /Library/HealthSvc/uninstall.sh            # keep configuration
sudo /Library/HealthSvc/uninstall.sh --purge    # remove everything
```

## Configuration

`configs/config.yaml` hot-reloads within 10 seconds of any change; no restart
needed:

```yaml
schedule:
  lock_times: ["23:10"]   # lock at these times (24h), multiple entries allowed
  weekdays: []            # empty = every day; or ["Monday","Friday"], or ["*"]
  enable: true
  check_interval: 30      # seconds between schedule checks
ntp:
  servers: [pool.ntp.org, time.apple.com, time.windows.com, time.google.com, cn.pool.ntp.org]
  max_retries: 3
  retry_interval: 5
  allow_local_time: true  # fall back to local clock when all NTP servers fail (false = more tamper-proof)
  max_time_offset: 300    # warn when local clock differs from NTP by more than this many seconds
```

## Tamper resistance & limits

| Attempt | Windows | macOS |
|---|---|---|
| Stop the daemon | admin required; service auto-restarts on crash | sudo required; KeepAlive brings it back |
| Change system clock | ineffective — scheduling uses NTP time | same |
| Edit the config | not writable by standard users (ACLs) | not writable by standard users (root:staff 664) |
| Unload the agent | disabling the task needs admin | a standard user can unload their own agent, but the daemon locks directly when the trigger sits unconsumed for 90 s |
| Cut the network | NTP failure honours `allow_local_time` | same |

Limits: if the child is an **administrator**, they can uninstall the whole
tool on either platform. Closing that hole requires enterprise MDM/policies,
which is beyond this tool's scope.

## Testing & troubleshooting

```bash
# Foreground dry run: exercises the schedule without firing triggers or locking
healthsvc -run -dry-run

# Set a lock time in the past, restart, and confirm _h.dat is created and
# _state.dat records it.
# macOS: exercise the agent manually (create the trigger as root first):
sudo touch /Library/HealthSvc/_h.dat
launchctl kickstart gui/$(id -u)/com.family.healthsvc.agent

tail -f /Library/HealthSvc/logs/health.log   # macOS
type logs\health.log                          # Windows
```

Windows builds need no signing for self-installation; distributing macOS
builds to others requires signing & notarization (self-compiled local use is
unaffected).

## License

[MIT](LICENSE)
