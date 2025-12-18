#!/bin/bash
set -e

# CloudSlash Uninstaller

DEST_DIR="/usr/local/bin"
DEST_FILE="${DEST_DIR}/cloudslash"

echo "Removing CloudSlash from $DEST_FILE..."

if [ -f "$DEST_FILE" ]; then
    if [ -w "$DEST_DIR" ]; then
        rm "$DEST_FILE"
    else
        sudo rm "$DEST_FILE"
    fi
    echo "✅ Binary removed."
else
    echo "Binary not found at $DEST_FILE."
fi

# Optional cleanup of config/logs if known paths exist
# e.g., ~/.cloudslash or similar?
# App uses ~/.aws/config, doesn't seem to store local config except maybe tfstate in PWD?
# Leaving as binary removal only.

echo "Uninstallation complete."
