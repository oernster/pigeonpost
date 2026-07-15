#!/usr/bin/env bash
# Builds the PigeonPost Flatpak for Linux (verified target: Ubuntu). Run from the repo root:
#
#   bash build_flatpak.sh
#
# Flow: install flatpak tooling if missing, add flathub, install the GNOME runtime and the
# golang and node SDK extensions, self-generate the desktop file, metainfo and manifest,
# build the app inside the sandbox (npm front end, then a CGO Go build against the runtime's
# webkit2gtk-4.1), install it for the current user and export a distributable bundle.
#
# The GNOME runtime is required because Wails v2 renders through webkit2gtk-4.1, which the
# freedesktop runtime does not carry. The Go build therefore uses -tags webkit2_41.
#
# Outputs: pigeonpost.flatpak (installable anywhere) and a user install of ${APP_ID}.
set -euo pipefail

APP_ID="uk.codecrafter.PigeonPost"
BIN_NAME="pigeonpost"
APP_NAME="PigeonPost"
APP_SUMMARY="Calm, local-first email, calendar and contacts"
HOMEPAGE="https://pigeonpost.ink"
RUNTIME="org.gnome.Platform"
SDK="org.gnome.Sdk"
RUNTIME_VERSION="48"
SDK_EXT_VERSION="24.08"   # the freedesktop base of GNOME 48; extensions pair with it
GOLANG_EXT="org.freedesktop.Sdk.Extension.golang"
NODE_EXT="org.freedesktop.Sdk.Extension.node22"
BUILD_DIR=".flatpak-build"
REPO_DIR=".flatpak-repo"
BUNDLE="${BIN_NAME}.flatpak"
MANIFEST="${APP_ID}.yml"
PACKAGING_DIR="packaging"
HICOLOR_SIZES="16 24 32 48 64 128 256 512"
VERSION="$(tr -d '[:space:]' < VERSION)"
RELEASE_DATE="$(date +%F)"

section() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

install_if_missing() {
    local tool="$1"
    command -v "$tool" > /dev/null 2>&1 && return 0
    section "Installing missing tool: $tool"
    if command -v apt-get > /dev/null 2>&1; then sudo apt-get install -y "$tool"
    elif command -v dnf > /dev/null 2>&1; then sudo dnf install -y "$tool"
    elif command -v pacman > /dev/null 2>&1; then sudo pacman -S --noconfirm "$tool"
    elif command -v zypper > /dev/null 2>&1; then sudo zypper install -y "$tool"
    else echo "error: install $tool with your package manager and re-run" >&2; exit 1
    fi
}

section "Tooling"
install_if_missing flatpak
install_if_missing flatpak-builder

section "Flathub remote and runtimes"
flatpak remote-add --if-not-exists --user flathub https://dl.flathub.org/repo/flathub.flatpakrepo
flatpak install --user --noninteractive flathub \
    "${RUNTIME}//${RUNTIME_VERSION}" \
    "${SDK}//${RUNTIME_VERSION}" \
    "${GOLANG_EXT}//${SDK_EXT_VERSION}" \
    "${NODE_EXT}//${SDK_EXT_VERSION}"

section "Generating Wails front-end bindings"
# frontend/wailsjs is generated (gitignored), so a fresh checkout does not carry it. The
# sandbox has no wails CLI, and the sandboxed `tsc` imports from ../wailsjs, so the bindings
# must exist on the host before flatpak-builder copies the tree. `wails generate module`
# compiles the Go app to introspect App methods, which triggers the `//go:embed all:frontend/dist`
# in main.go; seed a placeholder dist so that embed resolves (flatpak rebuilds the real dist).
if [ ! -d frontend/wailsjs ]; then
    WAILS_BIN="$(command -v wails || echo "${GOPATH:-$HOME/go}/bin/wails")"
    if [ ! -x "${WAILS_BIN}" ]; then
        echo "error: wails CLI not found; install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
        exit 1
    fi
    mkdir -p frontend/dist
    touch frontend/dist/.gitkeep
    "${WAILS_BIN}" generate module
fi

section "Writing packaging files"
mkdir -p "${PACKAGING_DIR}"

