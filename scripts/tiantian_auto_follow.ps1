param(
    [string]$ConfigPath = "",
    [switch]$SetupLogin,
    [switch]$Headed,
    [switch]$Preview,
    [switch]$SetCredential,
    [switch]$NoCredential,
    [switch]$KeepDownload,
    [string]$CredentialTarget = "investool-tiantian",
    [string]$HoldingUrl = "",
    [string]$ImportUrl = "",
    [int]$TimeoutMs = 0
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$toolRoot = Join-Path $repoRoot "tools\tiantian-auto"
$nodeModules = Join-Path $toolRoot "node_modules"
$credentialScript = Join-Path $repoRoot "scripts\tiantian_credential.ps1"

if ($SetCredential) {
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $credentialScript -Action Set -Target $CredentialTarget
    exit $LASTEXITCODE
}

if (-not (Test-Path -LiteralPath $nodeModules)) {
    Push-Location $toolRoot
    try {
        $env:PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD = "1"
        npm install --no-audit --no-fund
    } finally {
        Pop-Location
    }
}

$argsList = @("src/follow.js")
if ($ConfigPath) {
    $argsList += @("--config", (Resolve-Path -LiteralPath $ConfigPath).Path)
}
if ($SetupLogin) {
    $argsList += "--setup-login"
}
if ($Headed) {
    $argsList += "--headed"
}
if ($Preview) {
    $argsList += "--preview"
}
if ($KeepDownload) {
    $argsList += "--keep-download"
}
if ($HoldingUrl) {
    $argsList += @("--holding-url", $HoldingUrl)
}
if ($ImportUrl) {
    $argsList += @("--import-url", $ImportUrl)
}
if ($TimeoutMs -gt 0) {
    $argsList += @("--timeout-ms", $TimeoutMs.ToString())
}

Push-Location $toolRoot
$nodeExitCode = 1
$oldTianTianUsername = $env:TIANTIAN_USERNAME
$oldTianTianPassword = $env:TIANTIAN_PASSWORD
$mutex = New-Object System.Threading.Mutex($false, "Local\investool-tiantian-auto")
$hasMutex = $false
try {
    $hasMutex = $mutex.WaitOne(0)
    if (-not $hasMutex) {
        [pscustomobject]@{
            status = "already_running"
            error = "TianTian auto follow is already running."
        } | ConvertTo-Json
        $nodeExitCode = 6
    } else {
        if (-not $NoCredential) {
            $credentialJson = & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $credentialScript -Action Get -Target $CredentialTarget -IncludeSecret 2>$null
            if ($LASTEXITCODE -eq 0 -and $credentialJson) {
                $credential = $credentialJson | ConvertFrom-Json
                $env:TIANTIAN_USERNAME = $credential.UserName
                $env:TIANTIAN_PASSWORD = $credential.Password
            }
        }
        node @argsList
        $nodeExitCode = $LASTEXITCODE
    }
} finally {
    if ($hasMutex) {
        $mutex.ReleaseMutex()
    }
    $mutex.Dispose()
    $env:TIANTIAN_USERNAME = $oldTianTianUsername
    $env:TIANTIAN_PASSWORD = $oldTianTianPassword
    Pop-Location
}
exit $nodeExitCode
