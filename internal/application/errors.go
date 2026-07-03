package application

import "errors"

// ErrAccountNotFound is returned by an AccountStore when no account matches the given id.
var ErrAccountNotFound = errors.New("account not found")

// ErrBodyNotCached is returned by a MailStore when a message's full body has not been fetched yet.
var ErrBodyNotCached = errors.New("message body not cached")
