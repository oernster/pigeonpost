package update

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
)

type fakeDoer struct {
	seen   *http.Request
	status int
	body   string
	err    error
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.seen = req
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

const happyBody = `{
	"tag_name": "v1.14.0",
	"html_url": "https://github.com/oernster/pigeonpost/releases/tag/v1.14.0",
	"assets": [
		{"name": "PigeonPostSetup.exe", "browser_download_url": "https://dl/setup.exe"},
		{"name": "pigeonpost.flatpak", "browser_download_url": "https://dl/app.flatpak"},
		{"name": "no-url.dmg", "browser_download_url": ""},
		{"name": "", "browser_download_url": "https://dl/nameless"}
	]
}`

func sourceFor(doer *fakeDoer) *GitHubReleaseSource {
	return NewGitHubReleaseSourceWith(LatestReleaseAPIURL, doer)
}

func TestAPublishedReleaseIsReadWithItsWellFormedAssets(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{status: http.StatusOK, body: happyBody}
	info, err := sourceFor(doer).LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Version != "v1.14.0" {
		t.Fatalf("version: %q", info.Version)
	}
	if info.PageURL != "https://github.com/oernster/pigeonpost/releases/tag/v1.14.0" {
		t.Fatalf("page url: %q", info.PageURL)
	}
	want := []application.ReleaseAsset{
		{Name: "PigeonPostSetup.exe", DownloadURL: "https://dl/setup.exe"},
		{Name: "pigeonpost.flatpak", DownloadURL: "https://dl/app.flatpak"},
	}
	if len(info.Assets) != len(want) {
		t.Fatalf("assets: %+v", info.Assets)
	}
	for i, a := range want {
		if info.Assets[i] != a {
			t.Errorf("asset %d: got %+v, want %+v", i, info.Assets[i], a)
		}
	}
}

func TestTheRequestTargetsTheLatestReleaseEndpointAsJSON(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{status: http.StatusOK, body: happyBody}
	if _, err := sourceFor(doer).LatestRelease(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doer.seen.URL.String() != LatestReleaseAPIURL {
		t.Fatalf("url: %s", doer.seen.URL)
	}
	if doer.seen.Header.Get("Accept") != "application/vnd.github+json" {
		t.Fatalf("accept: %q", doer.seen.Header.Get("Accept"))
	}
	if doer.seen.Method != http.MethodGet {
		t.Fatalf("method: %s", doer.seen.Method)
	}
}

func TestAFailingClientYieldsAnError(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{err: errors.New("no network")}
	if _, err := sourceFor(doer).LatestRelease(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestANonOKStatusYieldsAnError(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{status: http.StatusNotFound, body: "{}"}
	if _, err := sourceFor(doer).LatestRelease(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAnUnparseableBodyYieldsAnError(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{status: http.StatusOK, body: "not json"}
	if _, err := sourceFor(doer).LatestRelease(context.Background()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAPayloadMissingItsIdentityYieldsAnError(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"html_url": "https://x"}`,
		`{"tag_name": "v1.14.0"}`,
		`{"tag_name": "", "html_url": ""}`,
	}
	for _, body := range cases {
		doer := &fakeDoer{status: http.StatusOK, body: body}
		if _, err := sourceFor(doer).LatestRelease(context.Background()); err == nil {
			t.Errorf("expected an error for body %s", body)
		}
	}
}

func TestAMalformedAssetsFieldReadsAsNoAssets(t *testing.T) {
	t.Parallel()
	doer := &fakeDoer{status: http.StatusOK, body: `{"tag_name": "v1.14.0", "html_url": "https://x"}`}
	info, err := sourceFor(doer).LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.Assets) != 0 {
		t.Fatalf("assets: %+v", info.Assets)
	}
}
