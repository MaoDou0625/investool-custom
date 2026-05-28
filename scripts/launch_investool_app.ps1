param(
    [string]$RepoPath = (Split-Path -Parent $PSScriptRoot),
    [string]$Url = "http://127.0.0.1:4869/fund",
    [int]$Port = 4869,
    [string]$BindAddress = "127.0.0.1",
    [string]$BrowserPath = "",
    [switch]$ReuseExistingServer
)

$ErrorActionPreference = "Stop"

function Write-LauncherLog {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Add-Content -LiteralPath $script:LogFile -Value "[$timestamp] $Message"
}

function Resolve-BrowserPath {
    param([string]$RequestedPath)

    if (![string]::IsNullOrWhiteSpace($RequestedPath)) {
        if (Test-Path -LiteralPath $RequestedPath) {
            return (Resolve-Path -LiteralPath $RequestedPath).Path
        }
        throw "Browser not found: $RequestedPath"
    }

    $command = Get-Command msedge.exe -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $candidates = @(
        "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe",
        "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
        "$env:LocalAppData\Microsoft\Edge\Application\msedge.exe",
        "$env:ProgramFiles\Google\Chrome\Application\chrome.exe",
        "${env:ProgramFiles(x86)}\Google\Chrome\Application\chrome.exe",
        "$env:LocalAppData\Google\Chrome\Application\chrome.exe"
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }

    $command = Get-Command chrome.exe -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }
    throw "Microsoft Edge or Google Chrome was not found."
}

function Get-ListeningProcessId {
    param(
        [string]$Address,
        [int]$LocalPort
    )
    $connection = Get-NetTCPConnection -LocalAddress $Address -LocalPort $LocalPort -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($connection) {
        return [int]$connection.OwningProcess
    }
    return $null
}

function Wait-ServerReady {
    param(
        [string]$Address,
        [int]$LocalPort,
        [int]$TimeoutSeconds = 30
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $listenerPid = Get-ListeningProcessId -Address $Address -LocalPort $LocalPort
        if ($listenerPid) {
            return $listenerPid
        }
        Start-Sleep -Milliseconds 500
    }
    throw "Server did not listen on ${Address}:$LocalPort within $TimeoutSeconds seconds."
}

function Get-ProcessExecutablePath {
    param([int]$ProcessId)
    $processInfo = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessId" -ErrorAction SilentlyContinue
    if ($processInfo) {
        return $processInfo.ExecutablePath
    }
    return ""
}

function Stop-ExistingInvestoolServer {
    param(
        [int]$ProcessId,
        [string]$ExpectedExePath
    )

    $processPath = Get-ProcessExecutablePath -ProcessId $ProcessId
    if (!$processPath -or ![string]::Equals($processPath, $ExpectedExePath, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Port $Port is already used by process $ProcessId ($processPath)."
    }

    Write-LauncherLog "Stopping existing InvesTool server process $ProcessId."
    Stop-Process -Id $ProcessId -Force
    Start-Sleep -Milliseconds 500
}

function Get-BrowserProfileProcesses {
    param(
        [string]$BrowserExeName,
        [string]$UserDataDir
    )

    $escapedName = $BrowserExeName.Replace("'", "''")
    Get-CimInstance Win32_Process -Filter "Name='$escapedName'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($UserDataDir) }
}

function Wait-BrowserClosed {
    param(
        [string]$BrowserExeName,
        [string]$UserDataDir,
        [int]$InitialWaitSeconds = 15
    )

    $deadline = (Get-Date).AddSeconds($InitialWaitSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-BrowserProfileProcesses -BrowserExeName $BrowserExeName -UserDataDir $UserDataDir) {
            break
        }
        Start-Sleep -Milliseconds 500
    }

    while (Get-BrowserProfileProcesses -BrowserExeName $BrowserExeName -UserDataDir $UserDataDir) {
        Start-Sleep -Seconds 1
    }
}

$RepoPath = (Resolve-Path -LiteralPath $RepoPath).Path
$exePath = Join-Path $RepoPath "bin\investool-custom.exe"
$configPath = ".\config.toml"
if (!(Test-Path -LiteralPath $exePath)) {
    throw "InvesTool executable not found: $exePath. Build it before launching the desktop app."
}

$logDir = Join-Path $RepoPath "tmp"
if (!(Test-Path -LiteralPath $logDir)) {
    New-Item -ItemType Directory -Path $logDir | Out-Null
}
$script:LogFile = Join-Path $logDir "investool_app_launcher.log"

$serverProcess = $null
$startedServer = $false
$browserProfileDir = Join-Path $env:LocalAppData "InvesToolCustom\AppBrowserProfile"

try {
    Write-LauncherLog "Launcher starting."
    $existingPid = Get-ListeningProcessId -Address $BindAddress -LocalPort $Port
    if ($existingPid) {
        if ($ReuseExistingServer) {
            Write-LauncherLog "Reusing existing server process $existingPid."
        } else {
            Stop-ExistingInvestoolServer -ProcessId $existingPid -ExpectedExePath $exePath
        }
    }

    if (!$existingPid -or !$ReuseExistingServer) {
        Write-LauncherLog "Starting server: $exePath webserver -c $configPath"
        $serverProcess = Start-Process -FilePath $exePath -ArgumentList @("webserver", "-c", $configPath) -WorkingDirectory $RepoPath -WindowStyle Hidden -PassThru
        $startedServer = $true
        Wait-ServerReady -Address $BindAddress -LocalPort $Port | Out-Null
    }

    if (!(Test-Path -LiteralPath $browserProfileDir)) {
        New-Item -ItemType Directory -Path $browserProfileDir | Out-Null
    }
    $resolvedBrowserPath = Resolve-BrowserPath -RequestedPath $BrowserPath
    $browserExeName = Split-Path -Leaf $resolvedBrowserPath
    $browserArgs = @(
        "--user-data-dir=$browserProfileDir",
        "--app=$Url",
        "--no-first-run",
        "--no-default-browser-check",
        "--disable-background-mode"
    )
    Write-LauncherLog "Starting browser app: $resolvedBrowserPath $($browserArgs -join ' ')"
    Start-Process -FilePath $resolvedBrowserPath -ArgumentList $browserArgs | Out-Null
    Wait-BrowserClosed -BrowserExeName $browserExeName -UserDataDir $browserProfileDir
    Write-LauncherLog "Browser app closed."
} catch {
    Write-LauncherLog "Launcher error: $($_.Exception.Message)"
    throw
} finally {
    if ($startedServer -and $serverProcess -and !(Get-Process -Id $serverProcess.Id -ErrorAction SilentlyContinue)) {
        $serverProcess = $null
    }
    if ($startedServer -and $serverProcess) {
        Write-LauncherLog "Stopping owned server process $($serverProcess.Id)."
        Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
    }
    Write-LauncherLog "Launcher exiting."
}
