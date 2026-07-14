#!/usr/bin/env bash
# Builds the PigeonPost macOS DMG for Apple Silicon. Run on an arm64 Mac from the repo root:
#
#   bash builddmg.sh
#
# Flow: generate icons, wails build for darwin/arm64, stamp the bundle version from VERSION,
# codesign the .app (hardened runtime), stage it with ditto, create-dmg, sign the DMG, then
# notarize and staple only when APPLE_ID and APPLE_APP_PASSWORD are set.
#
# Environment overrides:
#   DEVELOPER_ID_APPLICATION   signing identity (defaults to Oliver's Developer ID)
#   APPLE_ID, APPLE_APP_PASSWORD, APPLE_TEAM_ID   notarization credentials (skipped if unset)
#
# Output: PigeonPost.dmg in the repo root
set -euo pipefail

APP_NAME="PigeonPost"
PLATFORM="darwin/arm64"
APP_BUNDLE="build/bin/${APP_NAME}.app"
DIST_DIR="dist-dmg"
STAGE_DIR="${DIST_DIR}/stage"
VERSION="$(tr -d '[:space:]' < VERSION)"
# DIST_DIR/STAGE_DIR are scratch space; the final DMG lands in the repo root.
DMG_PATH="${APP_NAME}.dmg"

DEVELOPER_ID="${DEVELOPER_ID_APPLICATION:-Developer ID Application: Oliver Ernster (W7K465GKFJ)}"
APPLE_ID="${APPLE_ID:-}"
APPLE_APP_PASSWORD="${APPLE_APP_PASSWORD:-}"
APPLE_TEAM_ID="${APPLE_TEAM_ID:-W7K465GKFJ}"

section() { printf '\n\033[1m== %s ==\033[0m\n' "$1"; }

# require ensures a tool is on PATH, running the given install command if it is
# not. The install command is a full shell command (not just a brew formula) so
# go-installed tools like wails bootstrap the same way brew-installed ones do.
require() {
    local tool="$1" install="$2"
    if ! command -v "$tool" > /dev/null 2>&1; then
        section "Installing missing tool: $tool"
        eval "$install"
    fi
    command -v "$tool" > /dev/null 2>&1 || { echo "error: $tool is required (install: $install)" >&2; exit 1; }
}

section "Platform guard"
[ "$(uname -s)" = "Darwin" ] || { echo "error: this script must run on macOS" >&2; exit 1; }
[ "$(uname -m)" = "arm64" ] || { echo "error: this script targets Apple Silicon (arm64)" >&2; exit 1; }
command -v go > /dev/null 2>&1 || { echo "error: go is required (install from https://go.dev/dl/)" >&2; exit 1; }
# wails installs into the Go bin dir; put it on PATH so a fresh install resolves.
GOBIN_DIR="$(go env GOPATH)/bin"
case ":${PATH}:" in *":${GOBIN_DIR}:"*) ;; *) PATH="${GOBIN_DIR}:${PATH}" ;; esac
export PATH
require wails "go install github.com/wailsapp/wails/v2/cmd/wails@latest"
require npm "brew install node"
require create-dmg "brew install create-dmg"

section "Building ${APP_NAME} ${VERSION} (${PLATFORM})"
go run ./tools/genicons
wails build -clean -platform "${PLATFORM}"
[ -d "${APP_BUNDLE}" ] || { echo "error: ${APP_BUNDLE} was not produced" >&2; exit 1; }

section "Stamping bundle version from VERSION"
PLIST="${APP_BUNDLE}/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString ${VERSION}" "${PLIST}" \
    || /usr/libexec/PlistBuddy -c "Add :CFBundleShortVersionString string ${VERSION}" "${PLIST}"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion ${VERSION}" "${PLIST}" \
    || /usr/libexec/PlistBuddy -c "Add :CFBundleVersion string ${VERSION}" "${PLIST}"

section "Codesigning the app bundle"
codesign --force --deep --options runtime --sign "${DEVELOPER_ID}" "${APP_BUNDLE}"
codesign --verify --deep --strict "${APP_BUNDLE}"

section "Creating the DMG"
rm -rf "${DIST_DIR}"
rm -f "${DMG_PATH}"
mkdir -p "${STAGE_DIR}"
# ditto preserves the symlinks and metadata the embedded signature depends on.
ditto "${APP_BUNDLE}" "${STAGE_DIR}/${APP_NAME}.app"
VOLICON="${APP_BUNDLE}/Contents/Resources/iconfile.icns"
CREATE_DMG_ARGS=(
    --volname "${APP_NAME}"
    --window-size 540 380
    --icon-size 128
    --icon "${APP_NAME}.app" 140 190
    --app-drop-link 400 190
)
[ -f "${VOLICON}" ] && CREATE_DMG_ARGS+=(--volicon "${VOLICON}")
set +e
create-dmg "${CREATE_DMG_ARGS[@]}" "${DMG_PATH}" "${STAGE_DIR}"
STATUS=$?
set -e
# create-dmg exits 2 when it cannot set a custom window background (headless); still a good DMG.
if [ "${STATUS}" -ne 0 ] && [ "${STATUS}" -ne 2 ]; then
    echo "error: create-dmg failed with exit ${STATUS}" >&2
    exit "${STATUS}"
fi
rm -rf "${DIST_DIR}"

section "Signing the DMG"
codesign --force --sign "${DEVELOPER_ID}" "${DMG_PATH}"
codesign --verify "${DMG_PATH}"

if [ -n "${APPLE_ID}" ] && [ -n "${APPLE_APP_PASSWORD}" ]; then
    section "Notarizing (this waits on Apple)"
    xcrun notarytool submit "${DMG_PATH}" \
        --apple-id "${APPLE_ID}" \
        --password "${APPLE_APP_PASSWORD}" \
        --team-id "${APPLE_TEAM_ID}" \
        --wait
    xcrun stapler staple "${DMG_PATH}"
else
    section "Notarization skipped (set APPLE_ID and APPLE_APP_PASSWORD to enable)"
fi

section "Done"
echo "${DMG_PATH}"
