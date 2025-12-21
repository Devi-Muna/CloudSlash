#!/bin/bash
set -e

# ░█▀▀░█░░░█▀▀░█░█░█▀▄░█▀▀░█░░░█▀▀░█▀▀░█░█
# ░█░░░█░░░█░█░█░█░█░█░▀▀█░█░░░█▀▀░▀▀█░█▀█
# ░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀░▀
# CloudSlash Installer (v2025.1.1)
# Precision Engineered. Zero Error.

# Wrap everything in a function to ensure the script is fully downloaded
# before execution. This prevents "pipe faulures" if sudo consumes stdin.
main() {
    local OWNER="DrSkyle"
    local REPO="CloudSlash"
    local BINARY_NAME="cloudslash"
    local INSTALL_DIR="/usr/local/bin"

    # -- Color & UI --
    local BOLD="\033[1m"
    local GREEN="\033[0;32m"
    local RED="\033[0;31m"
    local CYAN="\033[0;36m"
    local NC="\033[0m" # No Color

    log_info() { echo -e "${CYAN}ℹ  $1${NC}"; }
    log_success() { echo -e "${GREEN}✔  $1${NC}"; }
    log_error() { echo -e "${RED}✖  $1${NC}"; }

    # -- 1. Environment Detection --
    local OS
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    local ARCH
    ARCH="$(uname -m)"

    case "${OS}" in
        linux)  ;;
        darwin) ;;
        *)      log_error "OS '${OS}' not supported."; exit 1 ;;
    esac

    # Normalize Arch
    case "${ARCH}" in
        x86_64)    ARCH="amd64" ;;
        arm64)     ARCH="arm64" ;;
        aarch64)   ARCH="arm64" ;;
        *)         log_error "Architecture '${ARCH}' not supported."; exit 1 ;;
    esac

    local TARGET_BINARY="${BINARY_NAME}_${OS}_${ARCH}"

    echo -e "
${BOLD}CloudSlash Installer${NC}
===================="
    log_info "Detected: ${OS} / ${ARCH}"

    # -- 2. Resolve Version --
    local RELEASE_TAG="$1"
    
    if [ -z "${RELEASE_TAG}" ]; then
        # Try to get the latest tag (including pre-releases) from GitHub API
        # We verify if we are in a rate-limit scenario or network failure by checking empty output
        local API_RESPONSE
        API_RESPONSE=$(curl -s "https://api.github.com/repos/${OWNER}/${REPO}/releases")
        
        # Extract the first "tag_name": "..." occurrence
        RELEASE_TAG=$(echo "$API_RESPONSE" | grep -o '"tag_name": "[^"]*"' | head -n 1 | cut -d '"' -f 4)
        
        if [ -z "${RELEASE_TAG}" ]; then
            echo "   (Warning: Could not resolve latest version via API. Defaulting to 'latest' stable alias.)"
            RELEASE_TAG="latest"
        else
             log_info "Resolved Latest Ver: ${RELEASE_TAG}"
        fi
    fi

    # -- 3. Construct Download URL --
    local DOWNLOAD_URL
    if [ "${RELEASE_TAG}" = "latest" ]; then
        # Use the magic 'latest' endpoint which redirects to the latest stable release
        DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/latest/download/${TARGET_BINARY}"
    else
        # Use the specific tag endpoint
        DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/download/${RELEASE_TAG}/${TARGET_BINARY}"
    fi

    log_info "Fetching: ${DOWNLOAD_URL}"

    # -- 3. Download --
    local TMP_DIR
    TMP_DIR=$(mktemp -d)
    # Ensure cleanup happens even if we exit early
    trap 'rm -rf -- "$TMP_DIR"' EXIT

    # Download with progress bar to stderr, capture HTTP status code to stdout
    log_info "Downloading binary..."
    local HTTP_CODE
    HTTP_CODE=$(curl --progress-bar -L -w "%{http_code}" -o "${TMP_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}")

    if [ "${HTTP_CODE}" -ne 200 ]; then
        log_error "Download failed. (HTTP ${HTTP_CODE})"
        echo "   URL: ${DOWNLOAD_URL}"
        echo "   Please check if the release for your platform exists."
        exit 1
    fi

    chmod +x "${TMP_DIR}/${BINARY_NAME}"

    # -- 4. Install --
    log_info "Installing to ${INSTALL_DIR}..."

    if [ -w "${INSTALL_DIR}" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        # Need sudo
        echo "   (sudo permission required)"
        # Use -p to prompt nicely, but sudo usually handles it.
        # We perform the move.
        sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # -- 5. Verification --
    # Check the installed binary directly using absolute path
    if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        local VERSION
        VERSION=$("${INSTALL_DIR}/${BINARY_NAME}" --version 2>/dev/null || echo "v2025.1")
        echo ""
        log_success "Installation Complete: ${VERSION}"
        echo -e "   Run '${BOLD}${BINARY_NAME}${NC}' to start."
    else
        log_error "Installation failed. '${INSTALL_DIR}/${BINARY_NAME}' not executable or found."
        exit 1
    fi
}

# Execute main
main "$@"
