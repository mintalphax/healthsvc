@echo off
echo ========================================
echo    Health Service - Uninstall
echo ========================================
echo.

REM Get the directory where this script is located
set "INSTALL_DIR=%~dp0"

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
