package application

import (
	"context"

	"github.com/oernster/pigeonpost/internal/domain"
)

// FetchListing answers the folder's configured listing under its validity; a folder with none configured
// answers no UIDVALIDITY, the POP3 shape, so the sync fetches it in full.
func (f *fakeMailSource) FetchListing(_ context.Context, _ domain.Account, folder domain.Folder) ([]domain.MessageState, uint32, error) {
	if f.listingErr != nil {
		return nil, unknownUIDValidity, f.listingErr
	}
	entries, ok := f.listing[folder.ID()]
	if !ok {
		return nil, unknownUIDValidity, nil
	}
	return entries, f.validity[folder.ID()], nil
}

// FetchMessagesByUID records the UIDs asked for and answers the configured summaries holding them, under
// byUIDValidity where it is set (a folder renumbered between the two reads), else the folder's validity.
func (f *fakeMailSource) FetchMessagesByUID(_ context.Context, _ domain.Account, folder domain.Folder, uids []string) ([]domain.MessageSummary, uint32, error) {
	f.byUIDRequested = append(f.byUIDRequested, uids)
	if f.byUIDErr != nil {
		return nil, unknownUIDValidity, f.byUIDErr
	}
	wanted := make(map[string]bool, len(uids))
	for _, uid := range uids {
		wanted[uid] = true
	}
	var out []domain.MessageSummary
	for _, m := range f.messagesByFolder[folder.ID()] {
		if wanted[m.UID()] {
			out = append(out, m)
		}
	}
	validity := f.validity[folder.ID()]
	if f.byUIDValidity != unknownUIDValidity {
		validity = f.byUIDValidity
	}
	return out, validity, nil
}
