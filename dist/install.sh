#!/bin/bash
set -e

# CloudSlash "One-Liner" Installer
# Fetches binaries from GitHub Releases (latest).

REPO_USER="DrSkyle"
REPO_NAME="CloudSlash"
BASE_URL="https://github.com/$REPO_USER/$REPO_NAME/releases/latest/download"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

if [ "$ARCH" == "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" == "aarch64" ] || [ "$ARCH" == "arm64" ]; then
  ARCH="arm64"
else
  echo "❌ Unsupported architecture: $ARCH"
  exit 1
fi

BINARY_NAME="cloudslash-${OS}-${ARCH}"
TARGET_URL="${BASE_URL}/${BINARY_NAME}"
DEST_DIR="/usr/local/bin"
DEST_FILE="${DEST_DIR}/cloudslash"

echo "🔍 Detected System: $OS ($ARCH)"
echo "🚀 Downloading from: $BASE_URL ..."

TMP_FILE="${DEST_FILE}.tmp"

if command -v curl >/dev/null 2>&1; then
  if ! sudo curl -f -L -o "$TMP_FILE" "$TARGET_URL"; then
      echo "❌ Download failed! Valid binary not found for $OS-$ARCH at $TARGET_URL"
      rm -f "$TMP_FILE"
      exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! sudo wget -O "$TMP_FILE" "$TARGET_URL"; then
      echo "❌ Download failed! Valid binary not found for $OS-$ARCH at $TARGET_URL"
      rm -f "$TMP_FILE"
      exit 1
  fi
else
  echo "❌ Error: curl or wget required."
  exit 1
fi

sudo chmod +x "$TMP_FILE"

# Explicitly remove old version to prevent overwriting issues
if [ -f "$DEST_FILE" ]; then
    sudo rm -f "$DEST_FILE"
fi

sudo mv "$TMP_FILE" "$DEST_FILE"

echo "✅ Installed successfully to $DEST_FILE"
echo "👉 Run 'cloudslash --help' to start!"
