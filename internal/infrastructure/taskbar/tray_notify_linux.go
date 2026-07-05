//go:build linux

package taskbar

import "github.com/godbus/dbus/v5"

const (
	notifyDest           = "org.freedesktop.Notifications"
	notifyPath           = "/org/freedesktop/Notifications"
	notifyMethod         = "org.freedesktop.Notifications.Notify"
	notifyDefaultTimeout = -1 // let the notification server choose how long to show it
)

// Notify raises a desktop notification through the freedesktop D-Bus notification service, the standard
// mechanism on Linux desktops. Any failure to reach the bus is ignored: a missing notification must
// never disturb the reminder scheduler. The session bus connection is shared and cached by the library,
// so it is not closed here.
func (t *Tray) Notify(title, body string) {
	if title == "" && body == "" {
		return
	}
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	obj := conn.Object(notifyDest, dbus.ObjectPath(notifyPath))
	obj.Call(
		notifyMethod, 0,
		t.appName,                   // app_name
		uint32(0),                   // replaces_id
		"",                          // app_icon
		title,                       // summary
		body,                        // body
		[]string{},                  // actions
		map[string]dbus.Variant{},   // hints
		int32(notifyDefaultTimeout), // expire_timeout
	)
}
