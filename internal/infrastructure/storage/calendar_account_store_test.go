package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// The store must satisfy the application CalendarAccountStore port.
var _ application.CalendarAccountStore = (*Store)(nil)

func buildCalendarAccount(t *testing.T, id string) domain.CalendarAccount {
	t.Helper()
	account, err := domain.NewCalendarAccount(id, "Fastmail",
		"https://caldav.fastmail.com/", "user@fastmail.com", domain.AuthPassword)
	if err != nil {
		t.Fatalf("calendar account: %v", err)
	}
	return account
}

func TestCalendarAccountRoundTrip(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	if err := store.SaveCalendarAccount(ctx, buildCalendarAccount(t, "c1")); err != nil {
		t.Fatalf("save: %v", err)
	}
	accounts, err := store.ListCalendarAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}
	got := accounts[0]
	if got.ID() != "c1" || got.DisplayName() != "Fastmail" || got.BaseURL() != "https://caldav.fastmail.com/" ||
		got.Username() != "user@fastmail.com" || got.Auth() != domain.AuthPassword {
		t.Errorf("account not round-tripped: %+v", got)
	}

	fetched, err := store.GetCalendarAccount(ctx, "c1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.ID() != "c1" {
		t.Errorf("GetCalendarAccount id = %q", fetched.ID())
	}
}

func TestSaveCalendarAccountReplaces(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveCalendarAccount(ctx, buildCalendarAccount(t, "c1")); err != nil {
		t.Fatalf("save: %v", err)
	}
	renamed, err := buildCalendarAccount(t, "c1").WithDisplayName("Work DAV")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := store.SaveCalendarAccount(ctx, renamed); err != nil {
		t.Fatalf("resave: %v", err)
	}
	accounts, err := store.ListCalendarAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 1 || accounts[0].DisplayName() != "Work DAV" {
		t.Errorf("replace failed: %+v", accounts)
	}
}

func TestGetCalendarAccountNotFound(t *testing.T) {
	store := openTestStore(t)
	if _, err := store.GetCalendarAccount(context.Background(), "missing"); !errors.Is(err, application.ErrCalendarAccountNotFound) {
		t.Errorf("error = %v, want ErrCalendarAccountNotFound", err)
	}
}

func TestDeleteCalendarAccount(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	if err := store.SaveCalendarAccount(ctx, buildCalendarAccount(t, "c1")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := store.DeleteCalendarAccount(ctx, "c1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	accounts, err := store.ListCalendarAccounts(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("account not deleted: %+v", accounts)
	}
	// Deleting a missing account is not an error.
	if err := store.DeleteCalendarAccount(ctx, "missing"); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}
