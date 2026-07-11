package main

// CreateFolder creates a new mailbox on the account's server and refreshes the cached folder list.
func (a *App) CreateFolder(accountID, name string) error {
	return a.folders.Create(a.ctx, accountID, name)
}

// RenameFolder renames a folder on the server and refreshes the cached folder list.
func (a *App) RenameFolder(folderID, newName string) error {
	return a.folders.Rename(a.ctx, folderID, newName)
}

// DeleteFolder deletes a folder on the server, clears its cached messages and refreshes the list.
func (a *App) DeleteFolder(folderID string) error {
	return a.folders.Delete(a.ctx, folderID)
}

// MoveFolder reparents a folder under a new parent on the server (an empty newParentID moves it to the
// top level) and refreshes the cached folder list. It backs the drag-and-drop reparent.
func (a *App) MoveFolder(folderID, newParentID string) error {
	return a.folders.Move(a.ctx, folderID, newParentID)
}
