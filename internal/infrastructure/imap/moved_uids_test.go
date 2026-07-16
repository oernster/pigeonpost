package imap

import (
	"reflect"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

func uidSet(nums ...imap.UID) imap.UIDSet {
	set := imap.UIDSet{}
	for _, n := range nums {
		set.AddNum(n)
	}
	return set
}

func TestMovedUIDsPairsSourceAndDestinationInOrder(t *testing.T) {
	data := &imapclient.MoveData{
		UIDValidity: 1,
		SourceUIDs:  uidSet(10, 20),
		DestUIDs:    uidSet(100, 200),
	}
	got := movedUIDs(data)
	want := map[string]string{"10": "100", "20": "200"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("movedUIDs = %v, want %v", got, want)
	}
}

func TestMovedUIDsNilDataMeansUnknown(t *testing.T) {
	if got := movedUIDs(nil); got != nil {
		t.Errorf("movedUIDs(nil) = %v, want nil", got)
	}
}

func TestMovedUIDsMissingSetsMeanUnknown(t *testing.T) {
	// A server without UIDPLUS sends no COPYUID, so the sets are absent (nil NumSet interfaces).
	if got := movedUIDs(&imapclient.MoveData{}); got != nil {
		t.Errorf("movedUIDs with absent sets = %v, want nil", got)
	}
}

func TestMovedUIDsUnbalancedSetsMeanUnknown(t *testing.T) {
	data := &imapclient.MoveData{
		SourceUIDs: uidSet(10, 20),
		DestUIDs:   uidSet(100),
	}
	if got := movedUIDs(data); got != nil {
		t.Errorf("movedUIDs with unbalanced sets = %v, want nil", got)
	}
}

func TestMovedUIDsEmptySetsMeanUnknown(t *testing.T) {
	data := &imapclient.MoveData{
		SourceUIDs: imap.UIDSet{},
		DestUIDs:   imap.UIDSet{},
	}
	if got := movedUIDs(data); got != nil {
		t.Errorf("movedUIDs with empty sets = %v, want nil", got)
	}
}

func TestMovedUIDsUnboundedRangeMeansUnknown(t *testing.T) {
	// A set holding a star range (n:*) cannot be enumerated, so no pairing is possible.
	open := imap.UIDSet{}
	open.AddRange(10, 0)
	data := &imapclient.MoveData{
		SourceUIDs: open,
		DestUIDs:   uidSet(100),
	}
	if got := movedUIDs(data); got != nil {
		t.Errorf("movedUIDs with an unbounded range = %v, want nil", got)
	}
}

func TestCopiedUIDsPairsSourceAndDestinationInOrder(t *testing.T) {
	data := &imap.CopyData{
		UIDValidity: 1,
		SourceUIDs:  uidSet(10, 20),
		DestUIDs:    uidSet(100, 200),
	}
	got := copiedUIDs(data)
	want := map[string]string{"10": "100", "20": "200"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("copiedUIDs = %v, want %v", got, want)
	}
}

func TestCopiedUIDsNilDataMeansUnknown(t *testing.T) {
	if got := copiedUIDs(nil); got != nil {
		t.Errorf("copiedUIDs(nil) = %v, want nil", got)
	}
}

func TestCopiedUIDsEmptySetsMeanUnknown(t *testing.T) {
	// A server without UIDPLUS sends no COPYUID, leaving the sets empty.
	if got := copiedUIDs(&imap.CopyData{}); got != nil {
		t.Errorf("copiedUIDs with empty sets = %v, want nil", got)
	}
}
