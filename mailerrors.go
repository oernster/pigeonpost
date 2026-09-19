package main

// mailerrors.go is the single point where a mail error crossing from Go into the user interface is made
// fit to read. Infrastructure adapters mark a failure with a domain sentinel; the technical detail that
// carries (the host and port, "dial", the server's own tagged response) is meaningless to a user, so the
// Wails facade runs every mail error through friendlyMailError before returning it.

import (
	"errors"

	"github.com/oernster/pigeonpost/internal/application"
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

// errSMTPRefused is the message shown when the server takes the sign-in and then refuses to accept mail
// from a client. It describes exactly that and no more, for the same reason errIMAPRefused does.
//
// The refusal is at the mailbox rather than at the credential: measured on 2026-09-08 against a personal
// Hotmail mailbox created that day, which PigeonPost was reading over IMAP at the same moment and which
// could send from Outlook on the web. So the message does not say the password is wrong, does not say the
// account was added the wrong way and does not send anyone to re-add it, because none of that was what
// was wrong.
//
// It deliberately does NOT repeat Microsoft's own link. That page describes a per-mailbox setting an
// administrator turns on in a work or school tenant; a personal Outlook.com or Hotmail account has no
// administrator and no such switch, so following it leads to a control the reader cannot reach. Sending
// someone to a page they cannot act on is the same failure as naming a setting that is already correct.
//
// What it offers instead is the one thing that has been observed to change: the mailbox getting older.
// This is the same shape as the IMAP refusal above, on the same provider, for the same reason.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errSMTPRefused = errors.New(
	"Microsoft accepted the sign-in then refused to send. Sending from an email app is switched off " +
		"for this mailbox, which Microsoft does to new personal accounts; the same account can still " +
		"send on the web. There is nothing to switch on, so try again later.")

// errSignInRefused is the message shown when the server declined the credential offered for a mailbox.
//
// It does not say the password is wrong. A tagged refusal carries no machine-readable reason, so a
// mistyped password, an app password the account holder has since revoked and a provider that has
// stopped taking plain passwords all arrive identically. Naming one of those as the cause is the mistake
// this file already records having made over IMAP being switched off.
//
// What it does instead is name the two things worth trying, in the order they are worth trying; then it stops.
// The app-password hint is a condition the reader checks against their own account rather than a claim
// about this failure; where the server states that condition itself, the message below is shown instead.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errSignInRefused = errors.New(
	"The mail server refused the sign-in for this account. Check the address and the password in " +
		"Settings. An account with two-step verification needs an app password from the provider " +
		"rather than the usual account password.")

// errAppPasswordRequired is the message shown when the server itself states that an application-specific
// password is wanted. It asserts nothing: the remedy it names is the one the server named.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errAppPasswordRequired = errors.New(
	"The mail server refused the sign-in and asked for an app password. Create one with your mail " +
		"provider, then put it in Settings in place of the account password.")

// errUnreadableResponse is the message shown when the server sent a reply the mail client could not
// decode. The reader saw the grammar production that ran out ("in body-type-1part: imapwire: expected " +
// "'(', got N") until 2026-09-19, which reads as a broken application and offers nothing to act on.
//
// It names no cause, because there is nothing the reader owns that is wrong: the exchange itself failed.
// It says what was kept and where the original went, so a report can carry the detail the message drops.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errUnreadableResponse = errors.New(
	"The mail server sent a reply this app could not read, so that folder was left as it was. " +
		"Nothing has been changed or lost. The server's own words are kept in the error log if you " +
		"want to report it.")

// errMessageGone is the message shown when an action addresses a message the local cache no longer
// holds. It is returned verbatim by the Wails facade and rendered as-is in the interface, so a
// capitalised, punctuated sentence is intended here.
//
// The reader used to be shown the query that failed ("scan message: sql: no rows in result set"),
// which reads as a broken application rather than as a list that has moved on.
//
//lint:ignore ST1005 user-facing message shown verbatim in the UI
var errMessageGone = errors.New(
	"That message is not in this folder any more. It was moved or removed since the list was last " +
		"read, so the list has been refreshed.")

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
// failure becomes the plain offline message, a refused session or a refused sign-in becomes the message
// describing that; a reply the client could not decode becomes a sentence rather than the grammar
// production that ran out, while every other error is returned unchanged so a genuine fault still
// surfaces its detail. A nil error stays nil.
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
	if err != nil && errors.Is(err, domain.ErrSMTPRefused) {
		return errSMTPRefused
	}
	if err != nil && errors.Is(err, application.ErrMessageNotCached) {
		return errMessageGone
	}
	// The narrower sign-in message first: where the server named the remedy, that is what the reader
	// gets; the general refusal is the fallback behind it.
	if err != nil && errors.Is(err, domain.ErrAppPasswordRequired) {
		return errAppPasswordRequired
	}
	if err != nil && errors.Is(err, domain.ErrSignInRefused) {
		return errSignInRefused
	}
	if err != nil && errors.Is(err, domain.ErrUnreadableResponse) {
		return errUnreadableResponse
	}
	return err
}
