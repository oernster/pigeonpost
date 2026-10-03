package structural

// PigeonPost's outbound list is a promise made in its documents: beyond your own mail and calendar
// servers it reaches out in three ways only (the GitHub releases check, a message's remote images when
// you load them, Microsoft sign-in). Until this file, nothing held that list: a new outbound call
// compiled, passed and shipped. These tests pin it to the source. Only the packages named below may
// import anything that opens a connection; the front end may make no request of its own.
//
// What this cannot see: a request Wails or its web view makes on its own account, a connection opened
// by a library through a package not listed here; an <img> the front end is handed. It is held over
// this repository's source, not over the binary.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// networkReachers are the directories allowed to open a connection, each with the entry on the
// documented outbound list it serves. A new one is a new way out of the machine: it belongs in the
// documents first, then here.
var networkReachers = map[string]string{
	"internal/infrastructure/imap":        "your own mail server (IMAP)",
	"internal/infrastructure/pop3":        "your own mail server (POP3)",
	"internal/infrastructure/smtp":        "your own mail server (SMTP)",
	"internal/infrastructure/caldav":      "your own calendar server (CalDAV)",
	"internal/infrastructure/oauth":       "Microsoft sign-in",
	"internal/infrastructure/remoteimage": "a message's remote images, when you load them",
	"internal/infrastructure/update":      "the GitHub releases check",
}

// networkBuilders are single files that import a network package to build a client they hand on and
// never send anything through themselves.
var networkBuilders = map[string]string{
	"main.go": "the composition root builds the sign-in package's HTTP client",
}

// networkRoots are the packages a connection is opened through; a path beneath one is one of them.
var networkRoots = []string{
	"net",
	"crypto/tls",
	"golang.org/x/net",
	"github.com/emersion/go-imap/v2/imapclient",
	"github.com/emersion/go-smtp",
	"github.com/emersion/go-webdav",
}

// parsingOnly are packages under a network root that only read or write a format and cannot open a
// connection: an address, a mail header, a line protocol over a connection opened elsewhere, HTML.
var parsingOnly = []string{
	"net/url",
	"net/mail",
	"net/textproto",
	"net/netip",
	"golang.org/x/net/html",
}

func underPath(imported, root string) bool {
	return imported == root || strings.HasPrefix(imported, root+"/")
}

func isNetworkPackage(imported string) bool {
	for _, parsing := range parsingOnly {
		if underPath(imported, parsing) {
			return false
		}
	}
	for _, root := range networkRoots {
		if underPath(imported, root) {
			return true
		}
	}
	return false
}

func mayReachNetwork(f goFile) bool {
	for dir := range networkReachers {
		if inPackage(f, dir) {
			return true
		}
	}
	_, builder := networkBuilders[f.name]
	return builder && f.relDir == "."
}

func TestOnlyTheListedPackagesReachTheNetwork(t *testing.T) {
	for _, f := range scanRepo(t) {
		if mayReachNetwork(f) {
			continue
		}
		for _, imported := range f.imports {
			if isNetworkPackage(imported) {
				t.Errorf("%s/%s imports %s: only the packages in networkReachers may open a connection",
					f.relDir, f.name, imported)
			}
		}
	}
}

// An exemption must name something that exists, so a moved package cannot leave the rule pointing at
// nothing while its new home goes unchecked.
func TestEveryNetworkExemptionExists(t *testing.T) {
	root := repoRoot(t)
	for dir := range networkReachers {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir)))
		if err != nil || !info.IsDir() {
			t.Errorf("%s is allowed to reach the network but is not a directory: %v", dir, err)
		}
	}
	for name := range networkBuilders {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("%s is allowed to build a network client but does not exist: %v", name, err)
		}
	}
}

func TestNetworkPackageRecognitionIsExact(t *testing.T) {
	for _, network := range []string{
		"net", "net/http", "net/http/httputil", "net/smtp", "crypto/tls", "golang.org/x/net/proxy",
		"github.com/emersion/go-imap/v2/imapclient", "github.com/emersion/go-smtp",
		"github.com/emersion/go-webdav/caldav",
	} {
		if !isNetworkPackage(network) {
			t.Errorf("%s was not recognised as a network package", network)
		}
	}
	for _, ordinary := range []string{
		"net/url", "net/mail", "net/textproto", "net/netip", "golang.org/x/net/html",
		"golang.org/x/net/html/atom", "network", "crypto/sha256", "github.com/emersion/go-imap/v2",
		"github.com/emersion/go-message/mail", "github.com/emersion/go-smtpd",
	} {
		if isNetworkPackage(ordinary) {
			t.Errorf("%s was taken for a network package", ordinary)
		}
	}
}

// frontendRequest matches every way a page script can ask the network for something itself.
var frontendRequest = regexp.MustCompile(`\bfetch\s*\(|XMLHttpRequest|\bWebSocket\b|\bEventSource\b|\bsendBeacon\b`)

// commentLine is a line holding only a comment, where "fetch (" is an English word rather than a call.
var commentLine = regexp.MustCompile(`^\s*(//|/\*|\*)`)

func isRequest(line string) bool {
	return !commentLine.MatchString(line) && frontendRequest.MatchString(line)
}

// The front end reaches the Go side through Wails bindings and nothing else: every request leaves
// through the packages above, where this file can see it.
func TestTheFrontEndMakesNoRequestOfItsOwn(t *testing.T) {
	src := filepath.Join(repoRoot(t), "frontend", "src")
	scanned := 0
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() || !(strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx")) ||
			strings.Contains(name, ".test.") {
			return nil
		}
		content, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		scanned++
		for n, line := range strings.Split(string(content), "\n") {
			if isRequest(line) {
				rel, _ := filepath.Rel(src, path)
				t.Errorf("frontend/src/%s:%d makes a request of its own: %s",
					filepath.ToSlash(rel), n+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan front end: %v", err)
	}
	if scanned == 0 {
		t.Fatal("no front-end source scanned; check the path")
	}
}

func TestFrontEndRequestRecognition(t *testing.T) {
	for _, request := range []string{
		"fetch(url)", "await fetch (u)", "new XMLHttpRequest()", "new WebSocket(u)",
		"new EventSource(u)", "navigator.sendBeacon(u, b)",
	} {
		if !isRequest(request) {
			t.Errorf("%q was not recognised as a request", request)
		}
	}
	for _, ordinary := range []string{
		"refetch(x)", "prefetchRows()", "fetchedAt", "WebSockets",
		"  // a failed fetch (offline, say) survives", " * during the fetch (the user switched)",
	} {
		if isRequest(ordinary) {
			t.Errorf("%q was taken for a request", ordinary)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}
