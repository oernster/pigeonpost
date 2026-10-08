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
// counterpart pushes nothing and settles nothing, without connecting. $Forwarded is a keyword rather
// than a system flag, so it is settled only where the server keeps it, exactly as PushKeyword does. It
// satisfies application.MailActions.
func (s *Source) PushFlag(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, flag domain.Flag, set bool) ([]string, error) {
	serverFlag, ok := serverFlags[flag]
	if !ok {
		return nil, nil
	}
	return s.pushFlag(ctx, account, folder, uids, serverFlag, set, settleableFor(serverFlag, set))
}

// PushKeyword is PushFlag for a tag keyword, the replay of pending tag intents. One difference: a server
// may accept a keyword it cannot keep, holding it for the session only; the read-back on that same
// session would then report it present. Settling on that read-back would clear the intent; the next
// fetch, on a fresh session, would find the keyword gone and the reconcile would delete the user's tag.
// So an added keyword is settled only where the SELECT said the server keeps it (PERMANENTFLAGS lists
// \* or the keyword itself); elsewhere it is pushed and nothing is settled, leaving the intent to guard
// the tag as before. A removal is always safe to settle. It satisfies application.MailActions.
func (s *Source) PushKeyword(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, keyword string, set bool) ([]string, error) {
	serverFlag := imap.Flag(keyword)
	return s.pushFlag(ctx, account, folder, uids, serverFlag, set, settleableFor(serverFlag, set))
}

// settleableFor answers when a push of flag can be settled from its read-back. A system flag (\Seen,
// \Answered, \Flagged) always can: RFC 3501 defines them. A keyword ($Forwarded, a tag) can be held for
// the session only, so an added one is settled only where the SELECT said the server keeps it; a
// removal always can, since a keyword the server could not keep is absent anyway.
func settleableFor(flag imap.Flag, set bool) func(*imap.SelectData) bool {
	if isSystemFlag(flag) || !set {
		return func(*imap.SelectData) bool { return true }
	}
	return func(data *imap.SelectData) bool { return keeps(data.PermanentFlags, flag) }
}

// isSystemFlag reports whether flag is one of IMAP's system flags, which all begin with a backslash; a
// keyword never does.
func isSystemFlag(flag imap.Flag) bool {
	return len(flag) > 0 && flag[0] == '\\'
}

// keeps reports whether a PERMANENTFLAGS list says the server keeps the given keyword. go-imap reads a
// missing PERMANENTFLAGS and an empty one alike, so neither counts as keeping it.
func keeps(permanent []imap.Flag, keyword imap.Flag) bool {
	return hasFlag(permanent, imap.FlagWildcard) || hasFlag(permanent, keyword)
}

// pushFlag is the work PushFlag and PushKeyword share: one connection, the flag stored on every UID in
// chunks, then, where settleable says the server's answer can be trusted, the flags read back to answer
// the settled UIDs. An empty batch connects to nothing.
func (s *Source) pushFlag(ctx context.Context, account domain.Account, folder domain.Folder, uids []string, serverFlag imap.Flag, set bool, settleable func(*imap.SelectData) bool) (settled []string, err error) {
	if len(uids) == 0 {
		return nil, nil
	}
	done := traceStep(account, fmt.Sprintf("push %s", serverFlag), folder.Path())
	defer func() { done(len(uids), err) }()
	chunks, err := uidChunks(uids)
	if err != nil {
		return nil, err
	}
	client, data, err := s.openFolderData(ctx, account, folder)
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
	if !settleable(data) {
		return nil, nil
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
	settled = make([]string, 0, len(nums))
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
