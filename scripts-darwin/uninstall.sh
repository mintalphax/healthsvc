#!/bin/bash
# HealthSvc - macOS 卸载脚本
# 用法: sudo ./uninstall.sh [--purge]
#   --purge  额外删除 /Library/HealthSvc（程序、配置与日志）

set -uo pipefail

DEST="/Library/HealthSvc"

if [ "$(id -u)" -ne 0 ]; then
    echo "[ERROR] 请用 sudo 运行本脚本"
    exit 1
fi

echo "[1/2] 停止并移除 LaunchDaemon / LaunchAgent ..."
if [ -x "$DEST/healthsvc" ]; then
    "$DEST/healthsvc" -uninstall
else
    /bin/launchctl bootout "system/com.family.healthsvc" 2>/dev/null || true
    uid=$(stat -f %u /dev/console 2>/dev/null || true)
    [ -n "$uid" ] && /bin/launchctl bootout "gui/$uid/com.family.healthsvc.agent" 2>/dev/null || true
    rm -f /Library/LaunchDaemons/com.family.healthsvc.plist
    rm -f /Library/LaunchAgents/com.family.healthsvc.agent.plist
fi

echo "[2/2] 清理程序文件 ..."
if [ "${1:-}" = "--purge" ]; then
    rm -rf "$DEST"
    echo "已删除 $DEST"
else
    echo "保留 $DEST（配置与日志）。如需彻底删除: sudo rm -rf $DEST"
fi

echo "卸载完成。"
