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
#
# Install channels:
#   (default)            Latest stable release
#   DTWIZ_CHANNEL=main   Latest snapshot from the main branch
#   DTWIZ_TAG=<tag>      Specific snapshot/pre-release tag
#
# Self-monitoring:
#   When DT_ENVIRONMENT is set, the script sends a self-monitoring event to that
#   Dynatrace environment when it starts and when it completes, fails, or is
#   cancelled. DT_PLATFORM_TOKEN is used for authorization when set.

REPO="dynatrace-oss/dtwiz"

# Release tag to install. If set, a snapshot pre-release with that tag is used
# instead of the latest stable release. Set via the DTWIZ_TAG env variable.
# Example: DTWIZ_TAG=snapshot-preview-my-branch source <(curl -sSL ...)
TAG="${DTWIZ_TAG:-}"

# Install channel. Set to "main" to install the latest snapshot from the main
# branch. Overrides DTWIZ_TAG when set to a known channel name.
# Example: DTWIZ_CHANNEL=main source <(curl -sSL ...)
CHANNEL="${DTWIZ_CHANNEL:-}"
if [ "$CHANNEL" = "main" ]; then
    TAG="snapshot-main"
fi

# ── Self-monitoring ────────────────────────────────────────────────────────────
# Execution id correlating the events of one run: 3 random hex chars.
DTWIZ_SM_EXEC_ID="$(od -An -N2 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' | cut -c1-3)"
[ -n "$DTWIZ_SM_EXEC_ID" ] || DTWIZ_SM_EXEC_ID="000"

DTWIZ_SM_OS="$(uname -s 2>/dev/null | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_.-')"
case "$DTWIZ_SM_OS" in
    darwin) DTWIZ_SM_OS_SHORT="mac" ;;
    linux)  DTWIZ_SM_OS_SHORT="lin" ;;
    *)      DTWIZ_SM_OS_SHORT="$(printf '%s' "$DTWIZ_SM_OS" | cut -c1-3)" ;;
esac

if [ "$CHANNEL" = "main" ]; then
    DTWIZ_SM_CHANNEL="main"
elif [ -n "$TAG" ]; then
    DTWIZ_SM_CHANNEL="tag"
else
    DTWIZ_SM_CHANNEL="stable"
fi
# The tag is only used in the JSON body; drop characters that would break it.
DTWIZ_SM_TAG="$(printf '%s' "$TAG" | tr -d '"\\' | tr -d '[:cntrl:]')"

# Resolved dtwiz version without the leading "v"; empty until resolved.
DTWIZ_SM_VERSION=""

