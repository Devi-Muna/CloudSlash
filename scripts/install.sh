#!/bin/bash
set -e

# ░█▀▀░█░░░█▀▀░█░█░█▀▄░█▀▀░█░░░█▀▀░█▀▀░█░█
# ░█░░░█░░░█░█░█░█░█░█░▀▀█░█░░░█▀▀░▀▀█░█▀█
# ░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀░▀
# CloudSlash Installer (v2025.1.2)
# Precision Engineered. Zero Error.

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

    # -- 2. Resolve Version (Simplified logic to avoid API limits) --
    local RELEASE_TAG="$1"
    local DOWNLOAD_URL

    if [ -z "${RELEASE_TAG}" ]; then
        # Default to 'latest' stable alias
        RELEASE_TAG="latest"
        DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/latest/download/${TARGET_BINARY}"
    else
        DOWNLOAD_URL="https://github.com/${OWNER}/${REPO}/releases/download/${RELEASE_TAG}/${TARGET_BINARY}"
    fi

    log_info "Fetching: ${RELEASE_TAG}"

    # -- 3. Download --
    local TMP_DIR
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf -- "$TMP_DIR"' EXIT

    log_info "Downloading binary..."
    local HTTP_CODE
    # -L follows redirects (crucial for 'latest'), -o saves to file
    HTTP_CODE=$(curl --progress-bar -L -w "%{http_code}" -o "${TMP_DIR}/${BINARY_NAME}" "${DOWNLOAD_URL}")

    if [ "${HTTP_CODE}" -ne 200 ]; then
        log_error "Download failed. (HTTP ${HTTP_CODE})"
        echo "   Target: ${DOWNLOAD_URL}"
        echo "   Please check if the release exists."
        exit 1
    fi

    chmod +x "${TMP_DIR}/${BINARY_NAME}"

    # -- 4. Install --
    log_info "Installing to ${INSTALL_DIR}..."

    if [ -w "${INSTALL_DIR}" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    else
        echo "   (sudo permission required)"
        # FIX: Ensure sudo reads password from tty if script is piped
        sudo mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}" < /dev/tty
    fi

    # -- 5. Verification --
    if [ -x "${INSTALL_DIR}/${BINARY_NAME}" ]; then
        echo ""
        log_success "Installation Complete."
        echo -e "   Run '${BOLD}${BINARY_NAME}${NC}' to start."

        # Path Check Warning
        if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
             echo -e "${CYAN}   [NOTE] ${INSTALL_DIR} is not in your \$PATH.${NC}"
        fi
    else
        log_error "Installation failed."
        exit 1
    fi
}

main "$@"
