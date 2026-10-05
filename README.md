# healthsvc — Scheduled Screen-Lock Service

[English](README.md) | [简体中文](README.zh-CN.md)

A small parental-control tool that locks the screen at scheduled times to
curb teen computer overuse. Works on **Windows** and **macOS**; ships as a
single Go binary with no runtime dependencies.

---

## Install from binaries (recommended — no development environment needed)

Download the archive for your platform from the
[Releases](https://github.com/mintalphax/healthsvc/releases) page:

| File | Platform |
|---|---|
| `healthsvc-windows-amd64.zip` | Windows 10/11 (x64) |
| `healthsvc-macos-arm64.zip` | macOS Apple Silicon (M1–M4) |
| `healthsvc-macos-amd64.zip` | macOS Intel |

Verify integrity with `SHA256SUMS.txt` if you like (`certutil -hashfile <file> SHA256`
on Windows, `shasum -a 256` on macOS).

### Windows — binary install

1. Extract the zip. **The extracted folder is the permanent install folder** —
   do not delete it after installing; the service reads its config and writes
   its logs there.
2. Right-click `install.bat` → **Run as administrator**. This registers the
   `KeepHealthService` service (starts at boot, auto-restarts on crash) and a
   per-minute scheduled task `HealthMonitorTask` that performs the actual
   screen lock in your session.
3. To change the lock times later, edit `configs\config.yaml` **inside that
   same folder** (see [Changing the schedule](#changing-the-lock-schedule-after-install)).
4. Logs: `logs\health.log` in the install folder.
5. **Uninstall**: right-click `uninstall.bat` → Run as administrator, in the
   same folder. It stops and removes the service and the scheduled task. After
   uninstalling, the folder is inert and can be deleted.

### macOS — binary install

1. Extract the zip and open the folder in Terminal.
2. `sudo ./install.sh`. If macOS blocks the binary ("cannot verify developer"),
   run `sudo xattr -rd com.apple.quarantine <extracted folder>` first, then
   retry.
3. The script deploys the program to `/Library/HealthSvc` and registers:
   - `/Library/LaunchDaemons/com.family.healthsvc.plist` — root scheduler
     daemon (starts at boot, KeepAlive)
   - `/Library/LaunchAgents/com.family.healthsvc.agent.plist` — per-user lock
     agent, loaded into the current console session immediately
4. To change the lock times later, edit `/Library/HealthSvc/configs/config.yaml`
   (`sudo vi` or any editor; the file is group-writable for admins).
5. Logs: `/Library/HealthSvc/logs/health.log` (daemon) and `agent.log` (lock
   agent).
6. **Uninstall**: `sudo /Library/HealthSvc/uninstall.sh` (keeps config and
   logs) or `sudo /Library/HealthSvc/uninstall.sh --purge` (removes
   everything).

### Changing the lock schedule after install

The config lives in the install location — **not** in the downloaded zip:

| Installed from | Config file |
|---|---|
| Windows binary | `<install folder>\configs\config.yaml` |
| macOS binary / source | `/Library/HealthSvc/configs/config.yaml` |
| macOS source checkout | copied to `/Library/HealthSvc/configs/config.yaml` on install |

Edit `lock_times` (24-hour `HH:MM`, multiple entries allowed), `weekdays`
(empty or `["*"]` = every day), and optionally `timezone` — see
[Configuration](#configuration). The daemon hot-reloads the file within 10
seconds and re-checks the schedule immediately, so there is nothing to
restart. Tail the log to confirm the reload was picked up.

---

## Build and install from source

Requires Go 1.24+. This path produces the same installed layout as the
binaries — only the origin of the executable differs.

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o healthsvc .

# Windows
GOOS=windows GOARCH=amd64 go build -o healthsvc.exe .
```

or run `./build.sh` / `build.bat` to produce both. For release packaging use
`./package.sh vX.Y.Z`.

### Windows — source install

1. Create an install folder and put into it: the freshly built
   `healthsvc.exe`, the `configs\` folder, and **the contents of**
   `scripts-windows\` (`install.bat` and its helpers must sit next to the
   exe).
2. Run that folder's `install.bat` as administrator.
3. Uninstall: `uninstall.bat` from the same folder, as administrator.

### macOS — source install

1. From the repo root: `sudo ./scripts-darwin/install.sh` — the script
   supports the repo layout (script in `scripts-darwin/`) and deploys to
   `/Library/HealthSvc` exactly like the binary version.
2. Uninstall: `sudo /Library/HealthSvc/uninstall.sh`.

---

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
  (e.g. the agent was unloaded), the daemon on macOS enforces the
  password-after-sleep policy for the console user and performs the lock
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
├── package.sh               # release packaging (zips + checksums)
└── build.sh / build.bat     # local & cross builds
```

## Configuration

Full `configs/config.yaml` reference — every field is hot-reloaded within 10
seconds of a change:

```yaml
schedule:
  lock_times: ["23:10"]   # lock at these times (24h), multiple entries allowed
  weekdays: []            # empty = every day; or ["Monday","Friday"], or ["*"]
  timezone: ""            # optional: pin the schedule to an IANA zone, e.g. "Asia/Shanghai";
                          # when set, changing the machine's system timezone has no effect
  enable: true
  check_interval: 30      # seconds between schedule checks
ntp:
  servers: [pool.ntp.org, time.apple.com, time.windows.com, time.google.com, cn.pool.ntp.org]
  max_retries: 3
  retry_interval: 5
  allow_local_time: true  # fall back to the local clock when all NTP servers fail (false = more tamper-proof)
  max_time_offset: 300    # warn when the local clock differs from NTP by more than this many seconds
```

`schedule.timezone` accepts any IANA zone name (`Asia/Shanghai`, `UTC`,
`Europe/Berlin`, …). Leave it empty to follow the machine's system timezone.

## Tamper resistance & limits

| Attempt | Windows | macOS |
|---|---|---|
| Stop the daemon | admin required; service auto-restarts on crash | sudo required; KeepAlive brings it back |
| Change the system clock | ineffective — scheduling uses NTP time | same |
| Change the system timezone | ineffective **when `schedule.timezone` is pinned**; otherwise it shifts the schedule (needs admin either way) | same |
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
```

**macOS: verify the lock agent is loaded** (the most common failure mode —
without it the display sleeps via the daemon fallback but may not require a
password):

```bash
launchctl print gui/$(id -u)/com.family.healthsvc.agent | head -5
defaults -currentHost read com.apple.screensaver askForPassword   # expect 1
```

If the agent is missing, log out and back in, then re-run the installer.

**Symptom: display sleeps but no password is asked on wake.** The lock agent
did not run; the daemon's fallback locked without the password policy. Fix
the agent as above, and check `logs/agent.log` for errors.

**Logs**:

- macOS daemon: `/Library/HealthSvc/logs/health.log`; agent:
  `agent.log`; launchd: `launchd.log`
- Windows: `logs\health.log` in the install folder

The daemon's startup lines print the exact config/state/trigger paths it is
using — check them first when in doubt.

Windows builds need no signing for self-installation; distributing macOS
builds to others requires signing & notarization (self-compiled local use is
unaffected).

## License

[MIT](LICENSE)
