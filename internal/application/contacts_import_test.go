package application

import (
	"context"
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

// TestImportContactsCarriesSkippedCount: records the codec skipped (a card with no name) reach the
// result beside the stored ones, so the user is told how many were left out rather than the import
// reading as complete.
func TestImportContactsCarriesSkippedCount(t *testing.T) {
	svc := NewContactService(&fakeContactStore{}, fixedID("x"))
	codec := &fakeContactCodec{decoded: []domain.Contact{mustContact(t, "c1", "Jo")}, skipped: 2}
	got, err := svc.ImportContacts(context.Background(), codec, []byte("data"))
	if err != nil {
		t.Fatalf("ImportContacts: %v", err)
	}
	if got != (ContactImportResult{Added: 1, Skipped: 2}) {
		t.Errorf("result = %+v, want 1 added and 2 skipped", got)
	}
}
