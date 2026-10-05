@echo off
setlocal enabledelayedexpansion
echo ========================================
echo    Health Service - Uninstall  [script v3]
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

REM --purge deletes everything inside the install folder (exe, configs, logs,
REM state). The folder itself is removed as well when no other process still
REM holds it - e.g. an open cmd prompt with that directory as its working
REM directory keeps the folder alive while its contents are wiped. Two safety
REM rules:
REM   * a folder containing go.mod or .git is the source checkout and is
REM     never purged, no matter which layout produced it;
REM   * cmd's own working directory is moved to %TEMP% first.
if /I not "%~1"=="--purge" (
    echo.
    echo Tip: run "uninstall.bat --purge" from an admin prompt to also delete
    echo the install folder and its config and log files.
    exit /b 0
)

if exist "%INSTALL_DIR%go.mod" (
    echo.
    echo [WARNING] --purge skipped: "%INSTALL_DIR%" contains go.mod and looks
    echo like the source checkout. Refusing to delete it - remove leftover
    echo files manually if that is what you intended.
    exit /b 0
)

if exist "%INSTALL_DIR%.git" (
    echo.
    echo [WARNING] --purge skipped: "%INSTALL_DIR%" contains a .git folder and
    echo looks like the source checkout. Refusing to delete it - remove
    echo leftover files manually if that is what you intended.
    exit /b 0
)

echo.
echo Purging contents of "%INSTALL_DIR%" ...
echo The folder itself is kept if another program ^(e.g. an open cmd prompt^)
echo still holds it; everything inside is removed either way.
cd /d "%TEMP%" 2>nul
REM Subdirectories first. Deleting the batch's own file must happen last and
REM inside the (goto) chain: cmd reads the script incrementally, so removing
REM this file any earlier silently kills the rest of the script.
for /d %%D in ("%INSTALL_DIR%*") do rd /s /q "%%~D" >nul 2>&1
(goto) 2>nul & del /f /q "%INSTALL_DIR%*" & rd "%INSTALL_DIR%" 2>nul
