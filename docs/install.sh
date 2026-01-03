#!/bin/bash

# CloudSlash Installer (Linux & macOS)

set -e

REPO="DrSkyle/CloudSlash"
VERSION="latest" # or specific tag
BINARY_NAME="cloudslash"

# Detect OS
OS="$(uname -s)"
case "${OS}" in
    Linux*)     OS="linux";;
    Darwin*)    OS="darwin";;
    *)          echo "Unsupported OS: ${OS}"; exit 1;;
esac

# Detect Arch
ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)    ARCH="amd64";;
    arm64)     ARCH="arm64";;
    aarch64)   ARCH="arm64";;
    *)         echo "Unsupported Architecture: ${ARCH}"; exit 1;;
esac

echo "✨ Installing CloudSlash for ${OS}/${ARCH}..."

# Construct URL (GitHub Releases)
# Example: https://github.com/DrSkyle/CloudSlash/releases/latest/download/cloudslash_linux_amd64
DOWNLOAD_URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY_NAME}_${OS}_${ARCH}"

# Install Directory
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
    # Ensure it's in path
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        echo "Adding $INSTALL_DIR to PATH..."
        echo "export PATH=\$PATH:$INSTALL_DIR" >> "$HOME/.bashrc"
        echo "export PATH=\$PATH:$INSTALL_DIR" >> "$HOME/.zshrc"
    fi
fi

# Download
echo ""
echo "   Downloading CloudSlash..."
echo "   Src: $DOWNLOAD_URL"
echo ""

# Use curl with progress bar (#), fail fast (f), follow redirects (L)
if curl -# -fL -o "${INSTALL_DIR}/${BINARY_NAME}" "$DOWNLOAD_URL"; then
    echo ""
    echo "   [OK] Download Complete."
else
    echo ""
    echo "   [ERROR] Download failed. Please check your internet or the release version."
    exit 1
fi

# Make Executable
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo ""
echo "✅ Installation Complete!"
echo "Run 'cloudslash' to get started."
echo ""
