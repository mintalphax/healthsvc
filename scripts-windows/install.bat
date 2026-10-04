@echo off
setlocal enabledelayedexpansion
echo ========================================
echo    Health Service - Install
echo ========================================
echo.

REM Get the directory where this script is located (works in both scripts/ and dist/healthsvc/)
set "INSTALL_DIR=%~dp0"

REM Check if running as administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ERROR] Please run this script as Administrator!
    echo Right-click this file and select "Run as administrator"
    pause
    exit /b 1
)

REM Check if executable exists
if not exist "%INSTALL_DIR%healthsvc.exe" (
    echo [ERROR] Cannot find healthsvc.exe in current directory
    echo Please run build.bat first
    pause
    exit /b 1
)

echo [1/4] Stop and remove old service (if exists)...
sc stop "KeepHealthService" >nul 2>&1
timeout /t 3 /nobreak >nul
sc delete "KeepHealthService" >nul 2>&1
timeout /t 3 /nobreak >nul

echo [2/4] Create scheduled task...
powershell -ExecutionPolicy Bypass -File "%INSTALL_DIR%create_task.ps1" -InstallDir "%INSTALL_DIR%"
if %errorLevel% equ 0 (
    echo Scheduled task created successfully
) else (
    echo [WARNING] Failed to create scheduled task
)

echo [3/4] Install new service...
%INSTALL_DIR%healthsvc.exe -install
if %errorLevel% neq 0 (
    echo [ERROR] Service installation failed, error code: %errorLevel%
    echo.
    echo Possible reasons:
    echo 1. Service already exists, run manually: sc delete KeepHealthService
    echo 2. config.yaml not found or has incorrect format
    echo 3. Insufficient permissions
    pause
    exit /b 1
)

echo [4/4] Start service...
sc start "KeepHealthService"
if %errorLevel% neq 0 (
    if %errorLevel% equ 1063 (
        echo Service is starting...
    ) else (
        echo [WARNING] Service installed but failed to start, error code: %errorLevel%
        echo Please check log file: logs\health.log
    )
) else (
    echo Service started successfully!
)

timeout /t 2 /nobreak >nul
sc query "KeepHealthService" | find "STATE"

echo.
echo ========================================
echo Installation Complete!
echo ========================================
echo.
echo Config: %INSTALL_DIR%configs\config.yaml
echo Log:    %INSTALL_DIR%logs\health.log
echo.
echo Common commands:
echo   Check status: sc query KeepHealthService
echo   Stop service:  sc stop KeepHealthService
echo   Start service: sc start KeepHealthService
