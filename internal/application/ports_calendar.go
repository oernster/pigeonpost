package application

import (
	"context"
	"time"

	"github.com/oernster/pigeonpost/internal/domain"
)

// CalendarStore persists calendars, their events and any preserved non-event passthrough components.
type CalendarStore interface {
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	SaveCalendar(ctx context.Context, calendar domain.Calendar) error
	DeleteCalendar(ctx context.Context, id string) error
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEvent(ctx context.Context, id string) (domain.Event, error)
	SaveEvent(ctx context.Context, event domain.Event) error
	DeleteEvent(ctx context.Context, id string) error
	// SavePassthrough stores or replaces (by UID) a preserved VTODO or VJOURNAL; ListPassthrough returns
	// them all for re-export.
	SavePassthrough(ctx context.Context, passthrough domain.CalendarPassthrough) error
	ListPassthrough(ctx context.Context) ([]domain.CalendarPassthrough, error)
}

// CalDAVSource reads calendars and their objects from a remote CalDAV server. It is the read half of the
// two-way DAV sync (the infrastructure go-webdav adapter implements it), the calendar counterpart to
// MailSource. RemoteCalendar and RemoteObject are defined with the sync service in caldav.go.
type CalDAVSource interface {
	ListCalendars(ctx context.Context) ([]RemoteCalendar, error)
	ListObjects(ctx context.Context, calendar RemoteCalendar) ([]RemoteObject, error)
	// CollectionCTag returns a collection's CTag (its calendarserver.org change tag), used to skip an
	// unchanged collection on a sync. An empty string means the server does not report one, so the caller
	// reconciles unconditionally; an error is a transport or parse failure, which the caller treats the same
	// way (it cannot skip, so it reconciles).
	CollectionCTag(ctx context.Context, collectionHref string) (string, error)
}

// CalendarAccountStore persists CalDAV/CardDAV accounts. Each account's password is not stored here; it
// lives in the OS keychain, keyed by the account id, as for a mail account.
type CalendarAccountStore interface {
	SaveCalendarAccount(ctx context.Context, account domain.CalendarAccount) error
	ListCalendarAccounts(ctx context.Context) ([]domain.CalendarAccount, error)
	GetCalendarAccount(ctx context.Context, id string) (domain.CalendarAccount, error)
	DeleteCalendarAccount(ctx context.Context, id string) error
}

// CalendarCredentialStore keeps a CalDAV/CardDAV account's password in the OS keychain, never the database.
type CalendarCredentialStore interface {
	CalendarPassword(ctx context.Context, account domain.CalendarAccount) (string, error)
	SetCalendarPassword(ctx context.Context, account domain.CalendarAccount, secret string) error
	DeleteCalendarPassword(ctx context.Context, account domain.CalendarAccount) error
}

// CalDAVSourceFactory builds a CalDAVSource for an account and password. It is the seam that keeps the
// application free of the go-webdav client: the infrastructure adapter implements it.
type CalDAVSourceFactory interface {
	NewSource(account domain.CalendarAccount, password string) (CalDAVSource, error)
}

// CalendarCodec converts events to and from a serialised calendar format (ICS). It is the import/export
// seam. A decoded event carries its own id (an ICS UID where present) so an import can reconcile against
// existing records. Non-event components PigeonPost does not model (to-dos and journal entries) are
// carried as passthrough so they survive a round-trip.
type CalendarCodec interface {
	Decode(data []byte) ([]domain.Event, []domain.CalendarPassthrough, error)
	Encode(events []domain.Event, passthrough []domain.CalendarPassthrough) ([]byte, error)
}

// SchedulingCodec converts iTIP (RFC 5546) scheduling messages to and from the text/calendar payload an
// email carries (RFC 6047 iMIP). It is the seam the scheduling service uses: DecodeScheduling reads an
// incoming invite or reply (the VCALENDAR METHOD and its events, each with its organiser and attendees),
// and the encode methods build the REQUEST, REPLY and CANCEL a two-way invite flow sends back out.
type SchedulingCodec interface {
	DecodeScheduling(data []byte) (domain.SchedulingMessage, error)
	// EncodeRequest builds a METHOD:REQUEST inviting the attendees carried on the events.
	EncodeRequest(events []domain.Event) ([]byte, error)
	// EncodeCancel builds a METHOD:CANCEL withdrawing the events.
	EncodeCancel(events []domain.Event) ([]byte, error)
	// EncodeReply builds a METHOD:REPLY carrying the responder as the single attendee with the status that
	// is their answer, so the organiser sees only the response that changed.
	EncodeReply(event domain.Event, responder domain.EmailAddress, status domain.ParticipationStatus) ([]byte, error)
}

// RecurrenceService performs the recurrence operations that need RRULE parsing, kept outside the domain
// because that parsing needs a dedicated library the domain must not depend on.
type RecurrenceService interface {
	// Expand turns a recurring event's rule and recurrence dates (RRULE, RDATE, EXDATE) into the concrete
	// occurrences whose start falls within the inclusive window [from, to]. Each returned instance carries
	// a RecurrenceID equal to its own start, which identifies the occurrence.
	Expand(event domain.Event, from, to time.Time) ([]domain.EventInstance, error)
	// TruncateBefore returns the given RRULE rewritten so the series ends before at, used when a
	// this-and-future edit or delete splits or shortens a series. Any COUNT is dropped in favour of an
	// UNTIL of one second before at, so the occurrence at at and all later ones are removed.
	TruncateBefore(rule string, at time.Time) (string, error)
	// SplitCountForward returns the master's RRULE for the forward half of a this-and-following split.
	// A COUNT-based rule has its COUNT reduced by the number of occurrences that precede at, so the split
	// keeps the series total instead of restarting the count; an open-ended or UNTIL-bounded rule is
	// returned unchanged.
	SplitCountForward(master domain.Event, at time.Time) (string, error)
}
