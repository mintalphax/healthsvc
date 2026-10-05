@echo off
setlocal enabledelayedexpansion
echo ========================================
echo    Health Service - Uninstall
echo ========================================
echo.

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "INSTALL_DIR=%SCRIPT_DIR%"

REM Same two layouts as install.bat: exe next to the script (release zip) or
REM one level up (source checkout, where install.bat also staged the helpers).
if not exist "%INSTALL_DIR%healthsvc.exe" if exist "%SCRIPT_DIR%..\healthsvc.exe" (
    pushd "%SCRIPT_DIR%.."
    set "INSTALL_DIR=!CD!\"
    popd
)

REM Check if running as administrator
net session >nul 2>&1
if %errorLevel% neq 0 (
    echo [ERROR] Please run this script as Administrator!
    echo Right-click this file and select "Run as administrator"
    pause
    exit /b 1
)

echo [1/2] Removing service...
if exist "%INSTALL_DIR%healthsvc.exe" (
    %INSTALL_DIR%healthsvc.exe -uninstall
    if %errorLevel% neq 0 (
        echo [WARNING] Service uninstallation failed, trying sc command...
        sc stop "KeepHealthService" >nul 2>&1
        timeout /t 2 /nobreak >nul
        sc delete "KeepHealthService" >nul 2>&1
    )
) else (
    echo Service executable not found, removing via sc...
    sc stop "KeepHealthService" >nul 2>&1
    timeout /t 2 /nobreak >nul
    sc delete "KeepHealthService" >nul 2>&1
)

echo [2/2] Remove scheduled task...
schtasks /Delete /TN "HealthMonitorTask" /F >nul 2>&1
if %errorLevel% equ 0 (
    echo Scheduled task removed successfully
) else (
    echo Scheduled task not found or already removed
)

echo.
echo ========================================
echo Uninstallation Complete!
echo ========================================

REM --purge also deletes the install folder (exe, configs, logs, state).
REM Only allowed when uninstall.bat itself lives in that folder - in the
REM source-checkout layout the "install folder" is the repo root, which must
REM never be wiped by a flag.
if /I not "%~1"=="--purge" (
    echo.
    echo Tip: run "uninstall.bat --purge" from an admin prompt to also delete
    echo the install folder and its config and log files.
    exit /b 0
)

if /I not "%SCRIPT_DIR%"=="%INSTALL_DIR%" (
    echo.
    echo [WARNING] --purge was skipped: "%INSTALL_DIR%" is not the folder that
    echo contains uninstall.bat - refusing to delete a source-checkout directory.
    echo Delete leftover files manually if that is what you intended.
    exit /b 0
)

echo.
echo Purging install folder "%INSTALL_DIR%" ...
(goto) 2>nul & rd /s /q "%INSTALL_DIR%"
