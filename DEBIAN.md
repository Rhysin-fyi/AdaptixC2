# Building AdaptixC2 on Debian and Kali

Debian, Kali, Ubuntu, and compatible Debian-family distributions have a
dedicated containerized build and local installer. The build keeps the required
Go, MinGW, CMake, and Qt versions inside the repository's container stages.

The packaged GUI client currently requires an x86-64 host.

## Easy install

From the repository root:

```bash
./install-debian.sh
```

The installer uses `sudo apt-get` when necessary and installs the runtime
libraries, OpenSSL, and Podman when no container engine is already available.
Debian receives FUSE 2 AppImage support when its release provides it. Current
Kali rolling no longer packages FUSE 2, so the launcher automatically uses the
AppImage's extract-and-run mode there. Kali and Debian both provide Podman
through their normal APT repositories. Set `CONTAINER_ENGINE=docker` to
install/use `docker.io` instead.

Useful modes:

```bash
# Only install Debian/Kali dependencies
./install-debian.sh --deps-only

# Reuse existing build artifacts
./install-debian.sh --no-build

# Do not invoke apt (for an already prepared/offline host)
./install-debian.sh --skip-deps

# Install somewhere else
./install-debian.sh --prefix /opt/adaptix-local
```

The local installation behavior matches Fedora: existing profile credentials
are preserved, a first-run local password and TLS certificate are generated,
the built Extension Kit is copied when present, and the private TeleC2
configuration is retained.

The build now stops immediately if any extender fails or omits its runtime
artifacts. As an additional safeguard, the installer enables TeleC2 only when
its config, shared plugin, client script, and private runtime configuration are
all present. Rerunning the installer repairs an older profile that references
an incomplete TeleC2 bundle.

Display the generated connection details, then start both components:

```bash
cat install/connection.txt
./install/run-adaptix.sh
```

## Individual build targets

```bash
make debian-client
make debian-server
make debian-extenders
make debian-server-ext
make debian-all
```

The build wrapper can also be called directly:

```bash
scripts/build-debian.sh client
scripts/build-debian.sh server
scripts/build-debian.sh extenders
scripts/build-debian.sh server-ext
scripts/build-debian.sh all
```

Artifacts are written to:

- `AdaptixClient/client-dist/AdaptixClient-x86_64.AppImage`
- `AdaptixServer/server-dist/`

If the host cannot mount AppImages with FUSE, extract the client instead:

```bash
AdaptixClient/client-dist/AdaptixClient-x86_64.AppImage --appimage-extract
QT_QPA_PLATFORM=xcb ./squashfs-root/AppRun
```
