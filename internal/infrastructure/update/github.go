// Package update implements the application ReleaseSource against GitHub's latest-release
// endpoint. The endpoint only ever reports a published, non-draft, non-prerelease release, so a
// tag pushed mid-development is structurally invisible: the guard is the endpoint's own contract,
// not a check here. The HTTP client is injected so tests never touch the network; the production
// constructor uses a short-timeout client because the check must never block for long.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/oernster/pigeonpost/internal/application"
)

// LatestReleaseAPIURL is GitHub's latest-release endpoint for this repository.
const LatestReleaseAPIURL = "https://api.github.com/repos/oernster/pigeonpost/releases/latest"

// acceptHeader advertises a JSON client to the GitHub API.
const acceptHeader = "application/vnd.github+json"

// requestTimeout bounds the one network call the app's update check makes.
const requestTimeout = 5 * time.Second

// Doer executes one HTTP request; *http.Client satisfies it and tests inject a fake.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// releasePayload is the subset of GitHub's latest-release JSON the check reads.
type releasePayload struct {
	TagName string         `json:"tag_name"`
	HTMLURL string         `json:"html_url"`
	Assets  []assetPayload `json:"assets"`
}

type assetPayload struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// GitHubReleaseSource is an application.ReleaseSource backed by the GitHub API.
type GitHubReleaseSource struct {
	apiURL string
	client Doer
}

// NewGitHubReleaseSource constructs the production source with a short-timeout HTTP client.
func NewGitHubReleaseSource() *GitHubReleaseSource {
	return NewGitHubReleaseSourceWith(LatestReleaseAPIURL, &http.Client{Timeout: requestTimeout})
}

// NewGitHubReleaseSourceWith constructs a source against the given endpoint and client, for tests.
func NewGitHubReleaseSourceWith(apiURL string, client Doer) *GitHubReleaseSource {
	return &GitHubReleaseSource{apiURL: apiURL, client: client}
}

// LatestRelease returns the latest published release or an error when it cannot be read. The
// caller (UpdateService.Check) treats any error as "no update", keeping the check silent.
func (s *GitHubReleaseSource) LatestRelease(ctx context.Context) (application.ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return application.ReleaseInfo{}, fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", acceptHeader)
	resp, err := s.client.Do(req)
	if err != nil {
		return application.ReleaseInfo{}, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return application.ReleaseInfo{}, fmt.Errorf("fetch latest release: status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return application.ReleaseInfo{}, fmt.Errorf("read release body: %w", err)
	}
	var payload releasePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return application.ReleaseInfo{}, fmt.Errorf("parse release body: %w", err)
	}
	if payload.TagName == "" || payload.HTMLURL == "" {
		return application.ReleaseInfo{}, fmt.Errorf("release payload missing tag or page URL")
	}
	assets := make([]application.ReleaseAsset, 0, len(payload.Assets))
	for _, a := range payload.Assets {
		if a.Name != "" && a.DownloadURL != "" {
			assets = append(assets, application.ReleaseAsset{Name: a.Name, DownloadURL: a.DownloadURL})
		}
	}
	return application.ReleaseInfo{
		Version: payload.TagName,
		PageURL: payload.HTMLURL,
		Assets:  assets,
	}, nil
}
