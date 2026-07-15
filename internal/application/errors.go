package application

import "errors"

// ErrAccountNotFound is returned by an AccountStore when no account matches the given id.
var ErrAccountNotFound = errors.New("account not found")

// ErrCalendarAccountNotFound is returned by a CalendarAccountStore when no CalDAV/CardDAV account matches
// the given id.
var ErrCalendarAccountNotFound = errors.New("calendar account not found")

// ErrBodyNotCached is returned by a MailStore when a message's full body has not been fetched yet.
var ErrBodyNotCached = errors.New("message body not cached")

// ErrNoDraftsFolder is returned when a draft cannot be saved because the account has no Drafts mailbox.
var ErrNoDraftsFolder = errors.New("account has no drafts folder")

// ErrUnknownSender is returned when a message asks to be sent from an address the account does not own
// (neither its primary address nor one of its configured identities), so it cannot be sent as it.
var ErrUnknownSender = errors.New("account cannot send as that address")

// ErrNoJunkFolder is returned when a message cannot be marked as junk because the account has no Junk
// (spam) mailbox to file it in.
var ErrNoJunkFolder = errors.New("account has no junk folder")

// ErrScheduleInPast is returned when a send-later request names an instant that is not in the future,
// so a stale picker can never fire a message the moment it is scheduled.
var ErrScheduleInPast = errors.New("the scheduled send time is in the past")

// ErrSnoozeInPast is returned when a snooze names an instant that is not in the future, so a stale
// picker can never hide a message that would resurface immediately.
var ErrSnoozeInPast = errors.New("the snooze time is in the past")

// ErrAlreadyJunk is returned when a message that already lives in the Junk folder is marked as junk.
var ErrAlreadyJunk = errors.New("message is already in the junk folder")

// ErrNotInJunk is returned when a message outside the Junk folder is marked as not junk.
var ErrNotInJunk = errors.New("message is not in the junk folder")

// ErrNoInboxFolder is returned when a message cannot be rescued from junk because the account has
// no Inbox folder to return it to.
var ErrNoInboxFolder = errors.New("account has no inbox folder")

// ErrEmptyFolderName is returned when a folder create or rename is given a blank name.
var ErrEmptyFolderName = errors.New("folder name is empty")

// ErrFolderMoveAcrossAccounts is returned when a folder move names a target parent belonging to a
// different account; a folder can only be moved within its own account's mailbox tree.
var ErrFolderMoveAcrossAccounts = errors.New("cannot move a folder to another account")

// ErrFolderMoveIntoSelf is returned when a folder move would reparent a folder under itself or under
// one of its own descendants, which no mailbox tree can represent.
var ErrFolderMoveIntoSelf = errors.New("cannot move a folder into itself or its own subtree")

// ErrNoInvite is returned when a message carries no text/calendar scheduling payload to act on.
var ErrNoInvite = errors.New("message carries no meeting invitation")

// ErrNotInvitable is returned when a response is asked for a message that is not a meeting REQUEST.
var ErrNotInvitable = errors.New("scheduling message is not a meeting request")

// ErrNotCancellation is returned when a cancellation is applied to a message that is not a CANCEL.
var ErrNotCancellation = errors.New("scheduling message is not a cancellation")

// ErrNotReply is returned when a reply is applied to a message that is not a REPLY.
var ErrNotReply = errors.New("scheduling message is not a reply")

// ErrNoOrganizer is returned when a meeting names no organizer to send a reply to.
var ErrNoOrganizer = errors.New("meeting has no organizer to reply to")

// ErrNoReplyAttendee is returned when a REPLY carries no attendee whose status could be applied.
var ErrNoReplyAttendee = errors.New("reply carries no attendee")

// ErrMeetingNotFound is returned when no stored meeting matches an incoming reply.
var ErrMeetingNotFound = errors.New("no matching meeting to update")
