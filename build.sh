#!/bin/bash
# Build healthsvc for both platforms from any host with Go 1.24+.
set -e
cd "$(dirname "$0")"

GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o healthsvc .
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o healthsvc.exe .

echo "built: healthsvc (macOS arm64), healthsvc.exe (Windows amd64)"
