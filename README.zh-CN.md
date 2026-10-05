# healthsvc（跨平台健康锁屏服务）

[English](README.md) | [简体中文](README.zh-CN.md)

防止青少年沉迷电脑的小工具：到配置的时间点自动锁屏。支持 **Windows** 和
**macOS**，单个 Go 可执行文件，无运行时依赖。

---

## 二进制安装（推荐，无需开发环境）

到 [Releases](https://github.com/mintalphax/healthsvc/releases) 下载对应平台的压缩包：

| 文件 | 适用平台 |
|---|---|
| `healthsvc-windows-amd64.zip` | Windows 10/11 (x64) |
| `healthsvc-macos-arm64.zip` | macOS Apple Silicon (M1–M4) |
| `healthsvc-macos-amd64.zip` | macOS Intel |

如需校验完整性，配合 `SHA256SUMS.txt`（Windows 用 `certutil -hashfile 文件 SHA256`，
macOS 用 `shasum -a 256`）。

### Windows · 二进制安装

1. 解压压缩包。**解压出来的文件夹就是永久安装目录**——装完后不要删，服务从这里
   读配置、写日志。
2. 右键 `install.bat` → **以管理员身份运行**。会注册 `KeepHealthService` 服务
   （开机自启、崩溃自动重启）和每分钟执行的计划任务 `HealthMonitorTask`
   （由它在你的会话里执行实际锁屏）。
3. 以后改锁屏时间：编辑**该文件夹内**的 `configs\config.yaml`
   （见下文[安装后修改锁屏时间](#安装后修改锁屏时间)）。
4. 日志：安装目录下的 `logs\health.log`。
5. **卸载**：同目录下右键 `uninstall.bat` 以管理员身份运行。会先停止再删除服务和
   计划任务；卸载后该文件夹即可删除。

### macOS · 二进制安装

1. 解压压缩包，终端进入解压出的文件夹。
2. 执行 `sudo ./install.sh`。若提示"无法验证开发者"，先执行
   `sudo xattr -rd com.apple.quarantine <解压出的文件夹>` 再重试。
3. 脚本会把程序部署到 `/Library/HealthSvc` 并注册：
   - `/Library/LaunchDaemons/com.family.healthsvc.plist` —— root 调度守护进程
     （开机自启，KeepAlive）
   - `/Library/LaunchAgents/com.family.healthsvc.agent.plist` —— 用户会话锁屏
     agent，安装时即加载进当前登录会话
4. 以后改锁屏时间：编辑 `/Library/HealthSvc/configs/config.yaml`
   （`sudo vi` 或任意编辑器，文件对管理员组可写）。
5. 日志：`/Library/HealthSvc/logs/health.log`（守护进程）与 `agent.log`（锁屏 agent）。
6. **卸载**：`sudo /Library/HealthSvc/uninstall.sh`（保留配置与日志），或
   `sudo /Library/HealthSvc/uninstall.sh --purge`（全部删除）。

### 安装后修改锁屏时间

配置文件在安装位置——**不在**下载的压缩包里：

| 安装方式 | 配置文件位置 |
|---|---|
| Windows 二进制 | `<安装目录>\configs\config.yaml` |
| macOS 二进制 / 源码 | `/Library/HealthSvc/configs/config.yaml` |

编辑 `lock_times`（24 小时制 `HH:MM`，可配多个）、`weekdays`（留空或 `["*"]`
表示每天），以及可选的 `timezone`——字段含义见[配置](#配置)一节。守护进程
10 秒内热重载配置并**立即重新检查**调度，无需重启任何东西。改完先自检：

```bash
/Library/HealthSvc/healthsvc -check        # macOS（读取已安装的配置）
healthsvc.exe -check                        # Windows（在安装目录里运行）
```

`-check` 会打印生效的调度（含钉扎的时区及其当前时间），配置有错（比如时区名
拼错）时以非零退出码报错。

---

## 源码构建安装

需要 Go 1.24+。这条路径最终得到的安装布局与二进制版完全相同，只是可执行文件
的来源不同。

```bash
# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o healthsvc .

# Windows
GOOS=windows GOARCH=amd64 go build -o healthsvc.exe .
```

或直接 `./build.sh` / `build.bat` 一次产出两个平台；发布打包用 `./package.sh vX.Y.Z`。

### Windows · 源码安装

1. 建一个安装文件夹，放入：新编译的 `healthsvc.exe`、`configs\` 文件夹、
   **`scripts-windows\` 里的全部文件**（`install.bat` 及其辅助脚本必须和 exe
   在同一目录）。
2. 在该文件夹里以管理员身份运行 `install.bat`。
3. 卸载：同目录的 `uninstall.bat`，以管理员身份运行。

### macOS · 源码安装

1. 在仓库根目录执行 `sudo ./scripts-darwin/install.sh` —— 脚本同时支持源码
   仓库布局（脚本位于 `scripts-darwin/`），部署结果与二进制版完全一致
   （`/Library/HealthSvc`）。
2. 卸载：`sudo /Library/HealthSvc/uninstall.sh`。

---

## macOS：确保唤醒要求密码

macOS 上的锁屏实现方式是**启动屏保**；唤醒时是否要求密码，由 *系统设置 →
锁定屏幕 → "屏幕保护程序开始或显示器关闭后需要密码"* 控制。

安装脚本和每次锁屏动作都会以程序化方式写入该策略
（`askForPassword = 1`，延迟 = 0），macOS 13–15 上开箱即用。但在
**macOS 26（Tahoe）** 上系统可能**不采纳程序化写入**——屏保启动了，唤醒却不
要密码。遇到这种情况：

1. 打开 **系统设置 → 锁定屏幕**。
2. 把 *屏幕保护程序开始或显示器关闭后需要密码* 设为**立即**。只需设置一次，
   会一直保留。
3. 做一次测试锁屏验证（会立即启动屏保）：

   ```bash
   sudo touch /Library/HealthSvc/_h.dat
   ```

   唤醒应该要求输入密码。随时可用
   `defaults -currentHost read com.apple.screensaver askForPassword`
   查看当前值（应为 `1`）；每次锁屏动作 `logs/agent.log` 里都应有
   `screen locked` 记录。

其他值得知道的 macOS 26 行为：

- Tahoe 把屏保超时设置从"锁定屏幕"面板挪走了，但"需要密码"仍在那里。
- 如果显示器完全不熄屏，通常是有应用持有电源断言（音视频播放、演示模式）——
  用 `pmset -g assertions` 检查。

---

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
  macOS 上守护进程会先为当前控制台用户强制"唤醒即要密码"策略，再以 root 直接
  执行 `pmset displaysleepnow` 锁屏。

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
├── package.sh               # 发布打包（zip + 校验和）
└── build.sh / build.bat     # 本机与交叉编译
```

## 配置

`configs/config.yaml` 完整字段参考——所有字段改动 10 秒内热重载：

```yaml
schedule:
  lock_times: ["23:10"]   # 到点锁屏，可配多个
  weekdays: []            # 空=每天；也可 ["Monday","Friday"] 或 ["*"]
  timezone: ""            # 可选：固定调度时区（IANA 名，如 "Asia/Shanghai"）；
                          # 设置后修改系统时区不影响调度
  enable: true
  check_interval: 30      # 检查间隔（秒）
ntp:
  servers: [pool.ntp.org, time.apple.com, time.windows.com, time.google.com, cn.pool.ntp.org]
  max_retries: 3
  retry_interval: 5
  allow_local_time: true  # NTP 全部失败时是否退回本地时间（false 更防篡改）
  max_time_offset: 300    # 本地时钟与 NTP 偏移超过该秒数则告警
```

`schedule.timezone` 接受任意 IANA 时区名。常用的（配置文件注释里也列了）：
`Asia/Shanghai`、`Asia/Hong_Kong`、`Asia/Taipei`、`Asia/Singapore`、
`Asia/Tokyo`、`Asia/Seoul`、`UTC`、`Europe/London`、`Europe/Berlin`、
`Europe/Paris`、`America/New_York`、`America/Chicago`、`America/Denver`、
`America/Los_Angeles`、`America/Vancouver`、`Australia/Sydney`。写错的时区名
会被拒绝：热重载时守护进程保留旧配置并记录告警日志；启动时则拒绝启动。改完
用 `healthsvc -check` 验证。

## 防篡改能力与边界

| 手段 | Windows | macOS |
|---|---|---|
| 停止守护进程 | 需管理员；且服务配置了崩溃自动重启 | 需 sudo；KeepAlive 自动拉起 |
| 改系统时钟 | 无效，调度使用 NTP 时间 | 无效，同左 |
| 改系统时区 | **设置了 `schedule.timezone` 时无效**；未设置时会平移调度（两种情况都需要管理员） | 同左 |
| 改配置文件 | 标准用户改不了（目录权限） | 标准用户改不了（root:staff 664） |
| 卸载 agent | 停用计划任务需管理员 | 标准用户可 unload 自己会话的 agent，但触发文件 90 秒无人消费时守护进程直接锁屏 |
| 断网 | NTP 失败按 `allow_local_time` 决定是否跳过 | 同左 |

边界：若孩子本身是**管理员**，两个平台都能卸载整套组件；彻底封死需要企业级
MDM/策略，超出本工具定位。

## 测试与排障

```bash
# 前台干跑：验证调度逻辑，不真的触发/锁屏
healthsvc -run -dry-run
```

**macOS：确认锁屏 agent 已加载**（最常见的故障点——agent 缺席时只会由守护进程
兜底熄屏，唤醒可能不需要密码）：

```bash
launchctl print gui/$(id -u)/com.family.healthsvc.agent | head -5
defaults -currentHost read com.apple.screensaver askForPassword   # 应为 1
```

如果 agent 缺失，注销重新登录后再运行一次安装脚本即可。

**症状：屏保启动了，但唤醒后没要求密码。** macOS 26 的已知问题——修复方法见
[macOS：确保唤醒要求密码](#macos确保唤醒要求密码) 一节。同时确认 agent 确实
运行了：`logs/agent.log` 里应有本次的 `screen locked` 记录。

**日志位置**：

- macOS 守护进程：`/Library/HealthSvc/logs/health.log`；agent：`agent.log`；
  launchd：`launchd.log`
- Windows：安装目录下 `logs\health.log`

守护进程启动时会打印它实际使用的 config/state/trigger 路径——有疑问先看这几行。

Windows 分发无需签名（自编译自装）；macOS 分发给他人在 Gatekeeper 下需签名
公证，本机编译自用不受影响。

## License

[MIT](LICENSE)