# dtwiz_send_event <step> [reason]
# Sends a self-monitoring event to DT_ENVIRONMENT. Silent, bounded to 3 seconds,
# and a no-op when DT_ENVIRONMENT is not set. Never affects the install.
dtwiz_send_event() {
    [ -n "${DT_ENVIRONMENT:-}" ] || return 0

    _dtwiz_sm_step="$1"
    _dtwiz_sm_reason="${2:-}"
    case "$_dtwiz_sm_step" in
        inv) _dtwiz_sm_step_name="invoked" ;;
        com) _dtwiz_sm_step_name="completed" ;;
        fai) _dtwiz_sm_step_name="failed" ;;
        can) _dtwiz_sm_step_name="cancelled" ;;
        *)   _dtwiz_sm_step_name="$_dtwiz_sm_step" ;;
    esac

    # Same normalization as the binary: classic URL, no trailing slash.
    _dtwiz_sm_url="$(printf '%s' "$DT_ENVIRONMENT" | sed 's|\.apps\.|.|; s|/*$||')/api/v2/events/ingest"

    _dtwiz_sm_ua="dtwiz/install;s=sh;st=${_dtwiz_sm_step}"
    [ -n "$DTWIZ_SM_VERSION" ] && _dtwiz_sm_ua="${_dtwiz_sm_ua};v=${DTWIZ_SM_VERSION}"
    [ -n "$_dtwiz_sm_reason" ] && _dtwiz_sm_ua="${_dtwiz_sm_ua};er=${_dtwiz_sm_reason}"

    _dtwiz_sm_props="\"executionId\":\"${DTWIZ_SM_EXEC_ID}\",\"script\":\"sh\",\"step\":\"${_dtwiz_sm_step_name}\",\"os\":\"${DTWIZ_SM_OS}\",\"channel\":\"${DTWIZ_SM_CHANNEL}\""
    [ -n "$DTWIZ_SM_TAG" ] && _dtwiz_sm_props="${_dtwiz_sm_props},\"tag\":\"${DTWIZ_SM_TAG}\""
    [ -n "$DTWIZ_SM_VERSION" ] && _dtwiz_sm_props="${_dtwiz_sm_props},\"version\":\"${DTWIZ_SM_VERSION}\""
    [ -n "$_dtwiz_sm_reason" ] && _dtwiz_sm_props="${_dtwiz_sm_props},\"error\":\"${_dtwiz_sm_reason}\""
    _dtwiz_sm_body="{\"eventType\":\"CUSTOM_INFO\",\"title\":\"dtwiz install script\",\"properties\":{${_dtwiz_sm_props}}}"

    if command -v curl >/dev/null 2>&1; then
        # The Authorization header is read from stdin so the token never shows up
        # in the process list. Without a token the event is still sent: the
        # environment's edge records the headers even when ingest rejects it.
        if [ -n "${DT_PLATFORM_TOKEN:-}" ]; then
            printf 'Authorization: Bearer %s\n' "$DT_PLATFORM_TOKEN"
        fi | curl -s -o /dev/null --connect-timeout 2 --max-time 3 \
            -X POST "$_dtwiz_sm_url" \
            -H @- \
            -H "Content-Type: application/json" \
            -H "User-Agent: ${_dtwiz_sm_ua}" \
            -H "Tab-Id: ${DTWIZ_SM_EXEC_ID};o=${DTWIZ_SM_OS_SHORT}" \
            -H "dtwiz-monitoring: dtwiz-start" \
            --data "$_dtwiz_sm_body" >/dev/null 2>&1 || true
    elif command -v wget >/dev/null 2>&1; then
        # Fallback so a missing curl can still be reported. wget can only take
        # headers on the command line, so the token is deliberately not sent.
        wget -q -O /dev/null --timeout=3 --tries=1 \
            --user-agent="$_dtwiz_sm_ua" \
            --header="Content-Type: application/json" \
            --header="Tab-Id: ${DTWIZ_SM_EXEC_ID};o=${DTWIZ_SM_OS_SHORT}" \
            --header="dtwiz-monitoring: dtwiz-start" \
            --post-data="$_dtwiz_sm_body" \
            "$_dtwiz_sm_url" >/dev/null 2>&1 || true
    fi
    return 0
}

# dtwiz_fail <reason> <message line>...
# Prints the message to stderr, reports the failure, and exits.
dtwiz_fail() {
    _dtwiz_fail_reason="$1"
    shift
    for _dtwiz_fail_line in "$@"; do
        echo "$_dtwiz_fail_line" >&2
    done
    dtwiz_send_event fai "$_dtwiz_fail_reason"
    exit 1
}

# dtwiz_check_github_reachable <HTTP status reported by curl>
# Fails with github_unreachable when GitHub gave no final answer: curl reports
# 000 when no response arrived (DNS, connect, timeout, TLS, proxy) and a 3xx
# when a redirect could not be followed. A 2xx, 4xx, or 5xx means GitHub
# answered; callers report a missing release as release_not_found. curl's exit
# status cannot make this distinction: an HTTP error exits with 22 or 56
# depending on the curl version and protocol.
dtwiz_check_github_reachable() {
    case "$1" in
        [245][0-9][0-9]) ;;
        *) dtwiz_fail github_unreachable \
            "Error: could not reach github.com." \
            "Check your network connection and proxy settings." ;;
    esac
}

dtwiz_send_event inv

# ── Parse known flags ──────────────────────────────────────────────────────────
INSTALL_DIR=""

while [ $# -gt 0 ]; do
    case "$1" in
        --install-dir)
            INSTALL_DIR="$2"; shift 2 ;;
        *)
            dtwiz_fail unknown_argument "Unknown argument: $1" ;;
    esac
done

# ── Detect OS ─────────────────────────────────────────────────────────────────
OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Darwin) OS="darwin" ;;
    Linux)  OS="linux"  ;;
    *)
        dtwiz_fail unsupported_os \
            "Unsupported OS: $OS_RAW" \
            "Use install.ps1 on Windows." ;;
esac

# ── Detect architecture ───────────────────────────────────────────────────────
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64)         ARCH="amd64" ;;
    arm64|aarch64)  ARCH="arm64" ;;
    *)
        dtwiz_fail unsupported_arch "Unsupported architecture: $ARCH_RAW" ;;
esac

echo "Detected platform: ${OS}/${ARCH}"

# ── Resolve release version ────────────────────────────────────────────────────
if ! command -v curl >/dev/null 2>&1; then
    dtwiz_fail curl_missing "Error: curl is required but not found."
