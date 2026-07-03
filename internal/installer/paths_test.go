package installer

import (
	"path/filepath"
	"testing"
)

func TestUserDataDir(t *testing.T) {
	dir, err := UserDataDir()
	if err != nil {
		t.Fatalf("user data dir: %v", err)
	}
	if filepath.Base(dir) != AppName {
		t.Errorf("UserDataDir = %q, want base %q", dir, AppName)
	}
}
