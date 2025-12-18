# CloudSlash Windows Installer
# Fetches exe directly from GitHub (main branch)

$RepoUser = "DrSkyle"
$RepoName = "CloudSlash"
$RepoUser = "DrSkyle"
$RepoName = "CloudSlash"
$BaseUrl = "https://github.com/$RepoUser/$RepoName/releases/latest/download"

$BinaryName = "cloudslash-windows-amd64.exe"
$TargetUrl = "$BaseUrl/$BinaryName"
$DestDir = $env:LOCALAPPDATA + "\CloudSlash"
$DestFile = "$DestDir\cloudslash.exe"

Write-Host "🔍 Detected System: Windows (amd64)"
Write-Host "🚀 Downloading from GitHub..."

if (-not (Test-Path -Path $DestDir)) {
    New-Item -ItemType Directory -Path $DestDir | Out-Null
}

# Handle updates for running binaries (Rename Trick)
if (Test-Path -Path $DestFile) {
    $OldFile = "$DestFile.old"
    if (Test-Path -Path $OldFile) {
        Remove-Item -Path $OldFile -Force -ErrorAction SilentlyContinue
    }
    try {
        Rename-Item -Path $DestFile -NewName $OldFile -Force -ErrorAction Stop
    } catch {
        Write-Warning "⚠️  Could not rename existing binary (it might be locked). Attempting overwrite..."
    }
}

try {
    Write-Host "⬇️  Downloading v1.0.2..."
    Invoke-WebRequest -Uri $TargetUrl -OutFile $DestFile -ErrorAction Stop
}
catch {
    Write-Error "❌ Download failed! Could not fetch $TargetUrl"
    # Try to restore backup if download failed
    if (Test-Path -Path "$DestFile.old") {
        Rename-Item -Path "$DestFile.old" -NewName $DestFile -Force
    }
    exit 1
}

# Add to PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$DestDir*") {
    Write-Host "🔧 Adding $DestDir to User PATH..."
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$DestDir", "User")
    $env:Path += ";$DestDir"
    Write-Host "✅ Added to PATH. (You may need to restart your terminal)"
}

Write-Host "✅ Installation complete!"
Write-Host "👉 Run 'cloudslash' to start!"
