@echo off
REM Build healthsvc.exe for Windows and healthsvc for macOS (cross-compile).
setlocal
cd /d "%~dp0"

go build -trimpath -ldflags "-s -w" -o healthsvc.exe .
set GOOS=darwin
set GOARCH=arm64
go build -trimpath -ldflags "-s -w" -o healthsvc .
set GOOS=
set GOARCH=

echo built: healthsvc.exe (Windows), healthsvc (macOS arm64)
