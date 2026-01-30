#!/bin/bash
set -e

# Read version from Source of Truth
VERSION=$(cat VERSION)
echo "Starting Cross-Compilation for CloudSlash ${VERSION}..."

# Linux
echo "Building Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -ldflags "-X 'github.com/DrSkyle/cloudslash/pkg/version.Current=${VERSION}'" -v -o dist/cloudslash_linux_amd64 ./cmd/cloudslash-cli
echo "Building Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -ldflags "-X 'github.com/DrSkyle/cloudslash/pkg/version.Current=${VERSION}'" -v -o dist/cloudslash_linux_arm64 ./cmd/cloudslash-cli

# macOS
echo "Building Darwin (amd64)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-X 'github.com/DrSkyle/cloudslash/pkg/version.Current=${VERSION}'" -v -o dist/cloudslash_darwin_amd64 ./cmd/cloudslash-cli
echo "Building Darwin (arm64)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-X 'github.com/DrSkyle/cloudslash/pkg/version.Current=${VERSION}'" -v -o dist/cloudslash_darwin_arm64 ./cmd/cloudslash-cli

# Windows
echo "Building Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags "-X 'github.com/DrSkyle/cloudslash/pkg/version.Current=${VERSION}'" -v -o dist/cloudslash_windows_amd64.exe ./cmd/cloudslash-cli

echo "Packaging Artifacts..."
echo "${VERSION}" > dist/version.txt
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
