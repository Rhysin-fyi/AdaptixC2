#!/usr/bin/env bash

set -Eeuo pipefail

usage() {
    cat <<'EOF'
Usage: scripts/setup-debian.sh [runtime|build]

Install Adaptix runtime dependencies on Debian, Kali, Ubuntu, or another
Debian-family distribution. The build mode also installs a container engine
when neither Podman nor Docker is available.

Environment:
  CONTAINER_ENGINE=podman|docker   Select the build engine.
  ADAPTIX_ALLOW_UNSUPPORTED_DISTRO=1
                                    Skip distribution detection (CI/testing).
EOF
}

fail() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

mode="${1:-build}"
case "$mode" in
    runtime|build) ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; exit 2 ;;
esac

[[ "$(uname -s)" == Linux ]] || fail "this setup script requires Linux"
[[ "$(uname -m)" == x86_64 ]] || fail "the packaged Adaptix client currently requires x86_64"

if [[ "${ADAPTIX_ALLOW_UNSUPPORTED_DISTRO:-0}" != 1 ]]; then
    [[ -r /etc/os-release ]] || fail "cannot identify this Linux distribution"
    # shellcheck disable=SC1091
    source /etc/os-release
    distro_words=" ${ID:-} ${ID_LIKE:-} "
    if [[ "$distro_words" != *" debian "* && "$distro_words" != *" kali "* && "$distro_words" != *" ubuntu "* ]]; then
        fail "this script supports Debian-family distributions; detected ${PRETTY_NAME:-unknown}"
    fi
fi

command -v apt-get >/dev/null 2>&1 || fail "apt-get is required"
command -v apt-cache >/dev/null 2>&1 || fail "apt-cache is required"

if ((EUID == 0)); then
    elevate=()
elif command -v sudo >/dev/null 2>&1; then
    elevate=(sudo)
else
    fail "run as root or install sudo so packages can be installed"
fi

printf '[*] Refreshing Debian package metadata...\n'
"${elevate[@]}" apt-get update

packages=(
    ca-certificates
    openssl
    libgl1
    libdbus-1-3
    libfontconfig1
    libfreetype6
    libxkbcommon-x11-0
    libxcb-cursor0
    libxcb-icccm4
    libxcb-image0
    libxcb-keysyms1
    libxcb-randr0
    libxcb-render-util0
    libxcb-shape0
    libxcb-xfixes0
    libxcb-xinerama0
    libxcb-xkb1
)

package_has_candidate() {
    local candidate
    candidate="$(apt-cache policy "$1" | awk '/Candidate:/ {print $2; exit}')"
    [[ -n "$candidate" && "$candidate" != "(none)" ]]
}

if package_has_candidate libfuse2; then
    packages+=(libfuse2)
elif package_has_candidate libfuse2t64; then
    packages+=(libfuse2t64)
else
    printf '[*] FUSE 2 is unavailable; the launcher will use AppImage extract-and-run mode.\n'
fi

if [[ "$mode" == build ]]; then
    engine="${CONTAINER_ENGINE:-}"
    if [[ -z "$engine" ]]; then
        if command -v podman >/dev/null 2>&1; then
            engine=podman
        elif command -v docker >/dev/null 2>&1; then
            engine=docker
        else
            engine=podman
        fi
    fi
    case "$engine" in
        podman)
            if ! command -v podman >/dev/null 2>&1; then
                packages+=(podman uidmap slirp4netns fuse-overlayfs)
            fi
            ;;
        docker)
            if ! command -v docker >/dev/null 2>&1; then
                packages+=(docker.io)
            fi
            ;;
        *) fail "CONTAINER_ENGINE must be podman or docker" ;;
    esac
fi

printf '[*] Installing Adaptix %s dependencies...\n' "$mode"
"${elevate[@]}" env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${packages[@]}"

printf '[+] Debian/Kali dependencies are installed.\n'
if [[ "$mode" == build ]]; then
    printf '[+] Container engine: %s\n' "$engine"
fi
