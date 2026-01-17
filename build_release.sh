#!/bin/bash
set -e

echo "Starting Cross-Compilation for CloudSlash v2.1.1..."

# Linux
echo "Building Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -v -o dist/cloudslash_linux_amd64 ./cmd/cloudslash
echo "Building Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -v -o dist/cloudslash_linux_arm64 ./cmd/cloudslash

# macOS
echo "Building Darwin (amd64)..."
GOOS=darwin GOARCH=amd64 go build -v -o dist/cloudslash_darwin_amd64 ./cmd/cloudslash
echo "Building Darwin (arm64)..."
GOOS=darwin GOARCH=arm64 go build -v -o dist/cloudslash_darwin_arm64 ./cmd/cloudslash

# Windows
echo "Building Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -v -o dist/cloudslash_windows_amd64.exe ./cmd/cloudslash

echo "Packaging Artifacts..."
echo "v2.1.1" > dist/version.txt
cp scripts/install.sh dist/install.sh
cp scripts/install.ps1 dist/install.ps1
cp LICENSE dist/LICENSE
cp docs/v2.1.1_RELEASE_NOTES.md dist/RELEASE_NOTES.md

echo "Generating Checksums..."
cd dist
sha256sum cloudslash_* > SHA256SUMS
cd ..

echo "Build Complete."
ls -lh dist/
