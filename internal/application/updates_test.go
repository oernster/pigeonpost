package application

import (
	"context"
	"errors"
	"testing"
)

type fakeReleaseSource struct {
	info ReleaseInfo
	err  error
}

func (f *fakeReleaseSource) LatestRelease(_ context.Context) (ReleaseInfo, error) {
	return f.info, f.err
}

const testCurrent = "1.13.2"
const testNewer = "1.14.0"
const testPageURL = "https://github.com/oernster/pigeonpost/releases/tag/v1.14.0"

func testAssets() []ReleaseAsset {
	return []ReleaseAsset{
		{Name: "PigeonPostSetup.exe", DownloadURL: "https://dl/setup.exe"},
		{Name: "PigeonPost.dmg", DownloadURL: "https://dl/app.dmg"},
		{Name: "pigeonpost.flatpak", DownloadURL: "https://dl/app.flatpak"},
	}
}

func testRelease() ReleaseInfo {
	return ReleaseInfo{Version: testNewer, PageURL: testPageURL, Assets: testAssets()}
}

func testService(info ReleaseInfo, err error, platformKey string) *UpdateService {
	return NewUpdateService(&fakeReleaseSource{info: info, err: err}, testCurrent, platformKey)
}

func TestAnUnreachableSourceReportsNoUpdate(t *testing.T) {
	t.Parallel()
	status := testService(ReleaseInfo{}, errors.New("no network"), PlatformWindows).
		Check(context.Background(), "")
	if status.Current != testCurrent || status.Latest != "" || status.UpdateAvailable {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.DownloadURL != "" || status.PageURL != "" {
		t.Fatalf("unreachable source must yield no URLs: %+v", status)
	}
}

func TestANewerReleaseIsAvailableWithItsDownloadAndPage(t *testing.T) {
	t.Parallel()
	status := testService(testRelease(), nil, PlatformWindows).Check(context.Background(), "")
	if !status.UpdateAvailable || status.Latest != testNewer {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.DownloadURL != "https://dl/setup.exe" || status.PageURL != testPageURL {
		t.Fatalf("unexpected URLs: %+v", status)
	}
}

func TestTheRunningVersionIsNotAnUpdate(t *testing.T) {
	t.Parallel()
	info := testRelease()
	info.Version = testCurrent
	status := testService(info, nil, PlatformWindows).Check(context.Background(), "")
	if status.UpdateAvailable || status.Latest != testCurrent {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestASkippedReleaseIsSeenButNotAvailable(t *testing.T) {
	t.Parallel()
	status := testService(testRelease(), nil, PlatformWindows).Check(context.Background(), testNewer)
	if status.UpdateAvailable || status.Latest != testNewer {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestASkipOfSomeOtherReleaseDoesNotSuppressThePrompt(t *testing.T) {
	t.Parallel()
	status := testService(testRelease(), nil, PlatformWindows).Check(context.Background(), "1.13.9")
	if !status.UpdateAvailable {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestEachPlatformPicksItsOwnAsset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		platformKey string
		want        string
	}{
		{PlatformWindows, "https://dl/setup.exe"},
		{PlatformMacOS, "https://dl/app.dmg"},
		{PlatformLinux, "https://dl/app.flatpak"},
	}
	for _, c := range cases {
		if got := SelectAssetURL(testAssets(), c.platformKey); got != c.want {
			t.Errorf("platform %s: got %q, want %q", c.platformKey, got, c.want)
		}
	}
}

func TestAssetMatchingIgnoresCase(t *testing.T) {
	t.Parallel()
	assets := []ReleaseAsset{{Name: "SETUP.EXE", DownloadURL: "https://dl/x"}}
	if got := SelectAssetURL(assets, PlatformWindows); got != "https://dl/x" {
		t.Fatalf("got %q", got)
	}
}

func TestNoMatchingAssetYieldsAnEmptyDownloadURL(t *testing.T) {
	t.Parallel()
	if got := SelectAssetURL(nil, PlatformWindows); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := SelectAssetURL(testAssets(), "beos"); got != "" {
		t.Fatalf("unknown platform key: got %q", got)
	}
}

func TestGoosMapsToAPlatformKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"windows": PlatformWindows,
		"darwin":  PlatformMacOS,
		"linux":   PlatformLinux,
		"freebsd": PlatformLinux,
	}
	for goos, want := range cases {
		if got := PlatformKeyFor(goos); got != want {
			t.Errorf("goos %s: got %q, want %q", goos, got, want)
		}
	}
}

func TestIsNewerVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.14.0", "1.13.2", true},
		{"2.0.0", "1.99.99", true},
		{"v1.14.0", "1.13.2", true},
		{"V1.14.0", "1.13.2", true},
		{" 1.14.0 ", "1.13.2", true},
		{"1.13.2.1", "1.13.2", true},
		{"1.13.2", "1.13.2", false},
		{"v1.13.2", "1.13.2", false},
		{"1.13.1", "1.13.2", false},
		{"1.9.9", "1.13.2", false},
		{"not-a-version", "1.13.2", false},
		{"1.14.0", "not-a-version", false},
		{"", "1.13.2", false},
		{"1.14.0-beta", "1.13.2", false},
	}
	for _, c := range cases {
		if got := IsNewerVersion(c.latest, c.current); got != c.want {
			t.Errorf("IsNewerVersion(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
