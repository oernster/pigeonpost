package domain

import "strings"

// folderPathSeparator is the IMAP-style hierarchy separator used to derive a folder's leaf name.
const folderPathSeparator = "/"

// FolderKind classifies a folder by its role. Custom is any user-created folder.
type FolderKind int

const (
	FolderInbox FolderKind = iota
	FolderSent
	FolderDrafts
	FolderTrash
	FolderJunk
	FolderArchive
	FolderCustom
)

// String returns a stable identifier for the folder kind.
func (k FolderKind) String() string {
	switch k {
	case FolderInbox:
		return "inbox"
	case FolderSent:
		return "sent"
	case FolderDrafts:
		return "drafts"
	case FolderTrash:
		return "trash"
	case FolderJunk:
		return "junk"
	case FolderArchive:
		return "archive"
	case FolderCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// sentLikeLeafNames are the well-known leaf names mail clients have used for a sent mailbox, lowercased.
// A folder wearing one of these names is sent-like even when it is not the account's canonical Sent, so
// sync can merge strays (an old client's "Sent Messages") into the one real Sent folder. The IMAP
// name-classification table mirrors these entries for its sent role.
var sentLikeLeafNames = map[string]struct{}{
	"sent":          {},
	"sent items":    {},
	"sent mail":     {},
	"sent messages": {},
}

// Folder is a mailbox within an account, with cached unread and total message counts. The separator is
// the server's mailbox hierarchy delimiter, kept so the leaf name and a rename path are derived
// correctly on servers that do not use "/" (StartMail, for example, uses "."). specialUse records that
// the server itself declared the folder's role via an RFC 6154 attribute; it is carried in memory from
// a live folder fetch and is not persisted.
type Folder struct {
	id         string
	accountID  string
	path       string
	name       string
	separator  string
	kind       FolderKind
	unread     int
	total      int
	specialUse bool
}

// NewFolder validates and constructs a folder using the default "/" hierarchy separator. It is a
// convenience over NewFolderWithSeparator for callers (and tests) that do not track the delimiter.
func NewFolder(id, accountID, path string, kind FolderKind, unread, total int) (Folder, error) {
	return NewFolderWithSeparator(id, accountID, path, folderPathSeparator, kind, unread, total)
}

// NewFolderWithSeparator validates and constructs a folder whose hierarchy delimiter is separator. An
// empty separator falls back to the default "/". The leaf name is derived from the path using it.
func NewFolderWithSeparator(id, accountID, path, separator string, kind FolderKind, unread, total int) (Folder, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Folder{}, ErrEmptyFolderID
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return Folder{}, ErrEmptyAccountID
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return Folder{}, ErrEmptyFolderPath
	}
	if err := validateCounts(unread, total); err != nil {
		return Folder{}, err
	}
	if separator == "" {
		separator = folderPathSeparator
	}
	return Folder{
		id:        id,
		accountID: accountID,
		path:      path,
		name:      leafName(path, separator),
		separator: separator,
		kind:      kind,
		unread:    unread,
		total:     total,
	}, nil
}

func validateCounts(unread, total int) error {
	if unread < 0 || total < 0 {
		return ErrNegativeCount
	}
	if unread > total {
		return ErrUnreadExceedsTotal
	}
	return nil
}

func leafName(path, separator string) string {
	if idx := strings.LastIndex(path, separator); idx >= 0 {
		return path[idx+len(separator):]
	}
	return path
}

// ID returns the folder identifier.
func (f Folder) ID() string { return f.id }

// AccountID returns the owning account identifier.
func (f Folder) AccountID() string { return f.accountID }

// Path returns the full hierarchical path.
func (f Folder) Path() string { return f.path }

// Name returns the leaf name derived from the path.
func (f Folder) Name() string { return f.name }

// Separator returns the server's mailbox hierarchy delimiter for this folder.
func (f Folder) Separator() string { return f.separator }

// Kind returns the folder role.
func (f Folder) Kind() FolderKind { return f.kind }

// Unread returns the cached unread count.
func (f Folder) Unread() int { return f.unread }

// Total returns the cached total message count.
func (f Folder) Total() int { return f.total }

// RenamedTo returns the full path this folder would have if its leaf name were changed to newLeaf,
// keeping the same parent hierarchy. It builds the destination path for a server-side rename.
func (f Folder) RenamedTo(newLeaf string) string {
	if idx := strings.LastIndex(f.path, f.separator); idx >= 0 {
		return f.path[:idx+len(f.separator)] + newLeaf
	}
	return newLeaf
}

// IsSentLike reports whether this folder is the account Sent or wears a well-known sent leaf name. The
// sync-time reconciler uses it to find stray sent folders left behind by other clients.
func (f Folder) IsSentLike() bool {
	if f.kind == FolderSent {
		return true
	}
	_, ok := sentLikeLeafNames[strings.ToLower(strings.TrimSpace(f.name))]
	return ok
}

// SpecialUse reports whether the server declared this folder's role via an RFC 6154 special-use
// attribute rather than the role being inferred from its name. A declared folder's position is the
// server's own choice (Gmail keeps Sent under [Gmail], for example), so reconciliation never moves it.
func (f Folder) SpecialUse() bool { return f.specialUse }

// WithSpecialUse returns a copy marked as declared (or not) by the server's special-use attributes.
func (f Folder) WithSpecialUse(declared bool) Folder {
	copied := f
	copied.specialUse = declared
	return copied
}

// WithCounts returns a copy with new unread and total counts, validated.
func (f Folder) WithCounts(unread, total int) (Folder, error) {
	if err := validateCounts(unread, total); err != nil {
		return Folder{}, err
	}
	copied := f
	copied.unread = unread
	copied.total = total
	return copied, nil
}
