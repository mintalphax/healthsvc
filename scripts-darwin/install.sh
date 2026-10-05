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
if [ -f "$SRC/uninstall.sh" ]; then
    cp -f "$SRC/uninstall.sh" "$DEST/uninstall.sh"
    chmod 755 "$DEST/uninstall.sh"
fi
# Drop the Gatekeeper quarantine attribute on downloaded binaries (ignore if unset)
xattr -d com.apple.quarantine "$DEST/healthsvc" 2>/dev/null || true
# Ad-hoc signing avoids "unsigned binary" issues on Apple Silicon
codesign -s - -f "$DEST/healthsvc" 2>/dev/null || true
# Copy the default config only on first install; reinstalls keep parent settings
if [ ! -f "$DEST/configs/config.yaml" ]; then
    mkdir -p "$DEST/configs"
    cp "$SRC/configs/config.yaml" "$DEST/configs/config.yaml"
fi
# root:staff 775 - the root daemon writes the trigger file; the user-session
# agent must be able to remove it
chown -R root:staff "$DEST"
chmod 775 "$DEST"
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
echo "----------------------------------------"
echo "$MSG_HELP_1"
echo "$MSG_HELP_2"
echo "$MSG_HELP_3"
echo "$MSG_HELP_4"
echo "$MSG_HELP_5"
echo "----------------------------------------"
echo "$MSG_HINT"
