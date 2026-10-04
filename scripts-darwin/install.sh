#!/bin/bash
# HealthSvc - macOS 安装脚本
# 用法: sudo ./install.sh
# 将二进制与配置部署到 /Library/HealthSvc 并注册 LaunchDaemon/LaunchAgent。

set -euo pipefail

SRC="$(cd "$(dirname "$0")/.." && pwd)"   # 项目根目录（含 healthsvc 二进制）
DEST="/Library/HealthSvc"

if [ "$(id -u)" -ne 0 ]; then
    echo "[ERROR] 请用 sudo 运行本脚本（与 Windows 版需要管理员一致）"
    exit 1
fi

if [ ! -x "$SRC/healthsvc" ]; then
    echo "[ERROR] 未找到 $SRC/healthsvc，请先构建: go build -o healthsvc ."
    exit 1
fi

echo "[1/4] 部署文件到 $DEST ..."
mkdir -p "$DEST/logs"
cp -f "$SRC/healthsvc" "$DEST/healthsvc"
chmod 755 "$DEST/healthsvc"
# 配置文件只在首次安装时复制，避免重装覆盖家长的设置
if [ ! -f "$DEST/configs/config.yaml" ]; then
    mkdir -p "$DEST/configs"
    cp "$SRC/configs/config.yaml" "$DEST/configs/config.yaml"
fi
# 根目录 root:staff 775 —— 守护进程(root)写触发文件，用户会话 agent 可删除
chown -R root:staff "$DEST"
chmod 775 "$DEST"
chmod 664 "$DEST/configs/config.yaml" 2>/dev/null || true

echo "[2/4] 注册 LaunchDaemon + LaunchAgent ..."
"$DEST/healthsvc" -install

echo "[3/4] 验证守护进程 ..."
sleep 2
if launchctl print "system/com.family.healthsvc" >/dev/null 2>&1; then
    echo "    LaunchDaemon 正在运行"
else
    echo "[WARN] LaunchDaemon 未运行，请查看 $DEST/logs/launchd.log"
fi

echo "[4/4] 完成。"
echo
echo "常用命令:"
echo "  查看状态:   sudo launchctl print system/com.family.healthsvc | head -20"
echo "  查看日志:   tail -f $DEST/logs/health.log"
echo "  修改配置:   sudo vi $DEST/configs/config.yaml（10 秒内自动生效）"
echo "  卸载:       sudo $DEST/uninstall.sh"
echo
echo "说明: 家长测试锁屏可执行  sudo $DEST/healthsvc -run -dry-run 预演调度。"
