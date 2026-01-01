<#
.SYNOPSIS
    CloudSlash Installer (Windows)
    Precision Engineered. Zero Error.

.DESCRIPTION
    Downloads and installs the latest CloudSlash binary for Windows.
    Adds the installation directory to the User PATH if missing.

.EXAMPLE
    irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/install.ps1 | iex
#>

$ErrorActionPreference = "Stop"

function Write-Info    { param($Msg); Write-Host "i  $Msg" -ForegroundColor Cyan }
function Write-Success { param($Msg); Write-Host "ok $Msg" -ForegroundColor Green }
function Write-ErrorMsg   { param($Msg); Write-Host "x  $Msg" -ForegroundColor Red }

function Main {
    param (
        [string]$ReleaseTag = "latest"
    )

    $Owner = "DrSkyle"
    $Repo = "CloudSlash"
    $BinaryName = "cloudslash.exe"
    # Install to LocalAppData to avoid needing Admin privileges
    $InstallDir = "$env:LOCALAPPDATA\CloudSlash"

    # -- 1. Header --
    Write-Host ""
    Write-Host "░█▀▀░█░░░█▀▀░█░█░█▀▄░█▀▀░█░░░█▀▀░█░░░█▀▀░█▀▀░█░█" -ForegroundColor Green
    Write-Host "░█░░░█░░░█░█░█░█░█░█░▀▀█░█░░░█▀▀░▀▀█░█▀█" -ForegroundColor Green
    Write-Host "░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀░▀" -ForegroundColor Green
    Write-Host "CloudSlash Installer (Windows)" -ForegroundColor Gray
    Write-Host "==============================" -ForegroundColor Gray
    Write-Host ""

    # -- 2. Architecture Detection --
    $Arch = "amd64"
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $Arch = "arm64"
    }
    Write-Info "Detected System: Windows / $Arch"

    # -- 3. Resolve Download URL --
    $TargetBinary = "cloudslash_windows_${Arch}.exe"
    $DownloadUrl = ""

    if ($ReleaseTag -eq "latest") {
        $DownloadUrl = "https://github.com/$Owner/$Repo/releases/latest/download/$TargetBinary"
    } else {
        $DownloadUrl = "https://github.com/$Owner/$Repo/releases/download/$ReleaseTag/$TargetBinary"
    }

    Write-Info "Fetching Version: $ReleaseTag"

    # -- 4. Download --
    $TempFile = "$env:TEMP\$TargetBinary"
    Write-Info "Downloading binary..."

    try {
        # Use .NET WebClient for better compatibility/speed than Invoke-WebRequest
        $WebClient = New-Object System.Net.WebClient
        $WebClient.DownloadFile($DownloadUrl, $TempFile)
    }
    catch {
        Write-ErrorMsg "Download failed."
        Write-Host "   Target: $DownloadUrl"
        Write-Host "   Error: $_"
        exit 1
    }

    # -- 5. Install --
    if (-not (Test-Path -Path $InstallDir)) {
        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    }

    Write-Info "Installing to $InstallDir..."
    
    # Move and Rename
    Move-Item -Path $TempFile -Destination "$InstallDir\$BinaryName" -Force

    # -- 6. Path Persistence (The Pro Feature) --
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Info "Adding CloudSlash to User PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        $PathUpdated = $true
    } else {
        $PathUpdated = $false
    }

    # -- 7. Verification --
    if (Test-Path "$InstallDir\$BinaryName") {
        Write-Host ""
        Write-Success "Installation Complete."
        
        if ($PathUpdated) {
            Write-Host "   [NOTE] We updated your PATH. Please restart your terminal." -ForegroundColor Yellow
        }
        Write-Host "   Run 'cloudslash' to start."
    } else {
        Write-ErrorMsg "Installation failed. File not found."
        exit 1
    }
}

Main @args
