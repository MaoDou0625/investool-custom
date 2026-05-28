param(
    [string]$RepoPath = (Split-Path -Parent $PSScriptRoot),
    [string]$ShortcutName = "InvesTool Custom.lnk",
    [string]$Url = "http://127.0.0.1:4869/fund",
    [string]$PortableGo = "D:\Code\tools\go1.20.14\bin\go.exe",
    [switch]$NoBuild
)

$ErrorActionPreference = "Stop"

function Resolve-Go {
    param([string]$PortableGoPath)

    $goCommand = Get-Command go -ErrorAction SilentlyContinue
    if ($goCommand) {
        return $goCommand.Source
    }
    if (Test-Path -LiteralPath $PortableGoPath) {
        return (Resolve-Path -LiteralPath $PortableGoPath).Path
    }
    throw "Go executable not found in PATH or at $PortableGoPath"
}

$RepoPath = (Resolve-Path -LiteralPath $RepoPath).Path
$desktopPath = [Environment]::GetFolderPath("Desktop")
if ([string]::IsNullOrWhiteSpace($desktopPath)) {
    throw "Desktop path was not found."
}

$shortcutPath = Join-Path $desktopPath $ShortcutName
$serverExePath = Join-Path $RepoPath "bin\investool-custom.exe"
$launcherExePath = Join-Path $RepoPath "bin\investool-app-launcher.exe"
$iconPath = Join-Path $RepoPath "statics\favicon.ico"
if (!(Test-Path -LiteralPath $iconPath)) {
    $iconPath = $launcherExePath
}

$oldTaskName = "InvesTool Custom App"
$oldTask = Get-ScheduledTask -TaskName $oldTaskName -ErrorAction SilentlyContinue
if ($oldTask) {
    Unregister-ScheduledTask -TaskName $oldTaskName -Confirm:$false
}

if (!$NoBuild) {
    $go = Resolve-Go -PortableGoPath $PortableGo
    $binDir = Split-Path -Parent $serverExePath
    if (!(Test-Path -LiteralPath $binDir)) {
        New-Item -ItemType Directory -Path $binDir | Out-Null
    }
    Push-Location -LiteralPath $RepoPath
    try {
        & $go build -o $serverExePath .
        & $go build -ldflags "-H=windowsgui" -o $launcherExePath .\tools\investool-app-launcher
    } finally {
        Pop-Location
    }
}

if (!(Test-Path -LiteralPath $launcherExePath)) {
    throw "Launcher executable not found: $launcherExePath"
}

$shell = New-Object -ComObject WScript.Shell
$shortcut = $shell.CreateShortcut($shortcutPath)
$shortcut.TargetPath = $launcherExePath
$shortcut.Arguments = "-repo `"$RepoPath`" -url `"$Url`""
$shortcut.WorkingDirectory = $RepoPath
$shortcut.Description = "Start InvesTool Custom as a desktop app."
if (Test-Path -LiteralPath $iconPath) {
    $shortcut.IconLocation = $iconPath
}
$shortcut.Save()

Write-Output $shortcutPath
