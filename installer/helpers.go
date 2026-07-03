package main

import (
	"io"
	"os"
	"strconv"
	"strings"
)

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// semverNewer reports whether version a is strictly newer than version b, comparing the numeric
// major.minor.patch fields and ignoring any pre-release suffix.
func semverNewer(a, b string) bool {
	pa, pb := parseSemver(a), parseSemver(b)
	for i := 0; i < len(pa); i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	core := strings.SplitN(strings.TrimSpace(v), "-", 2)[0]
	parts := strings.Split(core, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		out[i] = n
	}
	return out
}
