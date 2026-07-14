#!/usr/bin/env bash
# cleanup_flatpak.sh - Uninstall and purge the PigeonPost Flatpak
#
# Scoped to flatpak artefacts only.  It deliberately does NOT touch the
# outputs (build/, bin/, dist-installer, dist-dmg) produced by the other
# build paths, so the build paths stay independent.
set -euo pipefail

APP_ID="uk.codecrafter.PigeonPost"
BIN_NAME="pigeonpost"

bold=$(tput bold 2>/dev/null || true)
reset=$(tput sgr0 2>/dev/null || true)
section() { echo; echo "${bold}=== $* ===${reset}"; }

section "Uninstalling ${APP_ID}"
if flatpak list --user | grep -q "${APP_ID}"; then
    flatpak uninstall --user -y "${APP_ID}"
    echo "  Uninstalled."
else
    echo "  Not installed, skipping."
fi

section "Removing flatpak build artefacts"
rm -f "${BIN_NAME}.flatpak"
rm -rf .flatpak-build .flatpak-repo .flatpak-builder
rm -f "${APP_ID}.yml"
rm -rf packaging/
echo "  Done."

echo
echo "${bold}Purge complete.${reset}"
