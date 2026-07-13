package caldav

import (
	"github.com/oernster/pigeonpost/internal/application"
	"github.com/oernster/pigeonpost/internal/domain"
)

// Factory builds a CalDAV Client for an account, implementing application.CalDAVSourceFactory so the
// application can create a per-account source without importing this package.
type Factory struct{}

// Factory must satisfy the application port.
var _ application.CalDAVSourceFactory = Factory{}

// NewFactory returns a CalDAV source factory.
func NewFactory() Factory { return Factory{} }

// NewSource builds a Basic-auth CalDAV client for the account using the given password.
func (Factory) NewSource(account domain.CalendarAccount, password string) (application.CalDAVSource, error) {
	return NewClient(account.BaseURL(), account.Username(), password)
}
