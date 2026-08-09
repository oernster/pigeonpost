// Package application: the update check. UpdateService asks the injected ReleaseSource for the
// latest published release and compares it against the running version. The one call the service
// makes to the network happens indirectly through the source and the service never propagates a
// source failure: an unreachable source yields a status reporting no update, keeping the check
// silent on failure. A release the user chose to skip is reported as seen but not available, so
// the same version never prompts twice.
package application

import (
	"context"
	"strconv"
	"strings"
)

// ReleaseAsset names one downloadable file attached to a release.
type ReleaseAsset struct {
	Name        string
	DownloadURL string
}

// ReleaseInfo is the latest published release as reported by the release source.
type ReleaseInfo struct {
	Version string
	PageURL string
	Assets  []ReleaseAsset
}

// ReleaseSource supplies the latest published release. The concrete implementation lives in
// infrastructure and queries the project's GitHub releases; only a published, non-draft,
// non-prerelease release is ever reported, so a tag pushed mid-development can never prompt.
type ReleaseSource interface {
	LatestRelease(ctx context.Context) (ReleaseInfo, error)
}

// UpdateStatus is the outcome of one update check. Latest is empty when the source could not be
// reached, in which case UpdateAvailable is always false.
type UpdateStatus struct {
	Current         string
	Latest          string
	UpdateAvailable bool
	DownloadURL     string
	PageURL         string
}

// Platform keys naming which release asset the running OS wants.
const (
	PlatformWindows = "windows"
	PlatformMacOS   = "macos"
	PlatformLinux   = "linux"
)

// Optional release-tag prefix stripped before parsing (for example "v1.14.0").
const versionTagPrefix = "v"

// Version component separator.
const versionSeparator = "."

// Asset filename suffixes per platform, in preference order.
var assetSuffixes = map[string][]string{
	PlatformWindows: {".exe"},
	PlatformMacOS:   {".dmg"},
	PlatformLinux:   {".flatpak"},
}

// goos values with a dedicated platform key; anything else is treated as Linux.
var goosPlatformKeys = map[string]string{
	"windows": PlatformWindows,
	"darwin":  PlatformMacOS,
}

// PlatformKeyFor maps a runtime.GOOS value to a platform key.
func PlatformKeyFor(goos string) string {
	if key, ok := goosPlatformKeys[goos]; ok {
		return key
	}
	return PlatformLinux
}

// SelectAssetURL returns the download URL of the first asset matching the platform or empty.
func SelectAssetURL(assets []ReleaseAsset, platformKey string) string {
	for _, suffix := range assetSuffixes[platformKey] {
		for _, asset := range assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), suffix) {
				return asset.DownloadURL
			}
		}
	}
	return ""
}

// versionParts parses a dotted version with an optional leading "v" into integer components,
// reporting ok false when any component is not an integer.
func versionParts(version string) ([]int, bool) {
	text := strings.TrimSpace(version)
	if strings.HasPrefix(strings.ToLower(text), versionTagPrefix) {
		text = text[len(versionTagPrefix):]
	}
	fields := strings.Split(text, versionSeparator)
	parts := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}

// IsNewerVersion reports whether latest is a strictly newer dotted version than current. A version
// that cannot be parsed as dotted integers is never newer, so a malformed tag cannot prompt.
func IsNewerVersion(latest, current string) bool {
	latestParts, ok := versionParts(latest)
	if !ok {
		return false
	}
	currentParts, ok := versionParts(current)
	if !ok {
		return false
	}
	for i := 0; i < len(latestParts) && i < len(currentParts); i++ {
		if latestParts[i] != currentParts[i] {
			return latestParts[i] > currentParts[i]
		}
	}
	return len(latestParts) > len(currentParts)
}

// UpdateService compares the running version against the latest published release.
type UpdateService struct {
	source         ReleaseSource
	currentVersion string
	platformKey    string
}

// NewUpdateService constructs the service with its release source, the running version and the
// platform key naming which release asset this OS downloads.
func NewUpdateService(source ReleaseSource, currentVersion, platformKey string) *UpdateService {
	return &UpdateService{source: source, currentVersion: currentVersion, platformKey: platformKey}
}

// Check returns the update status for the running version. An unreachable source yields an empty
// Latest and no update, keeping the check silent on failure. A newer release whose version equals
// skippedVersion is reported with UpdateAvailable false.
func (s *UpdateService) Check(ctx context.Context, skippedVersion string) UpdateStatus {
	info, err := s.source.LatestRelease(ctx)
	if err != nil {
		return UpdateStatus{Current: s.currentVersion}
	}
	newer := IsNewerVersion(info.Version, s.currentVersion)
	return UpdateStatus{
		Current:         s.currentVersion,
		Latest:          info.Version,
		UpdateAvailable: newer && info.Version != skippedVersion,
		DownloadURL:     SelectAssetURL(info.Assets, s.platformKey),
		PageURL:         info.PageURL,
	}
}
