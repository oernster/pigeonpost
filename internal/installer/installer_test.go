package installer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %q: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestExtractZip(t *testing.T) {
	data := makeZip(t, map[string]string{
		"PigeonPost.exe":      "binary",
		"assets/appicon.png":  "icon",
		"assets/nested/a.txt": "deep",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err != nil {
		t.Fatalf("extract: %v", err)
	}
	for name, want := range map[string]string{
		"PigeonPost.exe":      "binary",
		"assets/appicon.png":  "icon",
		"assets/nested/a.txt": "deep",
	} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatalf("read %q: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("%q = %q, want %q", name, got, want)
		}
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	data := makeZip(t, map[string]string{"../escape.txt": "evil"})
	if err := ExtractZip(data, t.TempDir()); err == nil {
		t.Fatal("expected zip-slip to be rejected")
	}
}

func TestExtractZipRejectsGarbage(t *testing.T) {
	if err := ExtractZip([]byte("not a zip"), t.TempDir()); err == nil {
		t.Fatal("expected error on invalid archive")
	}
}

func TestInstallDir(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join("X:", "Local"))
	dir, err := InstallDir()
	if err != nil {
		t.Fatalf("install dir: %v", err)
	}
	want := filepath.Join("X:", "Local", "Programs", "PigeonPost")
	if dir != want {
		t.Errorf("InstallDir = %q, want %q", dir, want)
	}

	t.Setenv("LOCALAPPDATA", "")
	if _, err := InstallDir(); err == nil {
		t.Error("expected error when LOCALAPPDATA is unset")
	}
}

func TestDirSizeKB(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	kb, err := DirSizeKB(dir)
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	if kb != 2 {
		t.Errorf("DirSizeKB = %d, want 2", kb)
	}
}
