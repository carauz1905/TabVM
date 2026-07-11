#Requires -Version 5.1
<#
.SYNOPSIS
    Builds a release of TabVM: a self-contained agent (with the web UI embedded),
    the double-click launcher, a portable ZIP, and — if Inno Setup is installed —
    a Windows installer.
.DESCRIPTION
    Steps:
      1. Build the web UI (apps/web-ui -> dist).
      2. Copy the built UI into the agent's embed directory.
      3. Build tabvm-agent.exe and TabVM.exe (launcher) as windowed binaries.
      4. Stage README + icon and produce dist/TabVM-portable.zip.
      5. If ISCC (Inno Setup) is found, compile installer/tabvm.iss.
    The target machine only needs Oracle VirtualBox installed.
.PARAMETER SkipInstaller
    Build the portable ZIP only; do not attempt to compile the installer.
#>
param(
    [switch]$SkipInstaller
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir "..")
$AgentDir = Join-Path $RepoRoot "apps\desktop-agent"
$UiDir = Join-Path $RepoRoot "apps\web-ui"
$EmbedDir = Join-Path $AgentDir "internal\webui\dist"
$DistDir = Join-Path $RepoRoot "dist"
$StageDir = Join-Path $DistDir "TabVM"

Write-Output "[1/5] Building web UI..."
Push-Location $RepoRoot
try {
    $lockFile = Join-Path $RepoRoot "package-lock.json"
    if (Test-Path -LiteralPath $lockFile) { npm ci } else { npm install }
    if ($LASTEXITCODE -ne 0) { throw "npm install failed" }
}
finally { Pop-Location }

Push-Location $UiDir
try {
    npm run build
    if ($LASTEXITCODE -ne 0) { throw "web UI build failed" }
}
finally { Pop-Location }

Write-Output "[2/5] Embedding web UI into the agent..."
if (Test-Path -LiteralPath $EmbedDir) { Remove-Item -Recurse -Force $EmbedDir }
Copy-Item -Recurse -Force (Join-Path $UiDir "dist") $EmbedDir

Write-Output "[3/5] Building agent and launcher (windowed)..."
if (Test-Path -LiteralPath $StageDir) { Remove-Item -Recurse -Force $StageDir }
New-Item -ItemType Directory -Path $StageDir | Out-Null

Push-Location $AgentDir
try {
    $env:CGO_ENABLED = "0"
    go build -ldflags "-H=windowsgui" -o (Join-Path $StageDir "tabvm-agent.exe") .
    if ($LASTEXITCODE -ne 0) { throw "agent build failed" }
    go build -ldflags "-H=windowsgui" -o (Join-Path $StageDir "TabVM.exe") ".\cmd\launcher"
    if ($LASTEXITCODE -ne 0) { throw "launcher build failed" }
}
finally { Pop-Location }

Write-Output "[4/5] Staging README + icon and zipping..."
$readmeSource = Join-Path $ScriptDir "release-README.txt"
if (Test-Path -LiteralPath $readmeSource) {
    Copy-Item -Force $readmeSource (Join-Path $StageDir "README.txt")
}
$iconSource = Join-Path $RepoRoot "branding\icon\tabvm.ico"
if (Test-Path -LiteralPath $iconSource) {
    Copy-Item -Force $iconSource (Join-Path $StageDir "tabvm.ico")
}

$zipPath = Join-Path $DistDir "TabVM-portable.zip"
if (Test-Path -LiteralPath $zipPath) { Remove-Item -Force $zipPath }
Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $zipPath -Force
Write-Output "[OK] Portable package: $zipPath"

if ($SkipInstaller) {
    Write-Output "[5/5] Skipping installer (--SkipInstaller)."
    exit 0
}

Write-Output "[5/5] Compiling installer (if Inno Setup is available)..."
$iscc = Get-Command "ISCC.exe" -ErrorAction SilentlyContinue
if (-not $iscc) {
    $default = "C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
    if (Test-Path -LiteralPath $default) { $iscc = $default } else { $iscc = $null }
}
if ($iscc) {
    & $iscc (Join-Path $RepoRoot "installer\tabvm.iss")
    if ($LASTEXITCODE -ne 0) { throw "installer compilation failed" }
    Write-Output "[OK] Installer written to dist\ (see installer\tabvm.iss OutputDir)."
}
else {
    Write-Warning "Inno Setup (ISCC.exe) not found. Portable ZIP built; skipping installer."
    Write-Warning "Install Inno Setup 6 and re-run to produce TabVM-Setup.exe."
}
