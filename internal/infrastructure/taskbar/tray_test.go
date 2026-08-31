package taskbar

import "testing"

func TestBalloonTextEmptyBatch(t *testing.T) {
	title, body := BalloonText(nil)
	if title != "" || body != "" {
		t.Errorf("empty batch = (%q, %q), want two empty strings", title, body)
	}
}

func TestBalloonTextSingleReminder(t *testing.T) {
	title, body := BalloonText([]string{"Dentist"})
	if title != "Reminder" {
		t.Errorf("title = %q, want %q", title, "Reminder")
	}
	if body != "Dentist" {
		t.Errorf("body = %q, want the reminder summary", body)
	}
}

func TestBalloonTextManyRemindersSummarise(t *testing.T) {
	title, body := BalloonText([]string{"Dentist", "Standup", "Lunch"})
	if title != "3 reminders due" {
		t.Errorf("title = %q, want a count of the batch", title)
	}
	if body != "Dentist and 2 more" {
		t.Errorf("body = %q, want the first summary and a count of the rest", body)
	}
}

func TestMailBalloonTextEmptyBatch(t *testing.T) {
	title, body := MailBalloonText(nil)
	if title != "" || body != "" {
		t.Errorf("empty batch = (%q, %q), want two empty strings", title, body)
	}
}

func TestMailBalloonTextSingleMessage(t *testing.T) {
	title, body := MailBalloonText([]MailSummary{{Subject: "Lunch?", Sender: "Ada"}})
	if title != "New message" {
		t.Errorf("title = %q, want %q", title, "New message")
	}
	if body != "Lunch? from Ada" {
		t.Errorf("body = %q, want the subject and sender", body)
	}
}

func TestMailBalloonTextFillsBlankFields(t *testing.T) {
	_, body := MailBalloonText([]MailSummary{{}})
	if body != "(no subject) from unknown sender" {
		t.Errorf("body = %q, want placeholders for the blank fields", body)
	}
}

func TestMailBalloonTextManyMessagesSummarise(t *testing.T) {
	title, body := MailBalloonText([]MailSummary{
		{Subject: "One", Sender: "A"},
		{Subject: "Two", Sender: "B"},
		{Subject: "Three", Sender: "C"},
	})
	if title != "3 new messages" {
		t.Errorf("title = %q, want a count of the batch", title)
	}
	if body != "One from A and 2 more" {
		t.Errorf("body = %q, want the first message and a remainder count", body)
	}
}

// TestBalloonSuppressedGovernsTheBalloonOnly pins the rule that a focused window withholds. A reminder
// (force false) defers to its in-window banner while the window is in front; every other case shows.
func TestBalloonSuppressedGovernsTheBalloonOnly(t *testing.T) {
	cases := []struct {
		name          string
		force         bool
		windowFocused bool
		want          bool
	}{
		{"a reminder over the focused window is withheld", false, true, true},
		{"a reminder with the window elsewhere shows", false, false, false},
		{"new mail shows over the focused window", true, true, false},
		{"new mail with the window elsewhere shows", true, false, false},
	}
	for _, c := range cases {
		if got := balloonSuppressed(c.force, c.windowFocused); got != c.want {
			t.Errorf("%s: balloonSuppressed(%t, %t) = %t, want %t", c.name, c.force, c.windowFocused, got, c.want)
		}
	}
}
