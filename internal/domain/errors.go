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
	ErrEmptyBaseURL        = errors.New("calendar account base url is empty")
	ErrInvalidBaseURL      = errors.New("calendar account base url must be an http or https url")
	ErrEmptyUsername       = errors.New("calendar account username is empty")
	ErrEmptyFolderID       = errors.New("folder id is empty")
	ErrEmptyFolderPath     = errors.New("folder path is empty")
	ErrNegativeCount       = errors.New("count cannot be negative")
	ErrUnreadExceedsTotal  = errors.New("unread count cannot exceed total count")
	ErrEmptyMessageID      = errors.New("message id is empty")
	ErrInvalidUID          = errors.New("message uid must not be empty")
	ErrNegativeSize        = errors.New("message size cannot be negative")
	ErrEmptyTagID          = errors.New("tag id is empty")
	ErrEmptyTagName        = errors.New("tag name is empty")
	ErrEmptyTagKeyword     = errors.New("tag keyword is empty")
	ErrDuplicateTag        = errors.New("a tag with that name already exists")
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
	// ErrInvalidRuleMatchMode, ErrNoRuleConditions, ErrNoRuleActions and ErrMissingRuleFolder guard the
	// multi-condition rule shape: a rule needs a recognised match mode, at least one condition and at
	// least one action; a move action needs somewhere to move to.
	ErrInvalidRuleMatchMode = errors.New("rule match mode is not valid")
	ErrNoRuleConditions     = errors.New("rule has no conditions")
	ErrNoRuleActions        = errors.New("rule has no actions")
	ErrMissingRuleFolder    = errors.New("rule move action has no destination folder")
	ErrEmptyTemplateID      = errors.New("template id is empty")
	ErrEmptyTemplateName    = errors.New("template name is empty")
	ErrEmptyAttachmentName  = errors.New("attachment filename is empty")

	ErrEmptyContactID        = errors.New("contact id is empty")
	ErrEmptyContactName      = errors.New("contact formatted name is empty")
	ErrEmptyPhoneNumber      = errors.New("contact phone number is empty")
	ErrEmptyAddress          = errors.New("contact address is empty")
	ErrEmptyContactGroupID   = errors.New("contact group id is empty")
	ErrEmptyContactGroupName = errors.New("contact group name is empty")

	ErrEmptyCalendarID      = errors.New("calendar id is empty")
	ErrEmptyCalendarName    = errors.New("calendar name is empty")
	ErrEmptyEventID         = errors.New("event id is empty")
	ErrEmptyEventSummary    = errors.New("event summary is empty")
	ErrEmptyEventStart      = errors.New("event start time is not set")
	ErrEventEndsBeforeStart = errors.New("event end is before its start")

	ErrEmptyOrganizerAddress      = errors.New("organiser address is empty")
	ErrEmptyAttendeeAddress       = errors.New("attendee address is empty")
	ErrInvalidParticipationStatus = errors.New("participation status is not valid")
	ErrInvalidRole                = errors.New("attendee role is not valid")
	ErrInvalidMethod              = errors.New("scheduling method is not valid")
	ErrNoSchedulingEvents         = errors.New("scheduling message has no events")
	ErrEmptyCalendarPart          = errors.New("calendar part content is empty")

	ErrEmptySearchTerm      = errors.New("search term is empty")
	ErrEmptySearchGroup     = errors.New("search group has no terms")
	ErrNegatedTermInGroup   = errors.New("negated search term does not belong in a positive group")
	ErrPositiveTermExcluded = errors.New("positive search term does not belong in the exclusions")
)

// ErrOffline marks a failure caused by the mail server being unreachable (a connection could not be
// established), as opposed to the server rejecting a well-formed request. Infrastructure adapters wrap
// connection failures with it so the application layer can queue the operation for later rather than
// surfacing it as a hard error. Callers match with errors.Is.
var ErrOffline = errors.New("mail server is unreachable")

// ErrIMAPRefused marks a mailbox that accepted the credential and then refused an IMAP session. It says
// only that, because only that was observed. IMAP being switched off is the common cause on a personal
// Microsoft account and is not the only one: the same refusal was measured against a mailbox whose IMAP
// switch was on and had stayed on, days after it was created, while aged mailboxes on the same build
// connected normally.
//
// The name once said Disabled and the text once said "imap is not enabled for this mailbox", which put a
// guess where an observation belongs and sent people to change a setting that was already correct.
// Infrastructure adapters wrap the server's refusal with this so the interface can say something useful
// rather than quoting the protocol. Callers match with errors.Is.
var ErrIMAPRefused = errors.New("server refused an imap session")
