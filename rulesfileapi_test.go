package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rules-file DTOs travel to a front end whose generated types declare arrays. A Go nil slice
// encodes as JSON null and reading a length off null throws during render, which with no error
// boundary above the app takes the whole window down rather than one dialog. Every list on these two
// DTOs therefore goes through stringsOrEmpty. This asserts on the encoder's own output rather than on
// a hand-written fixture, since only the real output shows what the front end will receive.
func TestRuleImportDTOsSendArraysNotNull(t *testing.T) {
	plan := RuleImportPlanDTO{
		Path: "C:/tmp/rules.json", File: "rules.json",
		Add:         stringsOrEmpty(nil),
		Replace:     stringsOrEmpty(nil),
		Disable:     stringsOrEmpty(nil),
		Destructive: stringsOrEmpty(nil),
	}
	result := RuleImportResultDTO{Disabled: stringsOrEmpty(nil)}

	for _, dto := range []any{plan, result} {
		encoded, err := json.Marshal(dto)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(encoded), ":null") {
			t.Errorf("a list reached the wire as null, which the front end reads as a crash: %s", encoded)
		}
	}
}

// stringsOrEmpty leaves a list that has entries exactly as it is: it exists to replace nothing with an
// empty list, not to touch the answer.
func TestStringsOrEmptyKeepsTheNamesItIsGiven(t *testing.T) {
	got := stringsOrEmpty([]string{"News", "Receipts"})
	if len(got) != 2 || got[0] != "News" || got[1] != "Receipts" {
		t.Errorf("names were changed on the way to the wire: %v", got)
	}
}
