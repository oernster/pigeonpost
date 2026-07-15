package domain

// IDSeparator joins the components of locally minted mail identifiers: an account id and a mailbox
// path form a folder id and a folder id and a server UID form a message id. It is a control
// character that appears in neither mailbox names nor email addresses.
const IDSeparator = "\x1f"

// MessageIDFor composes a message's local identity from its folder id and server UID. It is the
// single spelling of that scheme, shared by the sync layer (which mints ids for fetched messages)
// and the action layer (which predicts a moved message's new id from the server's COPYUID reply,
// so the front end can undo a move by addressing the message where it landed).
func MessageIDFor(folderID, uid string) string {
	return folderID + IDSeparator + uid
}
