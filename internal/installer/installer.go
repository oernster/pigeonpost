// Package installer holds the per-user install logic for the PigeonPost setup program: payload
// extraction (cross-platform, testable) and, in windows.go, the registry and shortcut side effects.
package installer

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// AppName is the product name shown to the user and used for the install folder and registry key.
	AppName = "PigeonPost"
	// ExeName is the installed application executable.
	ExeName = "PigeonPost.exe"
	// Publisher is recorded in the uninstall registry entry.
	Publisher = "Oliver Ernster"

	installSubdir = "Programs"
	dirPerm       = 0o755
)

// InstallDir returns the per-user install directory: %LOCALAPPDATA%\Programs\PigeonPost. Installing
// under LOCALAPPDATA keeps the whole flow admin-free.
func InstallDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", fmt.Errorf("LOCALAPPDATA is not set")
	}
	return filepath.Join(base, installSubdir, AppName), nil
}

// UserDataDir returns the per-user data directory that holds the database and settings. It mirrors
// the location the application itself uses, so uninstall can optionally remove it.
func UserDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(base, AppName), nil
}

// ExtractZip extracts a zip archive into dest, creating directories as needed and rejecting any
// entry whose path would escape dest (zip-slip protection).
func ExtractZip(data []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open payload: %w", err)
	}
	if err := os.MkdirAll(dest, dirPerm); err != nil {
		return fmt.Errorf("create install dir %q: %w", dest, err)
	}
	for _, file := range reader.File {
		if err := extractEntry(file, dest); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(file *zip.File, dest string) error {
	target := filepath.Join(dest, file.Name)
	cleanDest := filepath.Clean(dest) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) {
		return fmt.Errorf("unsafe path in payload: %q", file.Name)
	}
	if file.FileInfo().IsDir() {
		return os.MkdirAll(target, dirPerm)
	}
	if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
		return fmt.Errorf("create dir for %q: %w", target, err)
	}
	src, err := file.Open()
	if err != nil {
		return fmt.Errorf("open entry %q: %w", file.Name, err)
	}
	defer src.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, dirPerm)
	if err != nil {
		return fmt.Errorf("create %q: %w", target, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("write %q: %w", target, err)
	}
	return nil
}

// DirSizeKB returns the total size of a directory tree in kilobytes, used for the uninstall entry's
// EstimatedSize value.
func DirSizeKB(dir string) (uint32, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("size %q: %w", dir, err)
	}
	return uint32(total / 1024), nil
}
