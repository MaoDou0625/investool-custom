param(
    [string]$RepoPath = (Split-Path -Parent $PSScriptRoot),
    [string]$PortableGo = "D:\Code\tools\go1.20.14\bin\go.exe",
    [string]$BuildOutput = ""
)

$ErrorActionPreference = "Stop"

if (!(Test-Path -LiteralPath $RepoPath)) {
    throw "Repo path not found: $RepoPath"
}

if ([string]::IsNullOrWhiteSpace($BuildOutput)) {
    $BuildOutput = Join-Path $RepoPath "bin\investool-custom.exe"
}

$goCommand = Get-Command go -ErrorAction SilentlyContinue
if ($goCommand) {
    $go = $goCommand.Source
} elseif (Test-Path -LiteralPath $PortableGo) {
    $go = $PortableGo
} else {
    throw "Go executable not found in PATH or at $PortableGo"
}

Set-Location -LiteralPath $RepoPath

$buildDir = Split-Path -Parent $BuildOutput
if (!(Test-Path -LiteralPath $buildDir)) {
    New-Item -ItemType Directory -Path $buildDir | Out-Null
}

& $go build -o $BuildOutput .
& $BuildOutput webserver --config .\config.toml
