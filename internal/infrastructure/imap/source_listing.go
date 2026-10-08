package imap

import (
	"context"
	"fmt"
	"strconv"

	"github.com/emersion/go-imap/v2"

	"github.com/oernster/pigeonpost/internal/domain"
)

// FetchListing selects the folder read-only and answers every message's UID with its flags and tag
// keywords, without headers, plus the UIDVALIDITY of that SELECT. It is what the incremental sync reads
// first: about forty bytes a message where a summary with its envelope and body structure runs to
// kilobytes, so a folder of 17,000 messages lists in moments. The flags and keywords are mapped exactly as
// a full fetch maps them (mapFlags, tagKeywords). It is the application.MailSource listing.
func (s *Source) FetchListing(ctx context.Context, account domain.Account, folder domain.Folder) (listing []domain.MessageState, validity uint32, err error) {
	done := traceStep(account, "list messages", folder.Path())
	defer func() { done(len(listing), err) }()
	client, err := s.connect(ctx, account)
	if err != nil {
		return nil, 0, err
	}
	defer s.release(ctx, client)

	selected, err := client.Select(folder.Path(), &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("imap: select %q: %w", folder.Path(), err)
	}
	if selected.NumMessages == 0 {
		return nil, selected.UIDValidity, nil
	}
	seqSet := imap.SeqSet{}
	seqSet.AddRange(1, selected.NumMessages)
	buffers, err := client.Fetch(seqSet, &imap.FetchOptions{UID: true, Flags: true}).Collect()
	if err != nil {
		return nil, 0, fmt.Errorf("imap: list %q: %w", folder.Path(), markUnreadable(err))
	}
	listing = make([]domain.MessageState, 0, len(buffers))
	for _, buf := range buffers {
		state, err := domain.NewMessageState(strconv.FormatUint(uint64(buf.UID), 10), mapFlags(buf.Flags), tagKeywords(buf.Flags))
		if err != nil {
			return nil, 0, fmt.Errorf("imap: list %q: uid %d: %w", folder.Path(), uint32(buf.UID), err)
		}
		listing = append(listing, state)
	}
	return listing, selected.UIDValidity, nil
}
