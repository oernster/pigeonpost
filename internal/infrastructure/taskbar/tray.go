package taskbar

import "fmt"

// balloonTitleSingle is the title shown on a tray balloon when a single reminder is due.
const balloonTitleSingle = "Reminder"

// BalloonText builds the title and body for a tray notification balloon from the summaries of the
// reminders that fired together in one batch. An empty batch yields empty strings, meaning no balloon.
// A single reminder shows its summary as the body; several show the first summary and a count of the
// rest, so the balloon stays within the small space the shell gives it.
func BalloonText(summaries []string) (title, body string) {
	switch len(summaries) {
	case 0:
		return "", ""
	case 1:
		return balloonTitleSingle, summaries[0]
	default:
		return fmt.Sprintf("%d reminders due", len(summaries)),
			fmt.Sprintf("%s and %d more", summaries[0], len(summaries)-1)
	}
}

// MailSummary is one newly arrived message reduced to the fields a notification shows.
type MailSummary struct {
	Subject string
	Sender  string
}

// MailBalloonText builds the title and body for a new-mail notification from a batch of newly arrived
// messages. An empty batch yields empty strings, meaning no notification. A single message shows its
// subject and sender; several show a count and the most recent, so the balloon stays compact.
func MailBalloonText(messages []MailSummary) (title, body string) {
	switch len(messages) {
	case 0:
		return "", ""
	case 1:
		return "New message", mailLine(messages[0])
	default:
		return fmt.Sprintf("%d new messages", len(messages)),
			fmt.Sprintf("%s and %d more", mailLine(messages[0]), len(messages)-1)
	}
}

// mailLine renders one message as "subject from sender", filling in placeholders for empty fields.
func mailLine(m MailSummary) string {
	subject := m.Subject
	if subject == "" {
		subject = "(no subject)"
	}
	sender := m.Sender
	if sender == "" {
		sender = "unknown sender"
	}
	return fmt.Sprintf("%s from %s", subject, sender)
}

// TrayActions holds the callbacks a tray context menu invokes. They are supplied by the composition
// root, which owns the Wails runtime, so this package stays free of any UI-framework dependency. On
// Windows the persistent tray icon calls them; off Windows there is no tray menu, so they are never
// invoked, but the type exists so the composition root compiles and wires identically everywhere.
type TrayActions struct {
	Open         func()
	About        func()
	Licence      func()
	CheckUpdates func()
	Quit         func()
}
