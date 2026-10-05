#!/bin/bash
# HealthSvc - macOS install script
# Usage: sudo ./install.sh
# Works with both layouts: the flat release layout (script next to the
# healthsvc binary) and the source repo layout (script in scripts-darwin/).
# Deploys the program to /Library/HealthSvc and registers the
# LaunchDaemon/LaunchAgent.
# Messages follow the system locale (zh* -> Chinese, otherwise English).

set -euo pipefail

DEST="/Library/HealthSvc"

# ---- messages (i18n) -------------------------------------------------------
case "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" in
zh*)
    MSG_ADMIN="[ERROR] 请用 sudo 运行本脚本（与 Windows 版需要管理员一致）"
    MSG_NOBIN="[ERROR] 未找到 healthsvc 二进制，请先构建: go build -o healthsvc ."
    MSG_STEP1="[1/4] 部署文件到 \$DEST ..."
    MSG_STEP2="[2/4] 注册 LaunchDaemon + LaunchAgent ..."
    MSG_STEP3="[3/4] 验证守护进程 ..."
    MSG_RUNNING="    LaunchDaemon 正在运行"
    MSG_NOT_RUNNING="[WARN] LaunchDaemon 未运行，请查看 \$DEST/logs/launchd.log"
    MSG_DONE="[4/4] 完成。"
    MSG_HELP_1="  查看状态:   sudo launchctl print system/com.family.healthsvc | head -20"
    MSG_HELP_2="  查看日志:   tail -f \$DEST/logs/health.log"
    MSG_HELP_3="  修改配置:   sudo vi \$DEST/configs/config.yaml（10 秒内自动生效）"
    MSG_HELP_4="  卸载:       sudo \$DEST/uninstall.sh"
    MSG_HELP_5="  检查 agent: launchctl print gui/\$(id -u)/com.family.healthsvc.agent | head -5"
    MSG_NOTE_1="[提示] macOS 26：若第一次真实锁屏测试唤醒后没有要求密码，"
    MSG_NOTE_2="       请在 系统设置 → 锁定屏幕 → 需要密码 设为\"立即\"（设一次即可），详见仓库 README。"
    MSG_HINT="说明: 改锁屏时间编辑 \$DEST/configs/config.yaml，10 秒内生效；家长测试可执行  sudo \$DEST/healthsvc -run -dry-run 预演调度。"
    ;;
*)
    MSG_ADMIN="[ERROR] Please run this script with sudo (admin rights are required, same as on Windows)"
    MSG_NOBIN="[ERROR] healthsvc binary not found; build it first: go build -o healthsvc ."
    MSG_STEP1="[1/4] Deploying files to \$DEST ..."
    MSG_STEP2="[2/4] Registering LaunchDaemon + LaunchAgent ..."
    MSG_STEP3="[3/4] Verifying the daemon ..."
    MSG_RUNNING="    LaunchDaemon is running"
    MSG_NOT_RUNNING="[WARN] LaunchDaemon is not running; see \$DEST/logs/launchd.log"
    MSG_DONE="[4/4] Done."
    MSG_HELP_1="  Status:      sudo launchctl print system/com.family.healthsvc | head -20"
    MSG_HELP_2="  Logs:        tail -f \$DEST/logs/health.log"
    MSG_HELP_3="  Edit config: sudo vi \$DEST/configs/config.yaml (hot-reloads within 10 s)"
    MSG_HELP_4="  Uninstall:   sudo \$DEST/uninstall.sh"
    MSG_HELP_5="  Check agent: launchctl print gui/\$(id -u)/com.family.healthsvc.agent | head -5"
    MSG_NOTE_1="[NOTE] macOS 26: if the first real lock test does not ask for a password on wake,"
    MSG_NOTE_2="       set System Settings -> Lock Screen -> Require password: Immediately once (see README)."
    MSG_HINT="Tip: to change the lock times edit \$DEST/configs/config.yaml (hot-reloads within 10 s); to preview the schedule run  sudo \$DEST/healthsvc -run -dry-run"
    ;;
esac

