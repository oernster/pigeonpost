//go:build windows

package installer

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// emlExtension is the file extension PigeonPost registers to open.
	emlExtension = ".eml"
	// emlProgID is PigeonPost's programmatic identifier for a .eml file: the handler Windows launches and
	// the entry the "Open with" picker and its "Always" default button refer to.
	emlProgID = "PigeonPost.eml"
	// emlProgIDDescription is the friendly type name Explorer shows for a .eml handled by PigeonPost.
	emlProgIDDescription = "Email Message"
	// appDescription is shown beside PigeonPost in the Windows Settings "Default apps" list.
	appDescription = "Local-first email, calendar and contacts client."

	// The keys below all live under HKEY_CURRENT_USER, so registering needs no administrator rights.
	classesEmlProgIDPath  = `Software\Classes\` + emlProgID
	classesExtProgidsPath = `Software\Classes\` + emlExtension + `\OpenWithProgids`
	classesAppPath        = `Software\Classes\Applications\` + ExeName
	capabilitiesPath      = `Software\` + AppName + `\Capabilities`
	capabilitiesAssocPath = capabilitiesPath + `\FileAssociations`
	appRootPath           = `Software\` + AppName
	registeredAppsPath    = `Software\RegisteredApplications`
)

// RegisterEmlAssociation registers PigeonPost as a handler for .eml files under the current user, so it
// appears in the Windows "Open with" picker (in the Suggested apps group) and in Settings > Default apps,
// where the user can choose it as the default. It never seizes the default itself: Windows only lets the
// user grant that, through the picker's "Always" button or the Default apps page. exePath is the installed
// executable and iconPath the icon shown for a .eml file (the executable carries the icon).
func RegisterEmlAssociation(exePath, iconPath string) error {
	command := `"` + exePath + `" "%1"`
	icon := `"` + iconPath + `",0`

	// The ProgID: the command to launch plus the icon and type name Explorer shows for a .eml file.
	defaults := map[string]string{
		classesEmlProgIDPath:                         emlProgIDDescription,
		classesEmlProgIDPath + `\DefaultIcon`:        icon,
		classesEmlProgIDPath + `\shell\open\command`: command,
		classesAppPath + `\shell\open\command`:       command,
	}
	for path, value := range defaults {
		if err := setDefaultValue(path, value); err != nil {
			return err
		}
	}

	// Advertise the extension without taking over the .eml class: OpenWithProgids only adds PigeonPost to
	// the candidate handlers; SupportedTypes lists the extension the application opens.
	named := []struct{ path, name, value string }{
		{classesAppPath, "FriendlyAppName", AppName},
		{classesExtProgidsPath, emlProgID, ""},
		{classesAppPath + `\SupportedTypes`, emlExtension, ""},
		// Register as a Default Programs application so PigeonPost is listed in Settings > Default apps and
		// in the picker's Suggested apps group, with .eml mapped to its ProgID.
		{capabilitiesPath, "ApplicationName", AppName},
		{capabilitiesPath, "ApplicationDescription", appDescription},
		{capabilitiesAssocPath, emlExtension, emlProgID},
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

// UnregisterEmlAssociation removes the .eml handler registration written by RegisterEmlAssociation. It is
// best effort per key: a key or value already gone is not an error, so a partial earlier install still
// cleans up. Only the values PigeonPost added to shared keys are removed; the shared keys themselves (the
// .eml class, RegisteredApplications) are left in place.
func UnregisterEmlAssociation() error {
	_ = deleteNamedValue(classesExtProgidsPath, emlProgID)
	_ = deleteNamedValue(registeredAppsPath, AppName)
	for _, path := range []string{classesEmlProgIDPath, classesAppPath, appRootPath} {
		if err := deleteKeyRecursive(registry.CURRENT_USER, path); err != nil {
			return err
		}
	}
	notifyAssociationsChanged()
	return nil
}

// setDefaultValue creates the key at path under HKCU if needed and sets its default (unnamed) value.
func setDefaultValue(path, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create key %q: %w", path, err)
	}
	defer key.Close()
	if err := key.SetStringValue("", value); err != nil {
		return fmt.Errorf("set default of %q: %w", path, err)
	}
	return nil
}

// setNamedValue creates the key at path under HKCU if needed and sets a named string value on it.
func setNamedValue(path, name, value string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create key %q: %w", path, err)
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("set %q of %q: %w", name, path, err)
	}
	return nil
}

// deleteNamedValue removes a single named value from a key, ignoring a missing key or value.
func deleteNamedValue(path, name string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer key.Close()
	_ = key.DeleteValue(name)
	return nil
}

// deleteKeyRecursive deletes a registry key and every subkey beneath it. registry.DeleteKey only removes a
// key that has no subkeys, so the ProgID and Applications trees (which hold shell\open\command below them)
// need this depth-first walk. A key that does not exist is treated as already removed.
func deleteKeyRecursive(root registry.Key, path string) error {
	key, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return nil
	}
	names, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return fmt.Errorf("read subkeys of %q: %w", path, err)
	}
	for _, name := range names {
		if err := deleteKeyRecursive(root, path+`\`+name); err != nil {
			return err
		}
	}
	if err := registry.DeleteKey(root, path); err != nil {
		return fmt.Errorf("delete key %q: %w", path, err)
	}
	return nil
}

var (
	modShell32         = windows.NewLazySystemDLL("shell32.dll")
	procSHChangeNotify = modShell32.NewProc("SHChangeNotify")
)

const (
	// shcneAssocChanged (SHCNE_ASSOCCHANGED) tells the shell a file association changed; shcnfIDList
	// (SHCNF_IDLIST) marks the unused event parameters as item-ID-list form. Together they ask Explorer to
	// reload cached associations so the new .eml handler appears without the user signing out.
	shcneAssocChanged = 0x08000000
	shcnfIDList       = 0x0000
)

// notifyAssociationsChanged asks the shell to reload file associations after they are written or removed, so
// the change is visible at once. It is best effort: the association is already in the registry; the
// refresh is only a convenience.
func notifyAssociationsChanged() {
	_, _, _ = procSHChangeNotify.Call(uintptr(shcneAssocChanged), uintptr(shcnfIDList), 0, 0)
}
