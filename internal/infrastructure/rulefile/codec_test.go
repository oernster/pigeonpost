package rulefile

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/oernster/pigeonpost/internal/application"
)

func sampleRules() []application.RuleTransfer {
	return []application.RuleTransfer{{
		Name: "Music", Enabled: true, MatchMode: "any", StopProcessing: true,
		Accounts: []string{"me@example.com"},
		Conditions: []application.RuleTransferCondition{
			{Field: "all", Operator: "contains", Text: "7digital"},
			{Field: "all", Operator: "contains", Text: "mediamonkey", Not: true, MatchCase: true},
		},
		Actions: []application.RuleTransferAction{
			{Kind: "moveTo", Account: "me@example.com", Folder: "Shopping.Music"},
		},
	}}
}

func TestCodecRoundTripsEveryField(t *testing.T) {
	data, err := New().Encode(sampleRules())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	got, err := New().Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := sampleRules()
	if len(got) != 1 {
		t.Fatalf("decoded %d rules, want 1", len(got))
	}
	if got[0].Name != want[0].Name || got[0].MatchMode != want[0].MatchMode ||
		!got[0].StopProcessing || !got[0].Enabled {
		t.Errorf("rule fields did not round trip: %+v", got[0])
	}
	if len(got[0].Conditions) != 2 || !got[0].Conditions[1].Not || !got[0].Conditions[1].MatchCase {
		t.Errorf("conditions did not round trip: %+v", got[0].Conditions)
	}
	if len(got[0].Actions) != 1 || got[0].Actions[0].Folder != "Shopping.Music" {
		t.Errorf("actions did not round trip: %+v", got[0].Actions)
	}
	if len(got[0].Accounts) != 1 || got[0].Accounts[0] != "me@example.com" {
		t.Errorf("scope did not round trip: %v", got[0].Accounts)
	}
}

// The file is a document someone may open and read, so it is indented, ends with a newline and names
// itself in its first fields.
func TestEncodedFileIsReadable(t *testing.T) {
	data, err := New().Encode(sampleRules())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	text := string(data)

	if !strings.HasSuffix(text, "}\n") {
		t.Error("the file does not end with a newline")
	}
	if !strings.Contains(text, "\n  \"kind\": \"pigeonpost.rules\"") {
		t.Errorf("the file does not name itself on its own indented line:\n%s", text)
	}
	// A destination is written as an account and a path, so the control character that joins them into
	// a folder id never reaches the file.
	if strings.ContainsRune(text, '\x1f') {
		t.Error("a folder id reached the file rather than its two halves")
	}
}

func TestEncodeGivesAnEmptySetAnEmptyRuleList(t *testing.T) {
	data, err := New().Encode(nil)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(data), "\"rules\": []") {
		t.Errorf("an empty export wrote %s", data)
	}
}

// An unscoped rule covers every account, which the file states as an empty list rather than as null.
// The distinction is not cosmetic: a hand-editing reader meeting "accounts": null has to know that
// Go's zero slice and an empty one mean the same thing here, while an empty list says it outright.
func TestEncodeWritesAnUnscopedRuleAsAnEmptyAccountList(t *testing.T) {
	data, err := New().Encode([]application.RuleTransfer{{Name: "Everywhere", Accounts: nil}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(data), "\"accounts\": []") {
		t.Errorf("an unscoped rule wrote %s", data)
	}
}

// Refusing a file that is not ours matters more than it looks: decoding it as an empty rule set would
// report "0 rules" about a file the user believed held theirs, which is a wrong answer told quietly.
func TestDecodeRefusesWhatIsNotARulesFile(t *testing.T) {
	cases := []struct {
		name string
		data string
		want error
	}{
		{"another JSON document", `{"kind":"pigeonpost.contacts","version":1}`, ErrNotARulesFile},
		{"no kind at all", `{"version":1,"rules":[]}`, ErrNotARulesFile},
		{"a later format", `{"kind":"pigeonpost.rules","version":99,"rules":[]}`, ErrFutureVersion},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := New().Decode([]byte(c.data)); !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestDecodeReportsMalformedJSON(t *testing.T) {
	if _, err := New().Decode([]byte("not json at all")); err == nil {
		t.Fatal("expected malformed JSON to be refused")
	}
}

// An older file is read by a newer reader: the version gate is one-sided on purpose.
func TestDecodeAcceptsItsOwnVersion(t *testing.T) {
	var doc map[string]any
	data, err := New().Encode(sampleRules())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["version"] != float64(version) {
		t.Fatalf("file version is %v, want %d", doc["version"], version)
	}
	if _, err := New().Decode(data); err != nil {
		t.Fatalf("a file of the current version was refused: %v", err)
	}
}
