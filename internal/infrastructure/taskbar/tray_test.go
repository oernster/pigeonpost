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
