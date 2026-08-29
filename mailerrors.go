package main

// mailerrors.go is the single point where a mail error crossing from Go into the user interface is made
// fit to read. Infrastructure adapters mark a failure with a domain sentinel; the technical detail that
// carries (the host and port, "dial", the server's own tagged response) is meaningless to a user, so the
// Wails facade runs every mail error through friendlyMailError before returning it.

import (
	"errors"

	"github.com/oernster/pigeonpost/internal/domain"
)

// errOffline is the message shown when a mail action fails because the server could not be reached. It
// is returned verbatim by the Wails facade and rendered as-is in the interface, so a capitalised,
// punctuated sentence is intended here.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errOffline = errors.New("Can't reach the mail server. You may be offline; check your internet connection and try again.")

// errIMAPRefused is the message shown when the server accepts the sign-in and then refuses an IMAP
// session. It describes exactly that and no more, because the cause is not knowable from the response.
//
// The message once said IMAP was switched off for the mailbox, which is the common cause on a new
// Microsoft account and was wrong often enough to matter. It was measured wrong on 2026-08-29 against a
// mailbox whose IMAP switch was demonstrably on and had stayed on: the server still answered
// "authenticated but not connected". Two aged Hotmail accounts on the same build connected normally, so
// the client is not at fault; a mailbox created days earlier is refused whatever its settings say. The
// same failure with the same endpoint and the same scopes is reported publicly against consumer
// Outlook.com and has been unanswered by Microsoft since December 2024.
//
// So the message states the observation, gives the check that resolves the common case, then says a new
// mailbox may be refused regardless. Telling someone to turn on a setting that is already on is worse
// than saying less: they change nothing, the error returns and the app looks broken rather than blocked.
//
// It names BOTH headings the section goes by. Microsoft's documentation says "Forwarding and IMAP" while
// the current web interface calls it "Sync email"; a message that names only one sends half its readers
// hunting for a heading their Outlook does not have.
//
// It is deliberately SHORT. An earlier draft spelled out every obstacle and ran to ninety words, which
// nobody reads: an error nobody finishes is worth less than a shorter one they act on. The remaining
// detail lives in the README and behind the wizard's help link, not in a red box.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errIMAPRefused = errors.New(
	"Microsoft accepted the sign-in then refused an IMAP session. Check IMAP is on at outlook.com " +
		"under Settings, Mail, then \"Sync email\" or \"Forwarding and IMAP\". A mailbox created in " +
		"the last few days is often refused even with IMAP on, so a new account may need to wait.")

// isOffline reports whether err was caused by the mail server being unreachable (domain.ErrOffline
// wrapped anywhere in the chain), as opposed to the server rejecting a well-formed request.
func isOffline(err error) bool {
	return err != nil && errors.Is(err, domain.ErrOffline)
}

// mailErrorRecorder keeps the raw text of an error that is about to be replaced. It is the seam the
// file writing sits behind, so this layer neither opens files nor knows where they live.
type mailErrorRecorder interface {
	Record(err error)
}

// mailError is what every caller uses: it translates the error for the interface; where the
// translation replaces the original it records the original first.
//
// The condition is the whole point. A message fit to read asserts a cause; asserting a cause is
// exactly when the evidence for it stops being available: the reader is told to change a setting; if
// that was the wrong reading of the failure there is nothing left to say so. This is how a mailbox
// with IMAP already switched on came to be told, repeatedly, to switch IMAP on. An error passed through
// unchanged still carries its own detail, so it is not recorded and the log stays a list of the cases
// where something was hidden.
func (a *App) mailError(err error) error {
	friendly := friendlyMailError(err)
	if err != nil && friendly != err && a.mailErrors != nil {
		a.mailErrors.Record(err)
	}
	return friendly
}

// friendlyMailError converts an internal mail error into one fit to show the user: a connectivity
// failure becomes the plain offline message and a mailbox with IMAP switched off becomes the message
// naming that setting, while every other error is returned unchanged so a genuine fault still surfaces
// its detail. A nil error stays nil.
//
// It is kept pure and separate from the recording above so the translation can be read and tested
// without a filesystem anywhere near it.
func friendlyMailError(err error) error {
	if isOffline(err) {
		return errOffline
	}
	if err != nil && errors.Is(err, domain.ErrIMAPRefused) {
		return errIMAPRefused
	}
	return err
}
