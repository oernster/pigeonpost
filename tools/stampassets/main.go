// Command stampassets versions the landing page's stylesheet and script links by what the files hold.
//
// GitHub Pages lets a browser cache a file for ten minutes, so a freshly deployed page can arrive with
// the stylesheet it replaced and render against rules that are no longer there. Each local .css or .js
// link in docs/ therefore carries ?v=<hash> of the file it points at: any change to the file is a new
// address; an unchanged file keeps its address and its cache.
//
// The hash is of the content with CRLF read as LF, so a Windows checkout and the LF blob GitHub serves
// agree and a run on another machine does not rewrite every page. Only links relative to the page are
// touched: one that names a scheme, starts // or is root-absolute is somebody else's file or a path the
// page cannot resolve, so it is left exactly as written. Any query already on a local link is replaced.
//
// It is idempotent: a page whose links already carry the current hashes is left alone rather than
// rewritten; each page's bytes, line endings included, are kept apart from the links themselves. Every
// hash is worked out before any page is written, so a link to a file that does not exist stops the run
// with that path named and nothing written.
//
// Run from the repo root: go run ./tools/stampassets
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// siteDir is the landing page GitHub Pages serves; pageExt picks the pages inside it to stamp.
const (
	siteDir = "docs"
	pageExt = ".html"
)

// hashLength is how many hex characters of the SHA-256 a link carries: ten is 40 bits, far past any
// chance of two versions of one file colliding; it is short enough to keep a link readable.
const hashLength = 10

// queryKey is the name of the query parameter that carries the hash.
const queryKey = "v"

// assetLink matches a double-quoted href or src naming a .css or .js file, with any query and fragment
// it already carries. The groups are the attribute, the path and the fragment (empty when absent).
var assetLink = regexp.MustCompile(`\b(href|src)="([^"?#]+\.(?:css|js))(?:\?[^"#]*)?(#[^"]*)?"`)

// scheme matches a link that opens with a URL scheme such as https: or data:.
var scheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// stamped is one page whose new bytes differ from what is on disk.
type stamped struct {
	path string
	data []byte
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stampassets:", err)
		os.Exit(1)
	}
}

func run() error {
	if info, err := os.Stat(siteDir); err != nil || !info.IsDir() {
		fmt.Println("stampassets: no site at", siteDir+", nothing to stamp")
		return nil
	}
	pages, err := sitePages()
	if err != nil {
		return err
	}

	hashes := map[string]string{}
	var pending []stamped
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			return err
		}
		out, err := versioned(page, data, hashes)
		if err != nil {
			return err
		}
		if !bytes.Equal(out, data) {
			pending = append(pending, stamped{page, out})
		}
	}

	if len(pending) == 0 {
		fmt.Println("stampassets: site assets already versioned")
		return nil
	}
	fmt.Println("stampassets: stamped asset versions into:")
	for _, p := range pending {
		info, err := os.Stat(p.path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(p.path, p.data, info.Mode().Perm()); err != nil {
			return err
		}
		fmt.Println("  " + p.path)
	}
	return nil
}

// sitePages lists every page under siteDir, in a stable order.
func sitePages() ([]string, error) {
	var pages []string
	err := filepath.WalkDir(siteDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), pageExt) {
			pages = append(pages, path)
		}
		return nil
	})
	sort.Strings(pages)
	return pages, err
}

// versioned returns the page with every local asset link carrying its hash. hashes caches each file's
// hash by its cleaned path, so a stylesheet shared by every page is read once.
func versioned(page string, data []byte, hashes map[string]string) ([]byte, error) {
	var missing error
	out := assetLink.ReplaceAllFunc(data, func(match []byte) []byte {
		if missing != nil {
			return match
		}
		groups := assetLink.FindSubmatch(match)
		attr, link, fragment := string(groups[1]), string(groups[2]), string(groups[3])
		if !isLocal(link) {
			return match
		}
		target := filepath.Clean(filepath.Join(filepath.Dir(page), filepath.FromSlash(link)))
		hash, ok := hashes[target]
		if !ok {
			var err error
			if hash, err = contentHash(target); err != nil {
				missing = fmt.Errorf("%s links %s: %w", page, link, err)
				return match
			}
			hashes[target] = hash
		}
		return []byte(fmt.Sprintf(`%s="%s?%s=%s%s"`, attr, link, queryKey, hash, fragment))
	})
	return out, missing
}

// isLocal reports whether a link is a path relative to the page rather than anything off-site.
func isLocal(link string) bool {
	return !strings.HasPrefix(link, "/") && !scheme.MatchString(link)
}

// contentHash is the short hash of a file's bytes with CRLF read as LF.
func contentHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(sum[:])[:hashLength], nil
}