fi

if [ -n "$TAG" ]; then
    # Use the provided release tag directly
    RELEASE_TAG="$TAG"
    if [ "$TAG" = "snapshot-main" ]; then
        echo "Installing latest snapshot from the main branch..."
    else
        echo "Installing preview snapshot for tag: ${TAG}"
    fi
    # The HTTP status is appended as the last line; -f keeps an error page out
    # of the body, so a missing release leaves only the status.
    RESPONSE="$(curl -fsSL -w '\n%{http_code}' \
        "https://github.com/${REPO}/releases/download/${RELEASE_TAG}/version.txt")"
    HTTP_CODE="$(printf '%s\n' "$RESPONSE" | tail -n 1)"
    VERSION="$(printf '%s\n' "$RESPONSE" | sed '$d')"
    if [ -z "$VERSION" ]; then
        dtwiz_check_github_reachable "$HTTP_CODE"
        if [ "$TAG" = "snapshot-main" ]; then
            dtwiz_fail release_not_found \
                "Error: could not find the main snapshot release." \
                "The 'snapshot-main' release may not have been published yet."
        else
            dtwiz_fail release_not_found \
                "Error: could not find a snapshot release for tag '${TAG}'." \
                "Make sure the tag exists and its release has been published."
        fi
    fi
else
    # Follow the /releases/latest redirect to extract the tag from the final URL.
    # curl prints the original URL when the request fails, so check the HTTP
    # status and that the result looks like a version tag.
    RESPONSE="$(curl -fsSL -o /dev/null -w '%{http_code} %{url_effective}' \
        "https://github.com/${REPO}/releases/latest")"
    dtwiz_check_github_reachable "${RESPONSE%% *}"
    RELEASE_TAG="${RESPONSE#* }"
    RELEASE_TAG="${RELEASE_TAG##*/}"
    case "$RELEASE_TAG" in
        v[0-9]*) ;;
        *) dtwiz_fail release_not_found "Error: could not determine the latest dtwiz release." ;;
    esac
    VERSION="$RELEASE_TAG"
fi

DTWIZ_SM_VERSION="${VERSION#v}"

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
if [ "$TAG" = "snapshot-main" ]; then
    echo "  - Channel:  main (pre-release snapshot)"
elif [ -n "$TAG" ]; then
    echo "  - Tag:      ${TAG} (pre-release)"
fi
echo "  - Download from github.com/${REPO}"
echo "  - Install to ${INSTALL_DIR}"
echo "  - Add ${INSTALL_DIR} to your PATH (if not already present)"
echo ""
printf 'Continue? [Y/n] '
read -r REPLY </dev/tty
case "$REPLY" in
    [Nn]|[Nn][Oo])
        echo "Installation cancelled."
        dtwiz_send_event can
        exit 0 ;;
esac

# ── Download and extract ───────────────────────────────────────────────────────
echo ""
echo "Downloading dtwiz ${VERSION}..."

ARCHIVE="dtwiz_${VERSION#v}_${OS}_${ARCH}.tar.gz"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT INT TERM

curl -fsSL \
    "https://github.com/${REPO}/releases/download/${RELEASE_TAG}/${ARCHIVE}" \
    -o "${WORK_DIR}/${ARCHIVE}" \
    || dtwiz_fail download_failed "Error: failed to download ${ARCHIVE}."

tar -xzf "${WORK_DIR}/${ARCHIVE}" -C "$WORK_DIR" \
    || dtwiz_fail extract_failed "Error: failed to extract ${ARCHIVE}."

if [ ! -f "${WORK_DIR}/dtwiz" ]; then
    dtwiz_fail binary_missing "Error: dtwiz binary not found after extraction."
fi

chmod +x "${WORK_DIR}/dtwiz"
mkdir -p "$INSTALL_DIR" \
    || dtwiz_fail move_failed "Error: could not create ${INSTALL_DIR}."

# ── Install binary ─────────────────────────────────────────────────────────────
DEST="${INSTALL_DIR}/dtwiz"
if [ -w "$INSTALL_DIR" ]; then
    mv "${WORK_DIR}/dtwiz" "$DEST" \
        || dtwiz_fail move_failed "Error: could not move dtwiz to ${DEST}."
else
    echo "Installing to ${INSTALL_DIR} requires elevated privileges..."
    sudo mv "${WORK_DIR}/dtwiz" "$DEST" \
        || dtwiz_fail move_failed "Error: could not move dtwiz to ${DEST}."
fi

echo ""
echo "dtwiz ${VERSION} installed to ${DEST}"
dtwiz_send_event com

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
