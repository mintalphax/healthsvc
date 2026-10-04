# Create scheduled task for HealthMonitor
param(
    [string]$InstallDir
)

# Remove quotes and trailing backslash from InstallDir
$InstallDir = $InstallDir.Trim('"').TrimEnd('\')
$InstallDir = $InstallDir + '\'

Write-Host "InstallDir: $InstallDir"

# Remove existing task if exists
schtasks /Delete /TN "HealthMonitorTask" /F 2>$null

# Build command path
$vbsPath = $InstallDir + "run_hidden.vbs"
$batPath = $InstallDir + "monitor.bat"
$cmd = 'wscript //B //Nologo "' + $vbsPath + '" "' + $batPath + '"'

Write-Host "Command: $cmd"

# Use schtasks to create task
$result = schtasks /Create /TN "HealthMonitorTask" /TR $cmd /SC MINUTE /MO 1 /F 2>&1
Write-Host $result

# Try to configure battery settings via COM object
try {
    $service = New-Object -ComObject Schedule.Service
    $service.Connect()
    $folder = $service.GetFolder("\")
    $task = $folder.GetTask("HealthMonitorTask")
    $taskDef = $task.Definition
    $taskDef.Settings.DisallowStartIfOnBatteries = $false
    $taskDef.Settings.StopIfGoingOnBatteries = $false
    $folder.RegisterTaskDefinition("HealthMonitorTask", $taskDef, 4, $null, $null, 0)
    Write-Host "Battery settings configured"
} catch {
    Write-Host "Note: Could not configure battery settings. Task may not run on battery power."
}
