package application

import "fmt"

// landedIndexes returns the positions in uids that a batched server call accepted. A call that succeeded
// accepted every one. A call refused part way accepted only the uids it reported a destination for: the
// IMAP adapter sends a large selection in chunks, so an early chunk can land before a later one is
// refused; it returns the landed part together with its error. Discarding that part would report
// every message as failed, return the moved rows to the list at UIDs that no longer exist there and lose
// the undo for what did move. A uid the server moved without saying where cannot be told apart from one it
// refused, so it counts as not landed; the next sync settles its row.
func landedIndexes(uids []string, reported map[string]string, err error) []int {
	landed := make([]int, 0, len(uids))
	for i, uid := range uids {
		if _, ok := reported[uid]; ok || err == nil {
			landed = append(landed, i)
		}
	}
	return landed
}

// landedUIDs is landedIndexes answered as the uids themselves, in batch order.
func landedUIDs(uids []string, reported map[string]string, err error) []string {
	landed := make([]string, 0, len(uids))
	for _, i := range landedIndexes(uids, reported, err) {
		landed = append(landed, uids[i])
	}
	return landed
}

// idsForUIDs maps the wanted uids of a batch (whose uids and ids run in parallel) back to their message
// ids, in batch order.
func idsForUIDs(uids, ids, wanted []string) []string {
	want := make(map[string]struct{}, len(wanted))
	for _, uid := range wanted {
		want[uid] = struct{}{}
	}
	out := make([]string, 0, len(wanted))
	for i, uid := range uids {
		if _, ok := want[uid]; ok {
			out = append(out, ids[i])
		}
	}
	return out
}

// idsOf maps the given uids of a folder batch back to their message ids, in batch order.
func (b folderBatch) idsOf(uids []string) []string {
	return idsForUIDs(b.uids, b.ids, uids)
}

// refusedBatchError describes a folder batch the server refused, naming the count it accepted rather than
// the count requested, so a partly accepted batch never reads as a total failure.
func refusedBatchError(action, folderID string, accepted, requested int, err error) error {
	return fmt.Errorf("%s messages in %q on server: %d of %d accepted: %w", action, folderID, accepted, requested, err)
}
