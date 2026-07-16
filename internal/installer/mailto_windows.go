//go:build windows

package installer

import "golang.org/x/sys/windows/registry"

const (
	// mailtoScheme is the URL scheme PigeonPost registers as a handler for.
	mailtoScheme = "mailto"
	// mailtoProgID is PigeonPost's programmatic identifier for a mailto: link: the handler Windows
	// launches and the entry the Settings "Default apps" MAILTO picker refers to.
	mailtoProgID = AppName + ".mailto"
	// mailtoProgIDDescription follows the Windows convention for protocol handlers ("URL:<scheme> Protocol").
	mailtoProgIDDescription = "URL:MailTo Protocol"

	classesMailtoProgIDPath = `Software\Classes\` + mailtoProgID
	capabilitiesURLPath     = capabilitiesPath + `\URLAssociations`
)

// RegisterMailtoProtocol registers PigeonPost as a mailto: handler under the current user, so it appears
// in Settings > Default apps for the MAILTO link type and can be chosen as the default email client. Like
// the .eml association it never seizes the default itself: Windows only lets the user grant that. It also
// writes the shared application capability keys, so calling it in-app (self-registration without a
// reinstall) is enough on its own. exePath is the installed executable, launched as `"exe" "%1"` with the
// full mailto URI as the argument; iconPath is the icon shown beside the handler.
func RegisterMailtoProtocol(exePath, iconPath string) error {
	command := `"` + exePath + `" "%1"`
	icon := `"` + iconPath + `",0`

	defaults := map[string]string{
		classesMailtoProgIDPath:                         mailtoProgIDDescription,
		classesMailtoProgIDPath + `\DefaultIcon`:        icon,
		classesMailtoProgIDPath + `\shell\open\command`: command,
	}
	for path, value := range defaults {
		if err := setDefaultValue(path, value); err != nil {
			return err
		}
	}

	named := []struct{ path, name, value string }{
		// The empty "URL Protocol" value marks the ProgID as a URL handler rather than a file type.
		{classesMailtoProgIDPath, "URL Protocol", ""},
		{capabilitiesPath, "ApplicationName", AppName},
		{capabilitiesPath, "ApplicationDescription", appDescription},
		{capabilitiesURLPath, mailtoScheme, mailtoProgID},
		{registeredAppsPath, AppName, capabilitiesPath},
	}
	for _, entry := range named {
		if err := setNamedValue(entry.path, entry.name, entry.value); err != nil {
			return err
		}
	}

	notifyAssociationsChanged()
	return nil
}

// UnregisterMailtoProtocol removes the mailto: registration written by RegisterMailtoProtocol. Best effort
// in the same way as the .eml cleanup: a key or value already gone is not an error. The shared application
// keys are left to UnregisterEmlAssociation, which removes the whole PigeonPost application root.
func UnregisterMailtoProtocol() error {
	_ = deleteNamedValue(capabilitiesURLPath, mailtoScheme)
	if err := deleteKeyRecursive(registry.CURRENT_USER, classesMailtoProgIDPath); err != nil {
		return err
	}
	notifyAssociationsChanged()
	return nil
}
