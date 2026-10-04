#!/bin/bash
# HealthSvc - macOS uninstall script
# Usage: sudo ./uninstall.sh [--purge]
#   --purge  also removes /Library/HealthSvc (program, config and logs)
# Messages follow the system locale (zh* -> Chinese, otherwise English).

set -uo pipefail

DEST="/Library/HealthSvc"

case "${LC_ALL:-${LC_MESSAGES:-${LANG:-}}}" in
zh*)
    MSG_ADMIN="[ERROR] 请用 sudo 运行本脚本"
    MSG_STEP1="[1/2] 停止并移除 LaunchDaemon / LaunchAgent ..."
    MSG_STEP2="[2/2] 清理程序文件 ..."
    MSG_PURGED="已删除 \$DEST"
    MSG_KEPT="保留 \$DEST（配置与日志）。如需彻底删除: sudo rm -rf \$DEST"
    MSG_DONE="卸载完成。"
    ;;
*)
    MSG_ADMIN="[ERROR] Please run this script with sudo"
    MSG_STEP1="[1/2] Stopping and removing LaunchDaemon / LaunchAgent ..."
    MSG_STEP2="[2/2] Cleaning up program files ..."
    MSG_PURGED="Removed \$DEST"
    MSG_KEPT="Kept \$DEST (config and logs). To remove everything: sudo rm -rf \$DEST"
    MSG_DONE="Uninstall complete."
    ;;
esac

if [ "$(id -u)" -ne 0 ]; then
    echo "$MSG_ADMIN"
    exit 1
fi

echo "$MSG_STEP1"
if [ -x "$DEST/healthsvc" ]; then
    "$DEST/healthsvc" -uninstall
else
    /bin/launchctl bootout "system/com.family.healthsvc" 2>/dev/null || true
    uid=$(stat -f %u /dev/console 2>/dev/null || true)
    [ -n "$uid" ] && /bin/launchctl bootout "gui/$uid/com.family.healthsvc.agent" 2>/dev/null || true
    rm -f /Library/LaunchDaemons/com.family.healthsvc.plist
    rm -f /Library/LaunchAgents/com.family.healthsvc.agent.plist
fi

echo "$MSG_STEP2"
if [ "${1:-}" = "--purge" ]; then
    rm -rf "$DEST"
    echo "$MSG_PURGED"
else
    echo "$MSG_KEPT"
fi

echo "$MSG_DONE"
