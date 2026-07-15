package domain

import "testing"

func TestMessageIDForJoinsFolderAndUIDWithTheSeparator(t *testing.T) {
	got := MessageIDFor("acc\x1fINBOX", "42")
	want := "acc\x1fINBOX" + IDSeparator + "42"
	if got != want {
		t.Fatalf("MessageIDFor = %q, want %q", got, want)
	}
}
