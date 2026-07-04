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
	ErrInvalidUID          = errors.New("message uid must not be empty")
	ErrNegativeSize        = errors.New("message size cannot be negative")
	ErrEmptyTagID          = errors.New("tag id is empty")
	ErrEmptyTagName        = errors.New("tag name is empty")
	ErrNoRecipients        = errors.New("message has no valid recipients")
	ErrNoSender            = errors.New("message has no sender")
	ErrEmptyOutboxID       = errors.New("outbox item id is empty")
	ErrInvalidOutboxKind   = errors.New("outbox item kind is not valid")
	ErrEmptyRuleID         = errors.New("rule id is empty")
	ErrEmptyRuleName       = errors.New("rule name is empty")
	ErrEmptyRuleMatch      = errors.New("rule match text is empty")
	ErrInvalidRuleField    = errors.New("rule field is not valid")
	ErrInvalidRuleOperator = errors.New("rule operator is not valid")
	ErrInvalidRuleAction   = errors.New("rule action is not valid")
	ErrEmptyAttachmentName = errors.New("attachment filename is empty")

	ErrEmptyContactID        = errors.New("contact id is empty")
	ErrEmptyContactName      = errors.New("contact formatted name is empty")
	ErrEmptyPhoneNumber      = errors.New("contact phone number is empty")
	ErrEmptyContactGroupID   = errors.New("contact group id is empty")
	ErrEmptyContactGroupName = errors.New("contact group name is empty")
)

// ErrOffline marks a failure caused by the mail server being unreachable (a connection could not be
// established), as opposed to the server rejecting a well-formed request. Infrastructure adapters wrap
// connection failures with it so the application layer can queue the operation for later rather than
// surfacing it as a hard error. Callers match with errors.Is.
var ErrOffline = errors.New("mail server is unreachable")
