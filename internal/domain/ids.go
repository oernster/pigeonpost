package domain

import "strings"

// IDSeparator joins the components of locally minted mail identifiers: an account id and a mailbox
// path form a folder id and a folder id and a server UID form a message id. It is a control
// character that appears in neither mailbox names nor email addresses.
const IDSeparator = "\x1f"

// FolderIDFor composes a folder's local identity from its account id and mailbox path. The two are
// joined by the separator above and nothing else, so the pair can always be recovered with
// SplitFolderID: that is what lets a rules file name a destination as an account plus a path, which
// reads as something a person wrote, rather than as an opaque id carrying a control character.
func FolderIDFor(accountID, path string) string {
	return accountID + IDSeparator + path
}

// SplitFolderID recovers the account id and mailbox path from a folder id. An id with no separator in
// it (or an empty one, which is what a non-move action carries) yields two empty strings rather than a
// guess, so a caller cannot mistake a malformed id for a folder at the top level of some account.
func SplitFolderID(folderID string) (accountID, path string) {
	account, mailbox, found := strings.Cut(folderID, IDSeparator)
	if !found {
		return "", ""
	}
	return account, mailbox
}

// MessageIDFor composes a message's local identity from its folder id and server UID. It is the
// single spelling of that scheme, shared by the sync layer (which mints ids for fetched messages)
// and the action layer (which predicts a moved message's new id from the server's COPYUID reply,
// so the front end can undo a move by addressing the message where it landed).
func MessageIDFor(folderID, uid string) string {
	return folderID + IDSeparator + uid
}
