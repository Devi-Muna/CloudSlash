# CloudSlash Installer (Windows)

$ErrorActionPreference = "Stop"

$Repo = "DrSkyle/CloudSlash"
$BinaryName = "cloudslash.exe"
$InstallDir = "$env:LOCALAPPDATA\CloudSlash"

# Detect Arch (Simplified for Windows usually x64 or arm64)
# Assuming amd64 for now as typical target
$Arch = "windows_amd64.exe" 

Write-Host "✨ Installing CloudSlash..." -ForegroundColor Cyan

# Create Directory
if (!(Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

# Construct URL
$DownloadUrl = "https://github.com/$Repo/releases/latest/download/cloudslash_$Arch"

# Download
Write-Host "Downloading from $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile "$InstallDir\$BinaryName"

# Add to PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding to PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path += ";$InstallDir"
}

Write-Host ""
Write-Host "✅ Installation Complete!" -ForegroundColor Green
Write-Host "Run 'cloudslash' to get started."
Write-Host "(You may need to restart your terminal)"
