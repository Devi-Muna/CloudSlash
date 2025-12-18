# CloudSlash Windows Installer
# Fetches exe directly from GitHub (main branch)

$RepoUser = "DrSkyle"
$RepoName = "CloudSlash"
$Branch = "main"
$BaseUrl = "https://raw.githubusercontent.com/$RepoUser/$RepoName/$Branch/dist"

$BinaryName = "cloudslash-windows-amd64.exe"
$TargetUrl = "$BaseUrl/$BinaryName"
$DestDir = $env:LOCALAPPDATA + "\CloudSlash"
$DestFile = "$DestDir\cloudslash.exe"

Write-Host "🔍 Detected System: Windows (amd64)"
Write-Host "🚀 Downloading from GitHub..."

if (-not (Test-Path -Path $DestDir)) {
    New-Item -ItemType Directory -Path $DestDir | Out-Null
}

try {
    Invoke-WebRequest -Uri $TargetUrl -OutFile $DestFile -ErrorAction Stop
}
catch {
    Write-Error "❌ Download failed! Could not fetch $TargetUrl"
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
