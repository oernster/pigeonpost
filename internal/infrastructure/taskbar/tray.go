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
