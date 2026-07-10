//go:build windows

package installer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ErrAppRunning indicates PigeonPost is running, so an install or uninstall that would overwrite or
// remove the running executable must not proceed until the user closes it.
//
//lint:ignore ST1005 user-facing installer dialog text shown verbatim; the sentence punctuation is intentional
var ErrAppRunning = errors.New("PigeonPost is running. Please close it and try again.")

// ErrAppStillRunning indicates PigeonPost was asked to close but was still running after the wait, so
// setup should not proceed.
//
//lint:ignore ST1005 user-facing installer dialog text shown verbatim; the sentence punctuation is intentional
var ErrAppStillRunning = errors.New("PigeonPost could not be closed. Please close it manually and try again.")

const (
	// terminateWaitTimeout bounds how long CloseRunningApp waits for the app to exit after terminating
	// it, so a stuck process cannot hang setup indefinitely.
	terminateWaitTimeout = 5 * time.Second
	// terminatePollStep is how often CloseRunningApp rechecks whether the app has exited.
	terminatePollStep = 100 * time.Millisecond
	// forcedExitCode is the exit code reported for a process ended by the installer.
	forcedExitCode = 1
)

// hiddenProcess hides the console window of a child process so that shelling out to PowerShell or
// cmd during install and uninstall does not flash a terminal at the user.
func hiddenProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// IsAppRunning reports whether a PigeonPost.exe process is currently running, so an install or
// uninstall can refuse to overwrite or remove the executable while it is in use rather than silently
// corrupting a running instance.
func IsAppRunning() bool {
	return isProcessRunning(ExeName)
}

// isProcessRunning reports whether any running process has the given executable name.
func isProcessRunning(exeName string) bool {
	return len(processIDs(exeName)) > 0
}

// processIDs returns the ids of every running process whose executable name matches, compared
// case-insensitively. Any failure taking or walking the process snapshot yields no ids, so a snapshot
// error never blocks a legitimate install or uninstall.
func processIDs(exeName string) []uint32 {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil
	}
	target := strings.ToLower(exeName)
	var pids []uint32
	for {
		if strings.ToLower(windows.UTF16ToString(entry.ExeFile[:])) == target {
			pids = append(pids, entry.ProcessID)
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			return pids
		}
	}
}

// CloseRunningApp ends every running PigeonPost.exe and waits for the executable lock to release, so the
// installer can overwrite or remove it. It backs the setup program's offer to close a running instance.
// Termination is forced rather than a polite window close because the app intercepts a normal close (it
// offers to minimise to the tray), so only ending the process reliably frees the file. A process that
// has already exited or cannot be opened is skipped; if the app is still present after the wait,
// ErrAppStillRunning is returned so setup does not proceed onto a locked file.
func CloseRunningApp() error {
	for _, pid := range processIDs(ExeName) {
		terminateProcess(pid)
	}
	deadline := time.Now().Add(terminateWaitTimeout)
	for IsAppRunning() {
		if time.Now().After(deadline) {
			return ErrAppStillRunning
		}
		time.Sleep(terminatePollStep)
	}
	return nil
}

// terminateProcess forcibly ends the process with the given id. Any failure to open or terminate it is
// ignored: a process that has already exited, or that this user cannot open, needs no further action.
func terminateProcess(pid uint32) {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_ = windows.TerminateProcess(handle, forcedExitCode)
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
