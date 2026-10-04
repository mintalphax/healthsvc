#!/bin/bash
# Build and package release zips for all supported platforms.
# 用法: ./package.sh [版本号]   例如 ./package.sh v1.0.0
# 产物在 dist/ 下：healthsvc-{windows-amd64,macos-arm64,macos-amd64}.zip + SHA256SUMS.txt
set -euo pipefail
cd "$(dirname "$0")"

VERSION="${1:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"
STAGE="dist"

rm -rf "$STAGE"
mkdir -p "$STAGE"

sign_if_possible() {  # ad-hoc sign darwin binaries (no-op elsewhere)
    command -v codesign >/dev/null 2>&1 && codesign -s - -f "$1" 2>/dev/null || true
}

# ---------- Windows amd64 ----------
W="$STAGE/healthsvc-windows-amd64"
mkdir -p "$W/configs"
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$W/healthsvc.exe" .
cp configs/config.yaml "$W/configs/"
cp scripts-windows/install.bat scripts-windows/uninstall.bat \
   scripts-windows/monitor.bat scripts-windows/run_hidden.vbs \
   scripts-windows/create_task.ps1 "$W/"
cp README.md LICENSE "$W/"
(cd "$STAGE" && zip -qr healthsvc-windows-amd64.zip healthsvc-windows-amd64)
echo "built $W"

# ---------- macOS (per-arch) ----------
package_macos() {
    local goarch="$1" name="healthsvc-macos-$2"
    local M="$STAGE/$name"
    mkdir -p "$M/configs"
    GOOS=darwin GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" -o "$M/healthsvc" .
    sign_if_possible "$M/healthsvc"
    cp configs/config.yaml "$M/configs/"
    cp scripts-darwin/install.sh scripts-darwin/uninstall.sh "$M/"
    chmod +x "$M/install.sh" "$M/uninstall.sh" "$M/healthsvc"
    cp README.md LICENSE "$M/"
    (cd "$STAGE" && zip -qr "$name.zip" "$name")
    echo "built $M"
}
package_macos arm64 arm64
package_macos amd64 amd64

# ---------- checksums ----------
(cd "$STAGE" && if command -v sha256sum >/dev/null 2>&1; then
    sha256sum *.zip > SHA256SUMS.txt
else
    shasum -a 256 *.zip > SHA256SUMS.txt
fi)

echo
echo "=== dist/ ==="
ls -la "$STAGE"
