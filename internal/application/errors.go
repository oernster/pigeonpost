package application

import "errors"

// ErrAccountNotFound is returned by an AccountStore when no account matches the given id.
var ErrAccountNotFound = errors.New("account not found")
