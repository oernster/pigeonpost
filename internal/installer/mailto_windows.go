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

	// The legacy default-mail-client list predates per-user URL associations and is still consulted
	// by older launchers (Qt's Windows mail helper among them), which otherwise resurrect whatever
	// stale entry lingers there (typically an old Outlook install).
	clientsMailPath    = `Software\Clients\Mail`
	clientsMailAppPath = clientsMailPath + `\` + AppName
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
		// Legacy mail-client registration, mirroring what Thunderbird writes, so old-style
		// launchers can resolve PigeonPost. Presence only: the legacy default itself is claimed
		// separately on an explicit user request (ClaimLegacyDefaultMailClient).
		clientsMailAppPath:                                          AppName,
		clientsMailAppPath + `\shell\open\command`:                  `"` + exePath + `"`,
		clientsMailAppPath + `\DefaultIcon`:                         icon,
		clientsMailAppPath + `\Protocols\mailto`:                    mailtoProgIDDescription,
		clientsMailAppPath + `\Protocols\mailto\DefaultIcon`:        icon,
		clientsMailAppPath + `\Protocols\mailto\shell\open\command`: command,
	}
	for path, value := range defaults {
		if err := setDefaultValue(path, value); err != nil {
			return err
		}
	}

	named := []struct{ path, name, value string }{
		// The empty "URL Protocol" value marks the ProgID as a URL handler rather than a file type.
		{classesMailtoProgIDPath, "URL Protocol", ""},
		{clientsMailAppPath + `\Protocols\mailto`, "URL Protocol", ""},
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

// ClaimLegacyDefaultMailClient sets the per-user LEGACY default mail client to PigeonPost by writing the
// default value of HKCU Software\Clients\Mail. Unlike the modern MAILTO UserChoice (hash-protected, only
// the Settings app may write it), this key is open to applications and is what Thunderbird's own "make
// default" writes. Only called on an explicit user request, never from plain installation.
func ClaimLegacyDefaultMailClient() error {
	return setDefaultValue(clientsMailPath, AppName)
}

// UnregisterMailtoProtocol removes the mailto: registration written by RegisterMailtoProtocol, releasing
// the legacy default first when PigeonPost holds it. Best effort in the same way as the .eml cleanup: a
// key or value already gone is not an error. The shared application keys are left to
// UnregisterEmlAssociation, which removes the whole PigeonPost application root.
func UnregisterMailtoProtocol() error {
	releaseLegacyDefaultMailClient()
	_ = deleteNamedValue(capabilitiesURLPath, mailtoScheme)
	for _, path := range []string{classesMailtoProgIDPath, clientsMailAppPath} {
		if err := deleteKeyRecursive(registry.CURRENT_USER, path); err != nil {
			return err
		}
	}
	notifyAssociationsChanged()
	return nil
}

// releaseLegacyDefaultMailClient clears the legacy per-user default mail client if PigeonPost holds it,
// leaving another client's claim untouched.
func releaseLegacyDefaultMailClient() {
	key, err := registry.OpenKey(
		registry.CURRENT_USER, clientsMailPath, registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		return
	}
	defer key.Close()
	if current, _, err := key.GetStringValue(""); err == nil && current == AppName {
		_ = key.DeleteValue("")
	}
}