cat > "${PACKAGING_DIR}/${APP_ID}.desktop" << DESKTOP
[Desktop Entry]
Name=${APP_NAME}
Comment=${APP_SUMMARY}
Exec=${BIN_NAME}
Icon=${APP_ID}
Terminal=false
Type=Application
Categories=Network;Email;Office;
DESKTOP

cat > "${PACKAGING_DIR}/${APP_ID}.metainfo.xml" << METAINFO
<?xml version="1.0" encoding="UTF-8"?>
<component type="desktop-application">
  <id>${APP_ID}</id>
  <name>${APP_NAME}</name>
  <summary>${APP_SUMMARY}</summary>
  <metadata_license>CC0-1.0</metadata_license>
  <project_license>GPL-3.0-only</project_license>
  <description>
    <p>
      A calm, local-first desktop client for email, calendar and contacts. Mail stays with
      your provider over IMAP or POP3, cached locally for instant offline search; passwords
      live in the OS keychain and no cloud sits in between.
    </p>
  </description>
  <launchable type="desktop-id">${APP_ID}.desktop</launchable>
  <url type="homepage">${HOMEPAGE}</url>
  <content_rating type="oars-1.1"/>
  <releases>
    <release version="${VERSION}" date="${RELEASE_DATE}"/>
  </releases>
</component>
METAINFO

section "Writing manifest"
ICON_INSTALL_CMDS=""
for size in ${HICOLOR_SIZES}; do
    ICON_INSTALL_CMDS="${ICON_INSTALL_CMDS}      - install -Dm644 build/linux/icons/${BIN_NAME}_${size}.png /app/share/icons/hicolor/${size}x${size}/apps/${APP_ID}.png
"
done

cat > "${MANIFEST}" << MANIFEST_EOF
app-id: ${APP_ID}
runtime: ${RUNTIME}
runtime-version: '${RUNTIME_VERSION}'
sdk: ${SDK}
sdk-extensions:
  - ${GOLANG_EXT}
  - ${NODE_EXT}
command: ${BIN_NAME}
finish-args:
  - --share=ipc
  - --socket=fallback-x11
  - --socket=wayland
  - --device=dri
  - --share=network
  - --filesystem=home
  - --talk-name=org.freedesktop.secrets
  - --talk-name=org.freedesktop.Notifications
  - --own-name=uk.codecrafter.pigeonpost
build-options:
  append-path: /usr/lib/sdk/golang/bin:/usr/lib/sdk/node22/bin
  build-args:
    - --share=network
  env:
    GOPATH: /run/build/${BIN_NAME}/gopath
    GOCACHE: /run/build/${BIN_NAME}/gocache
    GOFLAGS: -buildvcs=false
    npm_config_cache: /run/build/${BIN_NAME}/npm-cache
modules:
  - name: ${BIN_NAME}
    buildsystem: simple
    build-commands:
      - cd frontend && npm ci --no-audit --no-fund && npm run build
      - go run ./tools/genicons
      - go build -tags desktop,production,webkit2_41 -ldflags "-s -w" -o ${BIN_NAME} .
      - install -Dm755 ${BIN_NAME} /app/bin/${BIN_NAME}
      - chmod -R u+w gopath gocache 2>/dev/null || true
      - install -Dm644 packaging/${APP_ID}.desktop /app/share/applications/${APP_ID}.desktop
      - install -Dm644 packaging/${APP_ID}.metainfo.xml /app/share/metainfo/${APP_ID}.metainfo.xml
${ICON_INSTALL_CMDS}    sources:
      - type: dir
        path: .
MANIFEST_EOF

section "Building ${APP_NAME} ${VERSION} with flatpak-builder"
flatpak-builder --user --install --force-clean \
    --install-deps-from=flathub \
    --repo="${REPO_DIR}" \
    "${BUILD_DIR}" "${MANIFEST}"

section "Exporting bundle"
flatpak build-bundle \
    --runtime-repo=https://dl.flathub.org/repo/flathub.flatpakrepo \
    "${REPO_DIR}" "${BUNDLE}" "${APP_ID}"

section "Done"
echo "Installed for the current user: flatpak run ${APP_ID}"
echo "Distributable bundle: ${BUNDLE}"
