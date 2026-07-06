package pop3

import (
	"bytes"
	"fmt"

	"github.com/emersion/go-message"
	// Registers a CharsetReader so encoded-word headers in legacy charsets decode instead of failing.
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"

	"github.com/oernster/pigeonpost/internal/domain"
	"github.com/oernster/pigeonpost/internal/infrastructure/mailparse"
)

// idSeparator joins an account, the synthetic mailbox and a message UIDL into stable local
// identifiers. It is a control character that does not appear in mailbox names, email addresses or
// UIDLs, matching the identifier scheme used by the other mail adapters.
const idSeparator = "\x1f"

// inboxPath is the single synthetic mailbox a POP3 account exposes; POP3 has no server-side folders.
const inboxPath = "INBOX"

// inboxID is the local id of a POP3 account's synthetic Inbox folder.
func inboxID(accountID string) string {
	return accountID + idSeparator + inboxPath
}

// makeMessageID builds a message's stable local id from its folder and UIDL.
func makeMessageID(folderID, uid string) string {
	return folderID + idSeparator + uid
}

// numberForUID resolves a stored UIDL back to its session-local message number for the current
// connection, since RETR and TOP address messages by number, not UIDL.
func numberForUID(items []UIDItem, uid string) (int, bool) {
	for _, item := range items {
		if item.UID == uid {
			return item.Number, true
		}
	}
	return 0, false
}

// buildSummary parses a message's header bytes (from TOP or the head of RETR) into a domain summary,
// keyed by its UIDL. POP3 carries no server-side flags, so the summary starts unread and unflagged.
func buildSummary(folderID, uid string, header []byte, size int) (domain.MessageSummary, error) {
	entity, err := message.Read(bytes.NewReader(header))
	if err != nil && !message.IsUnknownCharset(err) {
		return domain.MessageSummary{}, fmt.Errorf("pop3: parse header: %w", err)
	}
	h := mail.Header{Header: entity.Header}
	subject, _ := h.Subject()
	// Subject() decodes RFC 2047 encoded-words; DecodeHeader adds the HTML-entity unescape (so a
	// template-built "Data &amp; X" reads as "Data & X"), matching the IMAP path.
	subject = mailparse.DecodeHeader(subject)
	date, _ := h.Date()
	messageID, _ := h.MessageID()
	return domain.NewMessageSummary(domain.MessageSummaryInput{
		ID:        makeMessageID(folderID, uid),
		FolderID:  folderID,
		UID:       uid,
		MessageID: messageID,
		From:      firstAddress(h.AddressList("From")),
		To:        allAddresses(h.AddressList("To")),
		Cc:        allAddresses(h.AddressList("Cc")),
		Subject:   subject,
		Date:      date,
		Size:      size,
		Flags:     domain.NewFlags(0),
	})
}

// firstAddress maps the first parseable address of a header list into a domain address, yielding the
// zero address when the list is empty or unparseable so a missing sender never fails a whole sync.
func firstAddress(addrs []*mail.Address, err error) domain.EmailAddress {
	if err != nil || len(addrs) == 0 {
		return domain.EmailAddress{}
	}
	return toDomainAddress(addrs[0])
}

// allAddresses maps every parseable address of a header list into domain addresses, skipping any that
// do not parse. Used for the To and Cc lists so reply-all can address the whole conversation.
func allAddresses(addrs []*mail.Address, err error) []domain.EmailAddress {
	if err != nil {
		return nil
	}
	out := make([]domain.EmailAddress, 0, len(addrs))
	for _, a := range addrs {
		if addr := toDomainAddress(a); !addr.IsZero() {
			out = append(out, addr)
		}
	}
	return out
}

// toDomainAddress converts one parsed mail address into a domain address, yielding the zero address on
// a nil or invalid address.
func toDomainAddress(a *mail.Address) domain.EmailAddress {
	if a == nil {
		return domain.EmailAddress{}
	}
	email, err := domain.NewEmailAddress(mailparse.DecodeHeader(a.Name), a.Address)
	if err != nil {
		return domain.EmailAddress{}
	}
	return email
}
