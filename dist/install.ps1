# CloudSlash Installer (PowerShell)
# Precision Engineered. Zero Error.

$ErrorActionPreference = "Stop"

$Owner = "DrSkyle"
$Repo = "CloudSlash"
$BinaryName = "cloudslash"

Write-Host "🔮 CloudSlash Installer" -ForegroundColor Cyan

# 1. Detect Architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

# 2. Construct Download URL
# Expects naming convention: cloudslash_windows_amd64.exe
$TargetBinary = "${BinaryName}_windows_${Arch}.exe"
$Url = "https://github.com/$Owner/$Repo/releases/latest/download/$TargetBinary"

Write-Host "   Target: $TargetBinary" -ForegroundColor Gray
Write-Host "   Source: $Url" -ForegroundColor Gray

# 3. Setup Install Directory
$InstallDir = "$env:LOCALAPPDATA\CloudSlash"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$OutputPath = Join-Path $InstallDir "${BinaryName}.exe"

# 4. Download
Write-Host "   Downloading..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $Url -OutFile $OutputPath
} catch {
    Write-Host "✖  Download failed. Please check if the release asset exists." -ForegroundColor Red
    Write-Host "   Error: $_" -ForegroundColor Red
    exit 1
}

# 5. Add to PATH (User Scope)
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "   Adding to PATH..." -ForegroundColor Yellow
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    $env:Path += ";$InstallDir"
}

Write-Host "✔  Installation Complete!" -ForegroundColor Green
Write-Host "   Run '$BinaryName' to start (you may need to restart your terminal)." -ForegroundColor Cyan
