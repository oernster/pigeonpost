package main

import (
	"errors"
	"reflect"
	"testing"
)

// TestBulkResultCountsWhatTheServerAccepted pins the facade half of a partly refused batch: the ids the
// application reports as landed reach the front end with their undo ids; only the rest are counted as
// failed, so 1 of 2 failing never reads as 2 of 2.
func TestBulkResultCountsWhatTheServerAccepted(t *testing.T) {
	newIDs := map[string]string{"a": "dest-a"}
	result := (&App{}).bulkResult([]string{"a", "b"}, []string{"a"}, newIDs, errors.New("refused"))
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1: one of the two was accepted", result.Failed)
	}
	if !reflect.DeepEqual(result.Ids, []string{"a"}) || !reflect.DeepEqual(result.NewIds, newIDs) {
		t.Errorf("Ids = %v NewIds = %v, want the accepted id with its undo id", result.Ids, result.NewIds)
	}
	if result.Error == "" {
		t.Error("the refusal must still reach the front end")
	}
}
