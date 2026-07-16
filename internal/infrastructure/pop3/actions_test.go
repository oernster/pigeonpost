package pop3

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func TestLocalOnlyFlagsAreNoOps(t *testing.T) {
	source := NewSource(nil)
	var account domain.Account
	var folder domain.Folder
	if err := source.SetSeen(context.Background(), account, folder, "u1", true); err != nil {
		t.Errorf("SetSeen should be a server no-op, got %v", err)
	}
	if err := source.SetFlagged(context.Background(), account, folder, "u1", true); err != nil {
		t.Errorf("SetFlagged should be a server no-op, got %v", err)
	}
	if err := source.SetAnswered(context.Background(), account, folder, "u1", true); err != nil {
		t.Errorf("SetAnswered should be a server no-op, got %v", err)
	}
	if err := source.SetForwarded(context.Background(), account, folder, "u1", true); err != nil {
		t.Errorf("SetForwarded should be a server no-op, got %v", err)
	}
	if err := source.SetKeyword(context.Background(), account, folder, "u1", "$PPtag_abc", true); err != nil {
		t.Errorf("SetKeyword should be a server no-op, got %v", err)
	}
}

func TestMoveAndCopyAreUnsupported(t *testing.T) {
	source := NewSource(nil)
	var account domain.Account
	var folder domain.Folder
	if _, err := source.Move(context.Background(), account, folder, "u1", "dest"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Move error = %v, want ErrUnsupported", err)
	}
	if _, err := source.MoveMany(context.Background(), account, folder, []string{"u1"}, "dest"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("MoveMany error = %v, want ErrUnsupported", err)
	}
	if _, err := source.Copy(context.Background(), account, folder, "u1", "dest"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("Copy error = %v, want ErrUnsupported", err)
	}
}
