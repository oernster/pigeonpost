package taskbar

import (
	"os"
	"strings"
	"testing"
)

// The chime must be sounded by Notify, never from inside the balloon it accompanies. This is a source
// scan rather than a behavioural test because the Windows tray is Win32-coupled and documented as
// excluded from unit tests; what can be held here is where the call sits.
//
// It exists because the two were one call for a release: sound.Play lived in showBalloon, so the rule
// that withholds a balloon over a focused window silenced the chime with it. A reminder falling due
// while the user was looking at the window then arrived with no noise at all. Nothing in the window
// makes a sound, so there was no audible duplicate for that rule to defer to. The coupling was the
// defect; only its placement can be asserted from here.
const (
	trayWindowsSource = "tray_windows.go"
	notifyFunc        = "func (t *Tray) Notify("
	showBalloonFunc   = "func (t *Tray) showBalloon("
	playCall          = "sound.Play("
)

func TestChimeIsSoundedByNotifyAndNotByTheBalloon(t *testing.T) {
	source, err := os.ReadFile(trayWindowsSource)
	if err != nil {
		t.Fatalf("read %s: %v", trayWindowsSource, err)
	}
	body := string(source)
	if strings.Count(body, playCall) != 1 {
		t.Fatalf("%s calls %s %d times, want exactly 1", trayWindowsSource, playCall, strings.Count(body, playCall))
	}
	notify, showBalloon := funcBody(t, body, notifyFunc), funcBody(t, body, showBalloonFunc)
	if !strings.Contains(notify, playCall) {
		t.Error("Notify does not sound the chime: an announcement withheld from the balloon would be silent")
	}
	if strings.Contains(showBalloon, playCall) {
		t.Error("showBalloon sounds the chime: that ties the sound to a balloon the focus rule can withhold")
	}
}

// funcBody returns the source between a function's signature and the closing brace in the first column
// that ends it, which is where gofmt puts every top-level function's terminator.
func funcBody(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	if start < 0 {
		t.Fatalf("%s does not declare %s", trayWindowsSource, signature)
	}
	rest := source[start:]
	end := strings.Index(rest, "\n}\n")
	if end < 0 {
		t.Fatalf("%s: no terminator found for %s", trayWindowsSource, signature)
	}
	return rest[:end]
}
