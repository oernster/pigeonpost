package caldav

import (
	"testing"

	"github.com/oernster/pigeonpost/internal/domain"
)

func TestFactoryNewSource(t *testing.T) {
	account, err := domain.NewCalendarAccount("c1", "Fastmail", "https://caldav.fastmail.com/", "user", domain.AuthPassword)
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	source, err := NewFactory().NewSource(account, "secret")
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	if source == nil {
		t.Fatal("expected a source")
	}
}
