#!/usr/bin/env bash

set -Eeuo pipefail

usage() {
    cat <<'EOF'
Usage: ./install-debian.sh [--no-build] [--skip-deps] [--deps-only] [--prefix PATH]

Install prerequisites, build, and create a self-contained Adaptix installation
on Debian, Kali, Ubuntu, or another Debian-family distribution.

Options:
  --no-build      Install existing build artifacts without rebuilding.
  --skip-deps     Do not run apt-get; verify/use packages already installed.
  --deps-only     Install dependencies and exit without building.
  --prefix PATH   Choose another installation directory.
  -h, --help      Show this help.

Environment:
  CONTAINER_ENGINE=podman|docker   Choose a container engine (Podman default).
EOF
}

fail() {
    printf 'error: %s\n' "$*" >&2
    exit 1
}

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
build=true
install_deps=true
deps_only=false
installer_args=()

while (($#)); do
    case "$1" in
        --no-build)
            build=false
            installer_args+=("$1")
            shift
            ;;
        --skip-deps)
            install_deps=false
            shift
            ;;
        --deps-only)
            deps_only=true
            shift
            ;;
        --prefix)
            (($# >= 2)) || fail "--prefix requires a path"
            installer_args+=("$1" "$2")
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *) fail "unknown option: $1" ;;
    esac
done

if [[ "$install_deps" == true ]]; then
    dependency_mode=build
    [[ "$build" == true || "$deps_only" == true ]] || dependency_mode=runtime
    "$repo_root/scripts/setup-debian.sh" "$dependency_mode"
fi

if [[ "$deps_only" == true ]]; then
    exit 0
fi

if [[ "$build" == true ]]; then
    "$repo_root/scripts/build-debian.sh" all
    installer_args=(--no-build "${installer_args[@]}")
fi

# The installation layout, generated profile, Extension Kit handling, TeleC2
# configuration, and running-server guard are shared with the Fedora path.
exec "$repo_root/scripts/install-fedora.sh" "${installer_args[@]}"
