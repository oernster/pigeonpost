package domain

import "strings"

// ContactGroup is a named set of contacts (a mailing list in Thunderbird and Outlook terms). It holds
// the ids of its members and is immutable once constructed; membership changes return copies.
type ContactGroup struct {
	id      string
	name    string
	members []string
}

// NewContactGroup validates and constructs a group. The id and name must be non-empty; blank and
// duplicate member ids are dropped, preserving first-seen order.
func NewContactGroup(id, name string, members []string) (ContactGroup, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return ContactGroup{}, ErrEmptyContactGroupID
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ContactGroup{}, ErrEmptyContactGroupName
	}
	return ContactGroup{id: id, name: name, members: dedupeMembers(members)}, nil
}

// ID returns the group identifier.
func (g ContactGroup) ID() string { return g.id }

// Name returns the group name.
func (g ContactGroup) Name() string { return g.name }

// Members returns a copy of the member contact ids, so a caller cannot mutate the group's state.
func (g ContactGroup) Members() []string {
	if len(g.members) == 0 {
		return nil
	}
	return append([]string(nil), g.members...)
}

// Contains reports whether the given contact id is a member of the group.
func (g ContactGroup) Contains(contactID string) bool {
	for _, m := range g.members {
		if m == contactID {
			return true
		}
	}
	return false
}

// WithMember returns a copy of the group with the contact id added. A blank id or one that is already a
// member returns the group unchanged.
func (g ContactGroup) WithMember(contactID string) ContactGroup {
	contactID = strings.TrimSpace(contactID)
	if contactID == "" || g.Contains(contactID) {
		return g
	}
	next := append(append([]string(nil), g.members...), contactID)
	return ContactGroup{id: g.id, name: g.name, members: next}
}

// WithoutMember returns a copy of the group with the contact id removed. An id that is not a member
// returns the group unchanged.
func (g ContactGroup) WithoutMember(contactID string) ContactGroup {
	if !g.Contains(contactID) {
		return g
	}
	next := make([]string, 0, len(g.members))
	for _, m := range g.members {
		if m != contactID {
			next = append(next, m)
		}
	}
	return ContactGroup{id: g.id, name: g.name, members: next}
}

// dedupeMembers trims, drops blanks and removes duplicate member ids, keeping first-seen order.
func dedupeMembers(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, m := range in {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}
