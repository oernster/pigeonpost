package application

import "errors"

// ErrAccountNotFound is returned by an AccountStore when no account matches the given id.
var ErrAccountNotFound = errors.New("account not found")

// ErrBodyNotCached is returned by a MailStore when a message's full body has not been fetched yet.
var ErrBodyNotCached = errors.New("message body not cached")

// ErrNoDraftsFolder is returned when a draft cannot be saved because the account has no Drafts mailbox.
var ErrNoDraftsFolder = errors.New("account has no drafts folder")

// ErrEmptyFolderName is returned when a folder create or rename is given a blank name.
var ErrEmptyFolderName = errors.New("folder name is empty")
