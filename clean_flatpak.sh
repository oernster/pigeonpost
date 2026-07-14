#!/usr/bin/env bash
# Removes everything build_flatpak.sh created: the user install and the flatpak build
# artefacts. Scoped to flatpak only; never touches build/bin, dist-installer or dist-dmg,
# so the three build paths stay independent.
set -euo pipefail

APP_ID="uk.codecrafter.PigeonPost"   # must match build_flatpak.sh
BIN_NAME="pigeonpost"
BUNDLE="${BIN_NAME}.flatpak"
MANIFEST="${APP_ID}.yml"

if flatpak list --user 2> /dev/null | grep -q "${APP_ID}"; then
    flatpak uninstall --user -y "${APP_ID}"
else
    echo "Not installed, skipping uninstall."
fi

rm -f "${BUNDLE}" "${MANIFEST}"
rm -rf .flatpak-build .flatpak-repo .flatpak-builder packaging

echo "Flatpak artefacts removed."
