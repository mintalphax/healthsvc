@echo off
setlocal enabledelayedexpansion
echo ========================================
echo    Health Service - Install
echo ========================================
echo.

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "INSTALL_DIR=%SCRIPT_DIR%"

REM Layout 1 (release zip / flat): the exe sits next to this script.
REM Layout 2 (source checkout): build.bat leaves the exe in the repo root,
REM one level above scripts-windows\. Resolve to it and copy the helper
REM scripts next to the exe - the scheduled task must run monitor.bat from
REM the exe's directory so it can find the _h.dat trigger file.
if not exist "%INSTALL_DIR%healthsvc.exe" if exist "%SCRIPT_DIR%..\healthsvc.exe" (
    pushd "%SCRIPT_DIR%.."
    set "INSTALL_DIR=!CD!\"
    popd
)

REM Still not found? Then the user must assemble the folder themselves.
if not exist "%INSTALL_DIR%healthsvc.exe" (
    echo [ERROR] Cannot find healthsvc.exe next to this script or in the parent directory.
    echo.
    echo Expected one of:
    echo   1. a release zip layout: healthsvc.exe, configs\ and these scripts in ONE folder
    echo   2. a source checkout: run build.bat first, then run this script again
    pause
    exit /b 1
)

if /I not "%SCRIPT_DIR%"=="%INSTALL_DIR%" (
    echo [INFO] Source layout detected: installing from "%INSTALL_DIR%"
    copy /Y "%SCRIPT_DIR%monitor.bat" "%INSTALL_DIR%" >nul
    copy /Y "%SCRIPT_DIR%run_hidden.vbs" "%INSTALL_DIR%" >nul
    copy /Y "%SCRIPT_DIR%create_task.ps1" "%INSTALL_DIR%" >nul
    copy /Y "%SCRIPT_DIR%uninstall.bat" "%INSTALL_DIR%" >nul
) else (
    REM Flat layout: still refresh the copies so a stale script version can
    REM never survive an install.
    copy /Y "%SCRIPT_DIR%monitor.bat" "%INSTALL_DIR%" >nul 2>&1
    copy /Y "%SCRIPT_DIR%run_hidden.vbs" "%INSTALL_DIR%" >nul 2>&1
    copy /Y "%SCRIPT_DIR%create_task.ps1" "%INSTALL_DIR%" >nul 2>&1
)

REM Check if running as administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ERROR] Please run this script as Administrator!
    echo Right-click this file and select "Run as administrator"
    pause
    exit /b 1
)

REM Remove Mark-of-the-Web from the installed files: zip archives downloaded
REM through a browser carry it, and the per-minute scheduled task would
REM otherwise raise an "Open File - Security Warning" prompt for monitor.bat.
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-ChildItem -LiteralPath '%INSTALL_DIR%' -Recurse -File | Unblock-File" >nul 2>&1

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
        echo Please check log file: %INSTALL_DIR%logs\health.log
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
