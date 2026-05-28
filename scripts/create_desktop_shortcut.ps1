param(
    [string]$RepoPath = (Split-Path -Parent $PSScriptRoot),
    [string]$ShortcutName = "InvesTool Custom.lnk",
    [string]$Url = "http://127.0.0.1:4869/fund"
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
$iconPath = Join-Path $RepoPath "statics\favicon.ico"
if (!(Test-Path -LiteralPath $iconPath)) {
    $iconPath = Join-Path $RepoPath "bin\investool-custom.exe"
}

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $powershellPath
$shortcut.Arguments = "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$launcherPath`" -Url `"$Url`""
$shortcut.WorkingDirectory = $RepoPath
$shortcut.Description = "Start InvesTool Custom as a desktop app."
if (Test-Path -LiteralPath $iconPath) {
    $shortcut.IconLocation = $iconPath
}
$shortcut.Save()

Write-Output $shortcutPath
