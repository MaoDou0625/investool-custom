param(
    [string]$RepoPath = (Split-Path -Parent $PSScriptRoot),
    [string]$ShortcutName = "InvesTool Custom.lnk",
    [string]$Url = "http://127.0.0.1:4869/fund",
    [string]$TaskName = "InvesTool Custom App",
    [switch]$DirectPowerShellShortcut
)

$ErrorActionPreference = "Stop"

$RepoPath = (Resolve-Path -LiteralPath $RepoPath).Path
$launcherPath = Join-Path $RepoPath "scripts\launch_investool_app.ps1"
if (!(Test-Path -LiteralPath $launcherPath)) {
    throw "Launcher script not found: $launcherPath"
}

$desktopPath = [Environment]::GetFolderPath("Desktop")
if ([string]::IsNullOrWhiteSpace($desktopPath)) {
    throw "Desktop path was not found."
}

$shortcutPath = Join-Path $desktopPath $ShortcutName
$powershellPath = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
$schtasksPath = Join-Path $env:SystemRoot "System32\schtasks.exe"
$iconPath = Join-Path $RepoPath "statics\favicon.ico"
if (!(Test-Path -LiteralPath $iconPath)) {
    $iconPath = Join-Path $RepoPath "bin\investool-custom.exe"
}

if (!$DirectPowerShellShortcut) {
    $taskAction = New-ScheduledTaskAction -Execute $powershellPath -Argument "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$launcherPath`" -Url `"$Url`""
    $taskPrincipal = New-ScheduledTaskPrincipal -UserId ([System.Security.Principal.WindowsIdentity]::GetCurrent().Name) -LogonType Interactive -RunLevel Limited
    $taskSettings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Hours 12)
    Register-ScheduledTask -TaskName $TaskName -Action $taskAction -Principal $taskPrincipal -Settings $taskSettings -Force | Out-Null
}

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
if ($DirectPowerShellShortcut) {
    $shortcut.TargetPath = $powershellPath
    $shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$launcherPath`" -Url `"$Url`""
} else {
    $shortcut.TargetPath = $schtasksPath
    $shortcut.Arguments = "/Run /TN `"$TaskName`""
}
$shortcut.WorkingDirectory = $RepoPath
$shortcut.Description = "Start InvesTool Custom as a desktop app."
if (Test-Path -LiteralPath $iconPath) {
    $shortcut.IconLocation = $iconPath
}
$shortcut.Save()

Write-Output $shortcutPath
