package ics

import (
	"strings"
	"testing"
)

func TestDescriptionTextPlainUnchanged(t *testing.T) {
	in := "Quarterly review with the team.\nBring the numbers."
	if got := descriptionText(in); got != in {
		t.Errorf("plain description changed:\n%q", got)
	}
}

func TestDescriptionTextStripsAnchorsKeepingUrls(t *testing.T) {
	in := "Join: https://teams.microsoft.com/l/meetup-join/abc\n" +
		"Can't join?\n" +
		`<a href="https://abtrace.workable.com/events/xyz?rsvp_intent=reschedule">Reschedule</a>` + "\n" +
		`<a href="https://abtrace.workable.com/events/xyz?rsvp_intent=decline">Cancel</a>`
	got := descriptionText(in)
	if strings.Contains(got, "<a ") || strings.Contains(got, "</a>") {
		t.Errorf("anchor tags survived:\n%q", got)
	}
	for _, want := range []string{
		"https://teams.microsoft.com/l/meetup-join/abc",
		"Reschedule (https://abtrace.workable.com/events/xyz?rsvp_intent=reschedule)",
		"Cancel (https://abtrace.workable.com/events/xyz?rsvp_intent=decline)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("result missing %q:\n%q", want, got)
		}
	}
}

func TestDescriptionTextBreaksAndEntities(t *testing.T) {
	if got := descriptionText("Line one<br>Line two"); got != "Line one\nLine two" {
		t.Errorf("br not converted: %q", got)
	}
	if got := descriptionText("Tom &amp; Jerry <b>show</b>"); got != "Tom & Jerry show" {
		t.Errorf("entities or tags not handled: %q", got)
	}
}

func TestDescriptionTextAnchorLabelIsUrl(t *testing.T) {
	// When the visible label already is the URL, it is not duplicated as "url (url)".
	in := `<a href="https://example.com/x">https://example.com/x</a>`
	if got := descriptionText(in); got != "https://example.com/x" {
		t.Errorf("duplicated url: %q", got)
	}
}
