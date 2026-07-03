package domain

import "errors"

// Sentinel errors returned by domain constructors. Callers match with errors.Is.
var (
	ErrEmptyEmailAddress   = errors.New("email address is empty")
	ErrInvalidEmailAddress = errors.New("email address is not valid")
	ErrInvalidColour       = errors.New("colour is not a valid #rrggbb hex value")
	ErrEmptyAccountID      = errors.New("account id is empty")
	ErrEmptyDisplayName    = errors.New("display name is empty")
	ErrEmptyHost           = errors.New("server host is empty")
	ErrInvalidPort         = errors.New("server port is out of range")
	ErrEmptyFolderID       = errors.New("folder id is empty")
	ErrEmptyFolderPath     = errors.New("folder path is empty")
	ErrNegativeCount       = errors.New("count cannot be negative")
	ErrUnreadExceedsTotal  = errors.New("unread count cannot exceed total count")
	ErrEmptyMessageID      = errors.New("message id is empty")
	ErrInvalidUID          = errors.New("message uid must be positive")
	ErrNegativeSize        = errors.New("message size cannot be negative")
	ErrEmptyTagID          = errors.New("tag id is empty")
	ErrEmptyTagName        = errors.New("tag name is empty")
	ErrNoRecipients        = errors.New("message has no valid recipients")
	ErrNoSender            = errors.New("message has no sender")
)
