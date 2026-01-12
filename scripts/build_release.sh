#!/bin/bash
set -e

APP_NAME="cloudslash"
OUTPUT_DIR="dist"
VERSION="v2.0.0"

echo "[BUILD] Cleaning dist/..."
rm -f "$OUTPUT_DIR"/${APP_NAME}_*
rm -f "$OUTPUT_DIR"/version.txt

echo "[BUILD] Generatng version.txt..."
echo "$VERSION" > "$OUTPUT_DIR/version.txt"

echo "[BUILD] Starting Cross-Compilation..."

# Darwin (macOS)
GOOS=darwin GOARCH=amd64 go build -o "$OUTPUT_DIR/${APP_NAME}_darwin_amd64" cmd/cloudslash/main.go
echo " -> Built darwin_amd64"
GOOS=darwin GOARCH=arm64 go build -o "$OUTPUT_DIR/${APP_NAME}_darwin_arm64" cmd/cloudslash/main.go
echo " -> Built darwin_arm64"

# Linux
GOOS=linux GOARCH=amd64 go build -o "$OUTPUT_DIR/${APP_NAME}_linux_amd64" cmd/cloudslash/main.go
echo " -> Built linux_amd64"
GOOS=linux GOARCH=arm64 go build -o "$OUTPUT_DIR/${APP_NAME}_linux_arm64" cmd/cloudslash/main.go
echo " -> Built linux_arm64"

# Windows
GOOS=windows GOARCH=amd64 go build -o "$OUTPUT_DIR/${APP_NAME}_windows_amd64.exe" cmd/cloudslash/main.go
echo " -> Built windows_amd64.exe"

echo "[BUILD] Success! Artifacts in $OUTPUT_DIR/"
ls -lh "$OUTPUT_DIR"
