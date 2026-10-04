# healthsvc（跨平台健康锁屏服务）

[English](README.md) | [简体中文](README.zh-CN.md)

防止青少年沉迷电脑的小工具：到配置的时间点自动锁屏。支持 **Windows** 和
**macOS**，单个 Go 可执行文件，无运行时依赖。

## 下载安装（无需开发环境）

到 [Releases](https://github.com/mintalphax/healthsvc/releases) 下载对应平台的压缩包：

| 文件 | 适用平台 |
|---|---|
| `healthsvc-windows-amd64.zip` | Windows 10/11 (x64) |
| `healthsvc-macos-arm64.zip` | macOS Apple Silicon (M1–M4) |
| `healthsvc-macos-amd64.zip` | macOS Intel |

- **Windows**：解压后进入文件夹，右键 `install.bat` → **以管理员身份运行**。
- **macOS**：解压后进入文件夹，执行 `sudo ./install.sh`。若提示"无法验证开发者"，
  先执行 `sudo xattr -rd com.apple.quarantine <解压出的文件夹>` 再运行。

校验文件完整性：下载 `SHA256SUMS.txt`，对比 `sha256sum`（Windows 用
`certutil -hashfile 文件 SHA256`）输出。

安装与卸载的详细说明见下文[安装](#安装)章节。

## 工作原理

```
                    ┌──────────────────────────────────────────────┐
                    │  特权守护进程（每 30 秒检查一次）                │
                    │  1. NTP 对时获得可信时间（防改系统时钟绕过）      │
                    │  2. 到达 lock_times 且今天未锁过 → 写触发文件     │
                    │     _h.dat，并把时间记入 _state.dat 去重        │
                    └───────────────────┬──────────────────────────┘
                                        │ _h.dat
                                        ▼
                    ┌──────────────────────────────────────────────┐
                    │  用户会话组件（只有它能锁用户的屏幕）             │
                    │  Windows: 每分钟计划任务 monitor.bat            │
                    │           → rundll32 LockWorkStation          │
                    │  macOS  : LaunchAgent (WatchPaths, 即时)       │
                    │           → 强制唤醒即要密码 + pmset 熄屏即锁    │
                    └──────────────────────────────────────────────┘
```

服务跑在 SYSTEM / root 上下文，无法直接锁交互会话的屏幕，因此采用
"触发文件 + 用户会话代理"的两段式结构：

- **macOS 用 launchd `WatchPaths`**：守护进程一写 `_h.dat`，agent 立刻被拉起，
  比 Windows 的每分钟轮询延迟更低。
- **直接锁屏兜底**：若触发文件 90 秒内没被用户会话组件消费（如 agent 被卸载），
  macOS 上守护进程会以 root 直接执行 `pmset displaysleepnow` 锁屏。

## 目录结构

```
healthsvc/
├── main.go                  # 入口与命令行（-install/-uninstall/-run/-agent/-dry-run）
├── main_darwin.go           # macOS：LaunchDaemon/LaunchAgent 安装、agent 模式
├── main_windows.go          # Windows：服务模式入口、服务注册
├── pkg/
│   ├── config/              # config.yaml 解析、校验、轮询热重载、调度判断
│   ├── ntp/                 # 手写 SNTP 客户端（多服务器、重试、偏移告警）
│   ├── lockscreen/          # FileTrigger 触发协议 + 各平台锁屏动作
│   ├── logger/              # 日志轮转与旧日志清理
│   └── service/             # 调度主循环、_state.dat 状态去重、Windows 服务封装
├── configs/config.yaml      # 默认配置
├── scripts-windows/         # Windows 安装/卸载脚本与服务计划任务脚本
├── scripts-darwin/          # macOS 安装/卸载脚本
└── build.sh / build.bat     # 本机与交叉编译
```

## 构建

需要 Go 1.24+。在本机构建，或从任一平台交叉编译两个产物：

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o healthsvc .

# Windows
GOOS=windows GOARCH=amd64 go build -o healthsvc.exe .
```

或直接 `./build.sh` / `build.bat` 一次产出两个平台。

## 安装

### Windows

1. 把 `healthsvc.exe`、`configs\`、`scripts-windows\` 放到同一目录。
2. 右键 **以管理员身份运行** `scripts-windows\install.bat`
   （注册服务 `KeepHealthService` + 每分钟计划任务 `HealthMonitorTask`）。
3. 卸载：管理员运行 `scripts-windows\uninstall.bat`。

### macOS

```bash
sudo ./scripts-darwin/install.sh
```

脚本会把程序部署到 `/Library/HealthSvc` 并注册：

- `/Library/LaunchDaemons/com.family.healthsvc.plist` —— root 守护进程（开机自启，
  KeepAlive 崩溃自动拉起）
- `/Library/LaunchAgents/com.family.healthsvc.agent.plist` —— 用户会话锁屏 agent
  （对当前登录用户即时生效，之后每个登录的用户自动加载）

安装脚本还会为当前用户设置"唤醒后立即要求密码"，这是 `pmset` 熄屏等于锁屏的前提。

卸载：

```bash
sudo /Library/HealthSvc/uninstall.sh            # 保留配置
sudo /Library/HealthSvc/uninstall.sh --purge    # 连程序带配置一起删除
```

## 配置

`configs/config.yaml` 改完 10 秒内自动热重载，无需重启：

```yaml
schedule:
  lock_times: ["23:10"]   # 到点锁屏，可配多个
  weekdays: []            # 空=每天；也可 ["Monday","Friday"] 或 ["*"]
  enable: true
  check_interval: 30      # 检查间隔（秒）
ntp:
  servers: [pool.ntp.org, time.apple.com, time.windows.com, time.google.com, cn.pool.ntp.org]
  max_retries: 3
  retry_interval: 5
  allow_local_time: true  # NTP 全部失败时是否退回本地时间（false 更防篡改）
  max_time_offset: 300    # 本地时钟与 NTP 偏移超过该秒数则告警
```

## 防篡改能力与边界

| 手段 | Windows | macOS |
|---|---|---|
| 停止守护进程 | 需管理员；且服务配置了崩溃自动重启 | 需 sudo；KeepAlive 自动拉起 |
| 改系统时钟 | 无效，调度使用 NTP 时间 | 无效，同左 |
| 改配置文件 | 标准用户改不了（目录权限） | 标准用户改不了（root:staff 664） |
| 卸载 agent | 停用计划任务需管理员 | 标准用户可 unload 自己会话的 agent，但触发文件 90 秒无人消费时守护进程直接锁屏 |
| 断网 | NTP 失败按 `allow_local_time` 决定是否跳过 | 同左 |

边界：若孩子本身是**管理员**，两个平台都能卸载整套组件；彻底封死需要企业级
MDM/策略，超出本工具定位。

## 测试与排障

```bash
# 前台干跑：不真的触发/锁屏，验证调度逻辑
healthsvc -run -dry-run

# 把 lock_times 改成已过去的时刻重启，确认 _h.dat 被创建、_state.dat 被记录
# macOS 手动验证 agent（需 root 先创建触发文件）:
sudo touch /Library/HealthSvc/_h.dat
launchctl kickstart gui/$(id -u)/com.family.healthsvc.agent

tail -f /Library/HealthSvc/logs/health.log   # macOS
type logs\health.log                          # Windows
```

Windows 分发无需签名（自编译自装）；macOS 分发给他人在 Gatekeeper 下需签名
公证，本机编译自用不受影响。

## License

[MIT](LICENSE)
