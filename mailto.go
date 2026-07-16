package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// mailtoPrefix identifies a mailto: URI among launch arguments, handed to PigeonPost when it is the
// registered mailto handler and the user clicks an email link anywhere in Windows.
const mailtoPrefix = "mailto:"

// mailtoFields is the parsed content of an RFC 6068 mailto URI, handed to the front end to pre-fill a
// compose window.
type mailtoFields struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

// firstMailtoArg returns the first mailto: URI among launch arguments or "" when there is none. Like
// firstEmailFileArg it scans every argument; argv[0] is an executable path so it never matches.
func firstMailtoArg(args []string) string {
	for _, arg := range args {
		if len(arg) > len(mailtoPrefix) && strings.EqualFold(arg[:len(mailtoPrefix)], mailtoPrefix) {
			return arg
		}
	}
	return ""
}

// parseMailto parses an RFC 6068 mailto URI: addresses in the path part, then to/cc/bcc/subject/body in
// the query. Addresses are comma-separated wherever they appear and repeated query keys accumulate.
func parseMailto(uri string) (mailtoFields, error) {
	if len(uri) < len(mailtoPrefix) || !strings.EqualFold(uri[:len(mailtoPrefix)], mailtoPrefix) {
		return mailtoFields{}, fmt.Errorf("not a mailto URI: %q", uri)
	}
	pathPart, queryPart, _ := strings.Cut(uri[len(mailtoPrefix):], "?")

	fields := mailtoFields{}
	toPath, err := url.PathUnescape(pathPart)
	if err != nil {
		return mailtoFields{}, fmt.Errorf("malformed mailto address part: %w", err)
	}
	fields.To = appendAddresses(fields.To, toPath)

	if queryPart != "" {
		// RFC 6068 encodes a space as %20 and a literal plus stays a plus, while ParseQuery decodes +
		// as a space; escape the plus first so an address like user+tag@example.org survives.
		values, err := url.ParseQuery(strings.ReplaceAll(queryPart, "+", "%2B"))
		if err != nil {
			return mailtoFields{}, fmt.Errorf("malformed mailto query: %w", err)
		}
		for _, to := range values["to"] {
			fields.To = appendAddresses(fields.To, to)
		}
		for _, cc := range values["cc"] {
			fields.Cc = appendAddresses(fields.Cc, cc)
		}
		for _, bcc := range values["bcc"] {
			fields.Bcc = appendAddresses(fields.Bcc, bcc)
		}
		fields.Subject = values.Get("subject")
		fields.Body = values.Get("body")
	}
	return fields, nil
}

// appendAddresses splits a comma-separated address list, trimming whitespace and dropping empties.
func appendAddresses(dst []string, raw string) []string {
	for _, part := range strings.Split(raw, ",") {
		if address := strings.TrimSpace(part); address != "" {
			dst = append(dst, address)
		}
	}
	return dst
}

// openMailto parses a mailto: URI and asks the front end to open a pre-filled compose window. It backs a
// clicked email link once PigeonPost is the default mail client, whether the app was already running or
// was cold-started for it. A parse failure is reported to the front end so the user is told.
func (a *App) openMailto(uri string) {
	fields, err := parseMailto(uri)
	if err != nil {
		runtime.EventsEmit(a.ctx, "mailto:open-error", err.Error())
		return
	}
	runtime.EventsEmit(a.ctx, "mailto:open", fields)
}

// ShowDefaultMailAppSettings opens the Windows Default apps settings so the user can make PigeonPost the
// default email client (the MAILTO link type). It first re-writes the protocol registration, so the entry
// is present even if the app was never installed through the setup program. Windows does not let an
// application set itself as the default silently, so the settings page is the supported route; offered
// from the Mail menu on Windows. Bound to the front end.
func (a *App) ShowDefaultMailAppSettings() {
	if err := registerMailtoHandler(); err != nil {
		runtime.LogWarningf(a.ctx, "mailto registration: %v", err)
	}
	runtime.BrowserOpenURL(a.ctx, defaultAppsSettingsURL)
}
