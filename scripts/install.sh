#!/usr/bin/env sh
# install.sh — Download and install dtwiz for the current platform.
#
# Usage (recommended — makes dtwiz available immediately without reopening your terminal):
#   source <(curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh)
#
# Alternative (requires opening a new terminal afterwards):
#   curl -sSL https://raw.githubusercontent.com/dynatrace-oss/dtwiz/main/scripts/install.sh | sh
#
# Requires bash or zsh. Pass --install-dir <dir> to override the install location.
# By default the binary is installed to /usr/local/bin (requires sudo) or
# ~/bin if /usr/local/bin is not writable.
#
# The script requires curl.

REPO="dynatrace-oss/dtwiz"

# ── Parse known flags ──────────────────────────────────────────────────────────
INSTALL_DIR=""

while [ $# -gt 0 ]; do
    case "$1" in
        --install-dir)
            INSTALL_DIR="$2"; shift 2 ;;
        *)
            echo "Unknown argument: $1" >&2
            exit 1 ;;
    esac
done

# ── Detect OS ─────────────────────────────────────────────────────────────────
OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    *)
        echo "Unsupported OS: $OS_RAW" >&2
        echo "Use install.ps1 on Windows." >&2
        exit 1 ;;
esac

# ── Detect architecture ───────────────────────────────────────────────────────
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64)         ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    *)
        echo "Unsupported architecture: $ARCH_RAW" >&2
        exit 1 ;;
esac

echo "Detected platform: ${OS}/${ARCH}"

# ── Resolve latest release version ────────────────────────────────────────────
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not found." >&2
    exit 1
fi

# Follow the /releases/latest redirect to extract the tag from the final URL.
VERSION="$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/${REPO}/releases/latest" \
    | sed 's|.*/||')"

if [ -z "$VERSION" ]; then
    echo "Error: could not determine the latest dtwiz version." >&2
    exit 1
fi

# ── Determine install directory ────────────────────────────────────────────────
if [ -z "$INSTALL_DIR" ]; then
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/bin"
    fi
fi

# ── Confirm installation ───────────────────────────────────────────────────────
echo ""
echo "This will download and install dtwiz ${VERSION}:"
echo "  - Download from github.com/${REPO}"
echo "  - Install to ${INSTALL_DIR}"
echo "  - Add ${INSTALL_DIR} to your PATH (if not already present)"
echo ""
printf 'Continue? [Y/n] '
read -r REPLY </dev/tty
case "$REPLY" in
    [Nn]|[Nn][Oo])
        echo "Installation cancelled."
        exit 0 ;;
esac

# ── Download and extract ───────────────────────────────────────────────────────
echo ""
echo "Downloading dtwiz ${VERSION}..."

ARCHIVE="dtwiz_${VERSION#v}_${OS}_${ARCH}.tar.gz"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT INT TERM

curl -fsSL \
    "https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}" \
    -o "${WORK_DIR}/${ARCHIVE}"

tar -xzf "${WORK_DIR}/${ARCHIVE}" -C "$WORK_DIR"

if [ ! -f "${WORK_DIR}/dtwiz" ]; then
    echo "Error: dtwiz binary not found after extraction." >&2
    exit 1
fi

chmod +x "${WORK_DIR}/dtwiz"
mkdir -p "$INSTALL_DIR"

# ── Install binary ─────────────────────────────────────────────────────────────
DEST="${INSTALL_DIR}/dtwiz"
if [ -w "$INSTALL_DIR" ]; then
    mv "${WORK_DIR}/dtwiz" "$DEST"
else
    echo "Installing to ${INSTALL_DIR} requires elevated privileges..."
    sudo mv "${WORK_DIR}/dtwiz" "$DEST"
fi

echo ""
echo "dtwiz ${VERSION} installed to ${DEST}"

# ── Add to PATH in shell profile if needed ─────────────────────────────────────
case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        # Already in the current session's PATH — nothing to do.
        ;;
    *)
        # Detect shell profile file
        PROFILE_FILE=""
        case "${SHELL}" in
            */zsh)
                PROFILE_FILE="${HOME}/.zshrc" ;;
            */bash)
                if [ "$(uname -s)" = "Darwin" ]; then
                    PROFILE_FILE="${HOME}/.bash_profile"
                else
                    PROFILE_FILE="${HOME}/.bashrc"
                fi ;;
            *)
                PROFILE_FILE="${HOME}/.profile" ;;
        esac

        EXPORT_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

        if [ -n "$PROFILE_FILE" ]; then
            # Only append if the line isn't already present
            if ! grep -qF "${INSTALL_DIR}" "${PROFILE_FILE}" 2>/dev/null; then
                printf '\n# Added by dtwiz installer\n%s\n' "$EXPORT_LINE" >> "$PROFILE_FILE"
                echo ""
                echo "  Added ${INSTALL_DIR} to PATH in ${PROFILE_FILE}"
            fi
        fi

        # Export PATH into the current shell. This takes effect immediately when
        # the script is sourced (source <(curl ...)); it is a no-op in a subshell.
        export PATH="${INSTALL_DIR}:$PATH"
        ;;
esac
