@echo off
REM This file is called by the scheduled task
REM It checks for the trigger file in the same directory as this script

REM Get the directory where this script is located
set "SCRIPT_DIR=%~dp0"
set "TRIGGER_FILE=%SCRIPT_DIR%_h.dat"

if exist "%TRIGGER_FILE%" (
    rundll32.exe user32.dll,LockWorkStation
    del /F /Q "%TRIGGER_FILE%"
)