if [ "$(id -u)" -ne 0 ]; then
    echo "$MSG_ADMIN"
    exit 1
fi

# Locate the install source: script dir first (release layout), then parent (repo layout)
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -x "$SCRIPT_DIR/healthsvc" ]; then
    SRC="$SCRIPT_DIR"
elif [ -x "$SCRIPT_DIR/../healthsvc" ]; then
    SRC="$(cd "$SCRIPT_DIR/.." && pwd)"
else
    echo "$MSG_NOBIN"
    exit 1
fi

echo "$MSG_STEP1"
mkdir -p "$DEST/logs"
cp -f "$SRC/healthsvc" "$DEST/healthsvc"
chmod 755 "$DEST/healthsvc"
# uninstall.sh always sits next to install.sh itself (scripts-darwin/ in the
# repo layout, flat dir in the release layout) - copying it from $SRC missed
# the repo layout and silently left /Library/HealthSvc/uninstall.sh missing.
if [ -f "$SCRIPT_DIR/uninstall.sh" ]; then
    cp -f "$SCRIPT_DIR/uninstall.sh" "$DEST/uninstall.sh"
    chmod 755 "$DEST/uninstall.sh"
fi
# Drop the Gatekeeper quarantine attribute on downloaded binaries (ignore if unset)
xattr -d com.apple.quarantine "$DEST/healthsvc" 2>/dev/null || true
# Ad-hoc signing avoids "unsigned binary" issues on Apple Silicon
codesign -s - -f "$DEST/healthsvc" 2>/dev/null || true
# Copy the default config only on first install; reinstalls keep parent
# settings. If the shipped default has changed (e.g. new options), save it
# alongside as config.yaml.new for manual diff & merge.
if [ ! -f "$DEST/configs/config.yaml" ]; then
    mkdir -p "$DEST/configs"
    cp "$SRC/configs/config.yaml" "$DEST/configs/config.yaml"
elif ! cmp -s "$SRC/configs/config.yaml" "$DEST/configs/config.yaml"; then
    cp "$SRC/configs/config.yaml" "$DEST/configs/config.yaml.new"
    echo "(new default config saved as configs/config.yaml.new - diff & merge new options | 新版默认配置已存为 configs/config.yaml.new，请对照合并新增选项)"
fi
# root:staff 775 - the root daemon writes the trigger file; the user-session
# agent must be able to remove it
chown -R root:staff "$DEST"
chmod 775 "$DEST"
# logs 目录必须对用户会话可写：agent 要在里面写 agent.log，否则它启动即崩溃
chmod 775 "$DEST/logs"
# 旧安装遗留的 agent.log 若为 root 属主，agent 无法追加，归还给控制台用户
console_uid="$(stat -f %u /dev/console 2>/dev/null || true)"
if [ -n "$console_uid" ] && [ -f "$DEST/logs/agent.log" ]; then
    chown "$console_uid":staff "$DEST/logs/agent.log"
    chmod 664 "$DEST/logs/agent.log"
fi
chmod 664 "$DEST/configs/config.yaml" 2>/dev/null || true

echo "$MSG_STEP2"
"$DEST/healthsvc" -install

echo "$MSG_STEP3"
sleep 2
if launchctl print "system/com.family.healthsvc" >/dev/null 2>&1; then
    echo "$MSG_RUNNING"
else
    echo "$MSG_NOT_RUNNING"
fi

echo "$MSG_DONE"
echo
# macOS 26 专属提示：程序化写入的密码策略可能不被采纳
macos_major="$(sw_vers -productVersion 2>/dev/null | cut -d. -f1)"
if [ "${macos_major:-0}" -ge 26 ] 2>/dev/null; then
    echo "$MSG_NOTE_1"
    echo "$MSG_NOTE_2"
    echo
fi
echo "----------------------------------------"
echo "$MSG_HELP_1"
echo "$MSG_HELP_2"
echo "$MSG_HELP_3"
echo "$MSG_HELP_4"
echo "$MSG_HELP_5"
echo "----------------------------------------"
echo "$MSG_HINT"
