package domain

import (
	"sort"
	"strings"
)

// Thread is a conversation: a group of messages that share a subject once reply and forward prefixes are
// removed. Its messages are ordered oldest first, so a reader follows the conversation top to bottom. It
// is immutable once constructed.
type Thread struct {
	subject  string
	messages []MessageSummary
}

// Subject returns the display subject of the conversation, taken from its most recent message so a
// subject the sender changed mid-thread still reads sensibly.
func (t Thread) Subject() string { return t.subject }

// Messages returns a copy of the conversation's messages, oldest first.
func (t Thread) Messages() []MessageSummary {
	return append([]MessageSummary(nil), t.messages...)
}

// Count returns how many messages the conversation holds.
func (t Thread) Count() int { return len(t.messages) }

// Latest returns the most recent message in the conversation, which the list shows as the thread's row.
func (t Thread) Latest() MessageSummary { return t.messages[len(t.messages)-1] }

// UnreadCount returns how many messages in the conversation are unread.
func (t Thread) UnreadCount() int {
	unread := 0
	for _, m := range t.messages {
		if !m.IsRead() {
			unread++
		}
	}
	return unread
}

// replyPrefixRe-free normalisation: replyPrefixes are the leading tokens (case-insensitive) that mark a
// reply or forward and are stripped when grouping, so "Re: Fwd: Lunch" threads with "Lunch".
var replyPrefixes = []string{"re", "fwd", "fw", "aw", "sv", "vs", "antw"}

// GroupThreads groups message summaries into conversations by their subject with reply and forward
// prefixes removed, comparing case-insensitively. Within a thread messages are ordered oldest first, and
// the threads themselves are ordered by their most recent message, newest first, so the busiest recent
// conversation sits at the top. Ordering is stable for equal dates, preserving the input order.
func GroupThreads(messages []MessageSummary) []Thread {
	order := make([]string, 0)
	groups := make(map[string][]MessageSummary)
	for _, m := range messages {
		key := normaliseSubject(m.Subject())
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], m)
	}

	threads := make([]Thread, 0, len(order))
	for _, key := range order {
		group := groups[key]
		sort.SliceStable(group, func(i, j int) bool {
			return group[i].Date().Before(group[j].Date())
		})
		threads = append(threads, Thread{subject: group[len(group)-1].Subject(), messages: group})
	}
	sort.SliceStable(threads, func(i, j int) bool {
		return threads[i].Latest().Date().After(threads[j].Latest().Date())
	})
	return threads
}

// normaliseSubject lowercases a subject and strips every leading reply or forward prefix (with an
// optional bracketed count such as "Re[2]:"), then collapses internal whitespace, so subjects that
// differ only by those prefixes and spacing thread together. An all-prefix or empty subject normalises
// to an empty string, grouping blank-subject messages as one conversation.
func normaliseSubject(subject string) string {
	s := strings.TrimSpace(subject)
	for {
		stripped := stripOneReplyPrefix(s)
		if stripped == s {
			break
		}
		s = strings.TrimSpace(stripped)
	}
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// stripOneReplyPrefix removes a single leading reply or forward prefix (e.g. "Re:", "FWD:", "Re[2]:")
// from the front of a subject, or returns it unchanged when none is present.
func stripOneReplyPrefix(s string) string {
	for _, prefix := range replyPrefixes {
		rest, ok := matchReplyPrefix(s, prefix)
		if ok {
			return rest
		}
	}
	return s
}

// matchReplyPrefix reports whether s begins with the given prefix as a reply marker, that is the prefix
// (case-insensitive) followed by an optional "[n]" or "(n)" count and then a colon, and returns the text
// after the colon when it does.
func matchReplyPrefix(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	rest := s[len(prefix):]
	rest = skipReplyCount(rest)
	if strings.HasPrefix(rest, ":") {
		return rest[1:], true
	}
	return "", false
}

// skipReplyCount drops a leading "[n]" or "(n)" count that some clients add to a reply prefix (Re[2]:),
// returning the remainder unchanged when there is none.
func skipReplyCount(s string) string {
	if s == "" {
		return s
	}
	open, close := byte('['), byte(']')
	if s[0] == '(' {
		open, close = '(', ')'
	}
	if s[0] != open {
		return s
	}
	end := strings.IndexByte(s, close)
	if end < 0 {
		return s
	}
	for _, r := range s[1:end] {
		if r < '0' || r > '9' {
			return s
		}
	}
	return s[end+1:]
}
