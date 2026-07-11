#Requires -Version 5.1
<#
.SYNOPSIS
    Checks that the minimum TabVM development prerequisites are present on Windows.
.DESCRIPTION
    Verifies Go 1.25+ toolchain, Node.js ^20.19.0 or >=22.12.0, npm, and optional VirtualBox/VBoxManage.
    Exits with 0 if the critical tools are present, otherwise exits with 1.
#>
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$script:checks = [System.Collections.Generic.List[object]]::new()

function Get-VersionString {
    param (
        [Parameter(Mandatory = $true)]
        [string]$Command,

        [Parameter(Mandatory = $true)]
        [string]$Argument
    )

    try {
        return & $Command $Argument 2>$null | Select-Object -First 1
    }
    catch {
        return $null
    }
}

function Test-CommandAvailable {
    param (
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [string]$Command
    )

    $found = $null -ne (Get-Command $Command -ErrorAction SilentlyContinue)
    return [PSCustomObject]@{
        Name    = $Name
        Found   = $found
        Version = "-"
    }
}

function Test-Go {
    $result = Test-CommandAvailable -Name "Go" -Command "go"
    if (-not $result.Found) {
        return $result
    }

    $versionString = Get-VersionString -Command "go" -Argument "version"
    $result.Version = if ($versionString) { $versionString } else { "unknown" }

    if (-not $versionString) {
        $result.Found = $false
        return $result
    }

    $match = [regex]::Match($versionString, 'go(\d+)\.(\d+)')
    if (-not $match.Success) {
        $result.Found = $false
        $result.Version = "$versionString (requires Go 1.25+)"
        return $result
    }

    $major = [int]$match.Groups[1].Value
    $minor = [int]$match.Groups[2].Value

    if ($major -lt 1 -or ($major -eq 1 -and $minor -lt 25)) {
        $result.Found = $false
        $result.Version = "$versionString (requires Go 1.25+)"
    }

    return $result
}

function Test-NodeJs {
    $result = Test-CommandAvailable -Name "Node.js" -Command "node"
    if (-not $result.Found) {
        return $result
    }

    $versionString = Get-VersionString -Command "node" -Argument "--version"
    $result.Version = if ($versionString) { $versionString } else { "unknown" }

    if (-not $versionString) {
        $result.Found = $false
        return $result
    }

    $major = 0
    $minor = 0
    if ($versionString -match '^v?(\d+)\.(\d+)') {
        $major = [int]$Matches[1]
        $minor = [int]$Matches[2]
    }

    $nodeOk = ($major -eq 20 -and $minor -ge 19) -or
              ($major -eq 22 -and $minor -ge 12) -or
              ($major -gt 22)

    if (-not $nodeOk) {
        $result.Found = $false
        $result.Version = "$versionString (requires ^20.19.0 or >=22.12.0)"
    }

    return $result
}

function Test-Npm {
    $result = Test-CommandAvailable -Name "npm" -Command "npm"
    if (-not $result.Found) {
        return $result
    }

    $versionString = Get-VersionString -Command "npm" -Argument "--version"
    $result.Version = if ($versionString) { $versionString } else { "unknown" }
    return $result
}

function Test-VirtualBox {
    $vboxPaths = @(
        "C:\Program Files\Oracle\VirtualBox\VBoxManage.exe",
        "C:\Program Files (x86)\Oracle\VirtualBox\VBoxManage.exe"
    )

    $found = $false
    foreach ($path in $vboxPaths) {
        if (Test-Path -LiteralPath $path) {
            $found = $true
            break
        }
    }

    return [PSCustomObject]@{
        Name    = "VirtualBox / VBoxManage"
        Found   = $found
        Version = "-"
    }
}

function Test-GuacamolePreflight {
    $javaPath = $env:TABVM_JAVA_PATH
    if (-not $javaPath) {
        $javaPath = "java"
    }

    $guacdPath = $env:TABVM_GUACD_PATH
    if (-not $guacdPath) {
        $guacdPath = "guacd"
    }

    $javaFound = $null -ne (Get-Command $javaPath -ErrorAction SilentlyContinue)
    $guacdFound = $null -ne (Get-Command $guacdPath -ErrorAction SilentlyContinue)
    $tomcatConfigured = ($env:TABVM_TOMCAT_HOME -and (Test-Path -LiteralPath $env:TABVM_TOMCAT_HOME -PathType Container))
    $warConfigured = ($env:TABVM_GUACAMOLE_WAR_PATH -and (Test-Path -LiteralPath $env:TABVM_GUACAMOLE_WAR_PATH -PathType Leaf))

    $parts = @()
    if ($javaFound) { $parts += "java" }
    if ($tomcatConfigured) { $parts += "tomcat" }
    if ($warConfigured) { $parts += "war" }
    if ($guacdFound) { $parts += "guacd" }

    # Ambient Java by itself does not mean Guacamole is partially present.
    # Treat the optional preflight as complete only when all four components
    # are detected or configured.
    $found = $javaFound -and $tomcatConfigured -and $warConfigured -and $guacdFound

    if ($found) {
        $version = $parts -join ", "
    }
    elseif ($parts.Count -gt 0) {
        $version = "configuration not complete ($($parts -join ', '))"
    }
    elseif ($env:TABVM_TOMCAT_HOME -or $env:TABVM_GUACAMOLE_WAR_PATH -or $env:TABVM_GUACD_PATH -or $env:TABVM_JAVA_PATH) {
        $version = "configured paths not found"
    }
    else {
        $version = "not configured"
    }

    return [PSCustomObject]@{
        Name    = "Guacamole preflight (optional)"
        Found   = $found
        Version = $version
    }
}

$script:checks.Add((Test-Go))
$script:checks.Add((Test-NodeJs))
$script:checks.Add((Test-Npm))
$script:checks.Add((Test-VirtualBox))
$script:checks.Add((Test-GuacamolePreflight))

Write-Output ""
Write-Output "TabVM prerequisite check"
Write-Output "------------------------"
foreach ($check in $script:checks) {
    $symbol = if ($check.Found) { "[OK]" } else { "[!]" }
    Write-Output "$symbol $($check.Name): $($check.Version)"
}
Write-Output ""

$goOk = ($script:checks | Where-Object { $_.Name -eq "Go" }).Found
$nodeOk = ($script:checks | Where-Object { $_.Name -eq "Node.js" }).Found
$npmOk = ($script:checks | Where-Object { $_.Name -eq "npm" }).Found
$vboxOk = ($script:checks | Where-Object { $_.Name -eq "VirtualBox / VBoxManage" }).Found

if (-not $goOk) {
    Write-Warning "Go is required to build the desktop agent. Download from https://go.dev/dl/"
}
if (-not ($nodeOk -and $npmOk)) {
    Write-Warning "Node.js ^20.19.0 or >=22.12.0 and npm are required to build the web UI. Download from https://nodejs.org/"
}
if (-not $vboxOk) {
    Write-Warning "VirtualBox / VBoxManage not found in default paths. VM discovery will report not found."
}

if ($goOk -and $nodeOk -and $npmOk) {
    Write-Output "[OK] Critical development tools are present."
    exit 0
}
else {
    Write-Warning "One or more critical tools are missing."
    exit 1
}
