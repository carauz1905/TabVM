#Requires -Version 5.1
<#
.SYNOPSIS
    Starts the TabVM desktop agent and web UI in development mode on Windows.
.DESCRIPTION
    Runs prerequisite checks, installs frontend dependencies, generates a per-run
    development session token, then launches the Go agent in one window and the
    Vite dev server in another. Both processes are local-only. Press Ctrl+C in each
    window to stop.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")
$PrereqScript = Join-Path $ScriptDir "check-prereqs.ps1"

& $PrereqScript
if ($LASTEXITCODE -ne 0) {
    Write-Warning "Prerequisite check failed. Resolve the issues above and try again."
    exit 1
}

function New-RandomSessionToken {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    $rng.GetBytes($bytes)
    $rng.Dispose()
    return ([System.BitConverter]::ToString($bytes) -replace '-', '')
}

$token = New-RandomSessionToken
$env:TABVM_AGENT_SESSION_TOKEN = $token
$env:TabVM__Agent__SessionToken = $token
$env:VITE_TABVM_SESSION_TOKEN = $token
Write-Output "[INFO] Generated a per-run development session token."

$agentDir = Join-Path $RepoRoot "apps\desktop-agent"
$uiDir = Join-Path $RepoRoot "apps\web-ui"

Write-Output "[INFO] Installing web UI dependencies..."
$lockFile = Join-Path $RepoRoot "package-lock.json"
$npmArgs = if (Test-Path -LiteralPath $lockFile) { @("ci") } else { @("install") }

$npmProcess = Start-Process `
    -FilePath "npm" `
    -WorkingDirectory $RepoRoot `
    -ArgumentList $npmArgs `
    -Wait `
    -NoNewWindow `
    -PassThru

if ($npmProcess.ExitCode -ne 0) {
    Write-Warning "npm $($npmArgs -join ' ') failed with exit code $($npmProcess.ExitCode). Fix the dependency issues before starting TabVM."
    exit 1
}

Write-Output "[INFO] Starting TabVM desktop agent..."
$agentProcess = Start-Process `
    -FilePath "go" `
    -WorkingDirectory $agentDir `
    -ArgumentList @("run", ".") `
    -PassThru

function Wait-ForAgentReady {
    param (
        [Parameter(Mandatory = $true)]
        [System.Diagnostics.Process]$Process,

        [Parameter(Mandatory = $true)]
        [string]$SessionToken,

        [Parameter(Mandatory = $true)]
        [int]$TimeoutSeconds
    )

    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        if ($Process.HasExited) {
            return $false
        }

        try {
            $headers = @{ "X-TabVM-Session-Token" = $SessionToken }
            $response = Invoke-WebRequest `
                -Uri "http://127.0.0.1:5230/api/vbox/discovery" `
                -Headers $headers `
                -UseBasicParsing `
                -TimeoutSec 1 `
                -ErrorAction Stop

            if ($response.StatusCode -eq 200) {
                return $true
            }
        }
        catch {
            $statusCode = $null
            if ($_.Exception.Response -and $_.Exception.Response.StatusCode) {
                $statusCode = [int]$_.Exception.Response.StatusCode
            }

            # A 503 from the authenticated discovery endpoint still proves the
            # newly launched agent accepted this run's token. It usually means
            # VirtualBox/VBoxManage is not installed yet, which is not a launcher
            # readiness failure.
            if ($statusCode -eq 503) {
                return $true
            }

            # Agent is not ready yet, or another service is answering on the port.
            # Keep polling until the launched process exits or the timeout expires.
        }

        Start-Sleep -Milliseconds 250
    }

    return $false
}

$agentReady = Wait-ForAgentReady -Process $agentProcess -SessionToken $token -TimeoutSeconds 30
if (-not $agentReady) {
    if (-not $agentProcess.HasExited) {
        $agentProcess | Stop-Process -Force -ErrorAction SilentlyContinue
    }

    if ($agentProcess.HasExited) {
        Write-Warning "Agent process exited early with code $($agentProcess.ExitCode)."
    }
    else {
        Write-Warning "Agent did not become healthy within the timeout."
    }

    exit 1
}

Write-Output "[INFO] Starting TabVM web UI..."
Start-Process `
    -FilePath "npm" `
    -WorkingDirectory $uiDir `
    -ArgumentList @("run", "dev")

Write-Output "[OK] Dev processes started. Open http://localhost:5173 in your browser."
