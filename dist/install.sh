#!/bin/bash
set -e

# CloudSlash "One-Liner" Installer
# Fetches binaries directly from the GitHub Repository (main branch).

REPO_USER="DrSkyle"
REPO_NAME="CloudSlash"
BRANCH="main"
BASE_URL="https://raw.githubusercontent.com/$REPO_USER/$REPO_NAME/$BRANCH/dist"

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

if command -v curl >/dev/null 2>&1; then
  if ! sudo curl -f -L -o "$DEST_FILE" "$TARGET_URL"; then
      echo "❌ Download failed! Valid binary not found for $OS-$ARCH at $TARGET_URL"
      exit 1
  fi
elif command -v wget >/dev/null 2>&1; then
  if ! sudo wget -O "$DEST_FILE" "$TARGET_URL"; then
      echo "❌ Download failed! Valid binary not found for $OS-$ARCH at $TARGET_URL"
      exit 1
  fi
else
  echo "❌ Error: curl or wget required."
  exit 1
fi

sudo chmod +x "$DEST_FILE"

echo "✅ Installed successfully to $DEST_FILE"
echo "👉 Run 'cloudslash --help' to start!"
