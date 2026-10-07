package imap

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"

	"github.com/oernster/pigeonpost/internal/domain"
)

// serverFlags maps each domain flag a replayed intent can carry to its IMAP counterpart, the inverse of
// mapFlags for the flags a user changes.
var serverFlags = map[domain.Flag]imap.Flag{
	domain.FlagSeen:      imap.FlagSeen,
	domain.FlagAnswered:  imap.FlagAnswered,
	domain.FlagFlagged:   imap.FlagFlagged,
	domain.FlagForwarded: imap.FlagForwarded,
}

// PushFlag sets or clears one flag on several messages in one folder over a single connection, then
// reads their flags back on that same connection and answers the UIDs that are settled: the server now
// reports the flag as asked (or no longer holds the message), so pushing it again could change nothing.
// A UID the server still reports otherwise is left out, so its intent stays to be retried.
//
// It exists for the replay of pending flag intents. That replay used to push each intent on a connection
// of its own and clear none of them unless the message's folder happened to be fetched; 14785 mark-read
// intents in folders no background pass fetches became a login every few seconds for as long as
// PigeonPost ran; StartMail blocked the address for it on 2026-10-07. A flag with no server
// counterpart pushes nothing and settles nothing, without connecting. It satisfies
// application.MailActions.
func (s *Source) PushFlag(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, flag domain.Flag, set bool) ([]string, error) {
	serverFlag, ok := serverFlags[flag]
	if !ok || len(uids) == 0 {
		return nil, nil
	}
	chunks, err := uidChunks(uids)
	if err != nil {
		return nil, err
	}
	client, err := s.openFolder(ctx, account, folder)
	if err != nil {
		return nil, err
	}
	defer s.release(ctx, client)

	op := imap.StoreFlagsDel
	if set {
		op = imap.StoreFlagsAdd
	}
	store := &imap.StoreFlags{Op: op, Silent: true, Flags: []imap.Flag{serverFlag}}
	for _, chunk := range chunks {
		if err := client.Store(chunk.set, store, nil).Close(); err != nil {
			return nil, fmt.Errorf("imap: store %s on %d messages: %w", serverFlag, chunk.count, err)
		}
	}
	held := make(map[imap.UID]bool, len(uids))
	agrees := make(map[imap.UID]bool, len(uids))
	for _, chunk := range chunks {
		messages, err := client.Fetch(chunk.set, &imap.FetchOptions{UID: true, Flags: true}).Collect()
		if err != nil {
			return nil, fmt.Errorf("imap: read back %s on %d messages: %w", serverFlag, chunk.count, err)
		}
		for _, message := range messages {
			held[message.UID] = true
			agrees[message.UID] = hasFlag(message.Flags, serverFlag) == set
		}
	}
	// The caller's own UID strings are answered, so it can match them back however it stored them;
	// parseUIDs keeps their order, which uidChunks has already proved parse.
	nums, err := parseUIDs(uids)
	if err != nil {
		return nil, err
	}
	settled := make([]string, 0, len(nums))
	for i, num := range nums {
		if !held[num] || agrees[num] {
			settled = append(settled, uids[i])
		}
	}
	return settled, nil
}

// hasFlag reports whether flags holds want.
func hasFlag(flags []imap.Flag, want imap.Flag) bool {
	for _, flag := range flags {
		if flag == want {
			return true
		}
	}
	return false
}
