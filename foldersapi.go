package main

// CreateFolder creates a new mailbox on the account's server and refreshes the cached folder list.
func (a *App) CreateFolder(accountID, name string) error {
	return a.folders.Create(a.ctx, accountID, name)
}

// CreateSubfolder creates a new mailbox directly under an existing folder on that folder's account
// and refreshes the cached folder list. name is the new folder's leaf name, never a path.
func (a *App) CreateSubfolder(parentFolderID, name string) error {
	return a.folders.CreateChild(a.ctx, parentFolderID, name)
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

// FolderUIState returns the account's saved folder display state (custom-folder order and collapsed
// paths). The front end reads it at account load and falls back to its own cache when the call fails.
func (a *App) FolderUIState(accountID string) (FolderUIStateDTO, error) {
	order, collapsed, err := a.folderUIState.Load(a.ctx, accountID)
	if err != nil {
		return FolderUIStateDTO{}, err
	}
	return FolderUIStateDTO{Order: order, Collapsed: collapsed}, nil
}

// SaveFolderUIState replaces the account's saved folder display state. The front end sends the full
// state after every reorder or collapse change, so the stored row never holds a partial update.
func (a *App) SaveFolderUIState(accountID string, order, collapsed []string) error {
	return a.folderUIState.Save(a.ctx, accountID, order, collapsed)
}
