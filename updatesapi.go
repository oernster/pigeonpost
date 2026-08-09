package main

// UpdateStatusDTO is the JSON-serialisable outcome of one update check. Latest is empty when
// GitHub could not be reached, in which case UpdateAvailable is always false. DownloadURL is the
// release asset for this OS (empty when the release carries none) and PageURL is the release page
// the front end falls back to.
type UpdateStatusDTO struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"updateAvailable"`
	DownloadURL     string `json:"downloadUrl"`
	PageURL         string `json:"pageUrl"`
}

// CheckForUpdates runs one update check against GitHub's latest-release endpoint (published
// releases only, so a pushed tag with no formal release can never prompt). skippedVersion is the
// release the user chose to skip, passed as empty for a manual check so an explicit request still
// reports a skipped release. Wails runs bound calls off the main thread, so the short network wait
// never blocks the interface; failure is reported as no update, never as an error.
func (a *App) CheckForUpdates(skippedVersion string) UpdateStatusDTO {
	status := a.updates.Check(a.ctx, skippedVersion)
	return UpdateStatusDTO{
		Current:         status.Current,
		Latest:          status.Latest,
		UpdateAvailable: status.UpdateAvailable,
		DownloadURL:     status.DownloadURL,
		PageURL:         status.PageURL,
	}
}
