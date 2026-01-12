#!/bin/bash
set -e

VERSION="v2.0.0"
NOTES="docs/v2.0.0_RELEASE_NOTES.md"
DIST_DIR="dist"

# Ensure gh is authenticated (assumed) and default repo set

echo "[INFO] Updating Release Notes on GitHub..."
gh release edit "$VERSION" --title "$VERSION" --notes-file "$NOTES"

echo "[INFO] Uploading Assets..."
gh release upload "$VERSION" \
    "$DIST_DIR/cloudslash_darwin_amd64" \
    "$DIST_DIR/cloudslash_darwin_arm64" \
    "$DIST_DIR/cloudslash_linux_amd64" \
    "$DIST_DIR/cloudslash_linux_arm64" \
    "$DIST_DIR/cloudslash_windows_amd64.exe" \
    "$DIST_DIR/install.sh" \
    "$DIST_DIR/install.ps1" \
    "$DIST_DIR/version.txt" \
    --clobber

echo "[SUCCESS] Release $VERSION updated with all artifacts (including version.txt)!"
