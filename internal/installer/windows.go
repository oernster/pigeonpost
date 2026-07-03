//go:build windows

package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hiddenProcess hides the console window of a child process so that shelling out to PowerShell or
// cmd during install and uninstall does not flash a terminal at the user.
func hiddenProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

const (
	uninstallKeyPath = `Software\Microsoft\Windows\CurrentVersion\Uninstall\PigeonPost`
	runKeyPath       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName     = "PigeonPost"
	shortcutName     = "PigeonPost.lnk"
)

// UninstallInfo carries the values written to the HKCU uninstall registry entry.
type UninstallInfo struct {
	Version      string
	InstallDir   string
	UninstallExe string
	IconPath     string
	EstimatedKB  uint32
}

// WriteUninstallEntry registers PigeonPost under the current user's uninstall list.
func WriteUninstallEntry(info UninstallInfo) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("create uninstall key: %w", err)
	}
	defer key.Close()

	strings := map[string]string{
		"DisplayName":     AppName,
		"DisplayVersion":  info.Version,
		"InstallLocation": info.InstallDir,
		"UninstallString": fmt.Sprintf("\"%s\" -uninstall", info.UninstallExe),
		"DisplayIcon":     info.IconPath,
		"Publisher":       Publisher,
	}
	for name, value := range strings {
		if err := key.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set %q: %w", name, err)
		}
	}
	for name, value := range map[string]uint32{"NoModify": 1, "NoRepair": 1, "EstimatedSize": info.EstimatedKB} {
		if err := key.SetDWordValue(name, value); err != nil {
			return fmt.Errorf("set %q: %w", name, err)
		}
	}
	return nil
}

// RemoveUninstallEntry deletes the uninstall registry entry.
func RemoveUninstallEntry() error {
	if err := registry.DeleteKey(registry.CURRENT_USER, uninstallKeyPath); err != nil {
		return fmt.Errorf("delete uninstall key: %w", err)
	}
	return nil
}

// InstalledVersion returns the currently installed version and whether PigeonPost is installed, by
// reading the uninstall registry entry.
func InstalledVersion() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, uninstallKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetStringValue("DisplayVersion")
	if err != nil {
		return "", false
	}
	return value, true
}

// SetLaunchOnBoot adds or removes the current user's Run entry that starts PigeonPost at login.
func SetLaunchOnBoot(exePath string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()
	if enabled {
		if err := key.SetStringValue(runValueName, "\""+exePath+"\""); err != nil {
			return fmt.Errorf("set run value: %w", err)
		}
		return nil
	}
	_ = key.DeleteValue(runValueName)
	return nil
}

// IsLaunchOnBoot reports whether the login Run entry is present.
func IsLaunchOnBoot() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	_, _, err = key.GetStringValue(runValueName)
	return err == nil
}

// CreateShortcut writes a .lnk file via the Windows Script Host, avoiding direct COM handling.
func CreateShortcut(linkPath, target, iconPath, workDir string) error {
	script := fmt.Sprintf(
		`$s=(New-Object -ComObject WScript.Shell).CreateShortcut(%q);`+
			`$s.TargetPath=%q;$s.IconLocation=%q;$s.WorkingDirectory=%q;$s.Save()`,
		linkPath, target, iconPath, workDir)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = hiddenProcess()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create shortcut %q: %w: %s", linkPath, err, string(out))
	}
	return nil
}

// CreateShortcuts places Start Menu and Desktop shortcuts. It is best effort per location.
func CreateShortcuts(exePath, workDir string) {
	if dir, err := StartMenuProgramsDir(); err == nil {
		_ = CreateShortcut(filepath.Join(dir, shortcutName), exePath, exePath, workDir)
	}
	if dir, err := DesktopDir(); err == nil {
		_ = CreateShortcut(filepath.Join(dir, shortcutName), exePath, exePath, workDir)
	}
}

// RemoveShortcuts deletes the Start Menu and Desktop shortcuts.
func RemoveShortcuts() {
	if dir, err := StartMenuProgramsDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, shortcutName))
	}
	if dir, err := DesktopDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, shortcutName))
	}
}

// DesktopDir returns the current user's Desktop directory.
func DesktopDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, "Desktop"), nil
}

// StartMenuProgramsDir returns the current user's Start Menu Programs directory.
func StartMenuProgramsDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA is not set")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}

// RemoveTree deletes a directory tree, used to remove user data on request.
func RemoveTree(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove %q: %w", dir, err)
	}
	return nil
}

// ScheduleDirDeletion spawns a detached shell that waits briefly, so this process can exit and
// release its own executable, then removes the install directory.
func ScheduleDirDeletion(dir string) {
	line := fmt.Sprintf(`ping 127.0.0.1 -n 3 >nul & rmdir /s /q "%s"`, dir)
	cmd := exec.Command("cmd", "/C", line)
	cmd.SysProcAttr = hiddenProcess()
	_ = cmd.Start()
}
