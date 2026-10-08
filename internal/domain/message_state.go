package domain

import "strings"

// MessageState is what a folder listing reports for one message: its UID with the flags and tag keywords
// it carries now, without its headers. A sync of a folder it has synced before asks for this alone, which
// is small even for tens of thousands of messages, then fetches full summaries only for the UIDs it has
// not cached. Fetching every summary on every sync was measured at up to 40 seconds for 410 messages
// against Outlook.com (2026-10-08): a 54-folder account never finished one.
type MessageState struct {
	uid      string
	flags    Flags
	keywords []string
}

// NewMessageState validates and constructs a listing entry; the UID must not be blank.
func NewMessageState(uid string, flags Flags, keywords []string) (MessageState, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return MessageState{}, ErrInvalidUID
	}
	return MessageState{uid: uid, flags: flags, keywords: append([]string(nil), keywords...)}, nil
}

// UID returns the message's server handle.
func (s MessageState) UID() string { return s.uid }

// Flags returns the flags the server reports on the message now.
func (s MessageState) Flags() Flags { return s.flags }

// Keywords returns a copy of the tag keywords the server reports on the message now.
func (s MessageState) Keywords() []string { return append([]string(nil), s.keywords...) }

// NewUIDs answers the listed UIDs no cached summary holds, in listing order: the messages whose
// summaries must still be fetched.
func NewUIDs(cached []MessageSummary, listing []MessageState) []string {
	held := uidSet(cached)
	var fresh []string
	for _, entry := range listing {
		if !held[entry.uid] {
			fresh = append(fresh, entry.uid)
		}
	}
	return fresh
}

// MergeListing builds a folder's current summaries from what is cached, what the listing reports and the
// summaries just fetched for new UIDs. Every listed message appears once, in listing order: a fetched
// summary where there is one; otherwise the cached summary carrying the listing's flags and keywords,
// since those change on the server without the headers changing. A cached message the listing no longer
// reports is gone from the server, so it is left out. A listed message with neither (it vanished between
// the listing and the fetch) is left out too; the next sync lists it again or not at all.
func MergeListing(cached []MessageSummary, listing []MessageState, fetched []MessageSummary) []MessageSummary {
	byUID := make(map[string]MessageSummary, len(cached))
	for _, m := range cached {
		byUID[m.uid] = m
	}
	fetchedByUID := make(map[string]MessageSummary, len(fetched))
	for _, m := range fetched {
		fetchedByUID[m.uid] = m
	}
	merged := make([]MessageSummary, 0, len(listing))
	for _, entry := range listing {
		if m, ok := fetchedByUID[entry.uid]; ok {
			merged = append(merged, m)
			continue
		}
		if m, ok := byUID[entry.uid]; ok {
			merged = append(merged, m.WithFlags(entry.flags).WithKeywords(entry.keywords))
		}
	}
	return merged
}

// uidSet answers the UIDs the summaries hold.
func uidSet(messages []MessageSummary) map[string]bool {
	set := make(map[string]bool, len(messages))
	for _, m := range messages {
		set[m.uid] = true
	}
	return set
}
