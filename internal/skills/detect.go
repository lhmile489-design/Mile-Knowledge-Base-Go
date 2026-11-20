package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Detection statuses (FR-SKM-DETECT).
const (
	StatusDetected = "detected"
	StatusLeftover = "leftover"
	StatusMissing  = "missing"
)

// UninstallEntry is one uninstall-registry row we match against.
type UninstallEntry struct {
	DisplayName     string `json:"displayName"`
	InstallLocation string `json:"installLocation"`
}

// Detector holds the read-only inputs of tool detection: paths, PATH and the
// uninstall registry. Every field is injectable so tests never touch the real
// machine and e2e can use a fake environment.
type Detector struct {
	Home             string
	LocalAppData     string
	UserProgramFiles string
	PathDirs         []string
	CLIDirs          []string
	Uninstall        []UninstallEntry
}

// DetectedTool is one registry row plus its resolved paths and status.
type DetectedTool struct {
	ID          string
	Name        string
	IconKey     string
	Shared      bool
	Status      string
	InstallDir  string
	AppDir      string
	ScanDirs    []string
	BuiltinDirs []string
	Readers     []string
	Official    bool
	Enabled     bool
}

// NewSystemDetector reads the real machine, or the fake environment in an
// e2e build (SKILLS_FAKE_HOME/PATH/REGISTRY, 技术设计 15.7).
func NewSystemDetector() *Detector {
	if fakeInputsEnabled {
		return newFakeDetector()
	}
	home, _ := os.UserHomeDir()
	detector := &Detector{Home: home, LocalAppData: os.Getenv("LOCALAPPDATA")}
	if p, err := windows.KnownFolderPath(windows.FOLDERID_UserProgramFiles, 0); err == nil {
		detector.UserProgramFiles = p
	}
	detector.PathDirs = filepath.SplitList(os.Getenv("PATH"))
	detector.CLIDirs = []string{
		filepath.Join(os.Getenv("APPDATA"), "npm"),
		filepath.Join(home, ".local", "bin"),
	}
	detector.Uninstall = readUninstall()
	return detector
}

// newFakeDetector builds the e2e detector from the three SKILLS_FAKE_* vars.
func newFakeDetector() *Detector {
	home := os.Getenv("SKILLS_FAKE_HOME")
	if home == "" {
		return &Detector{}
	}
	detector := &Detector{
		Home:         home,
		LocalAppData: filepath.Join(home, "AppData", "Local"),
		CLIDirs:      []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "AppData", "Roaming", "npm")},
		PathDirs:     filepath.SplitList(os.Getenv("SKILLS_FAKE_PATH")),
	}
	detector.UserProgramFiles = filepath.Join(home, "Programs")
	if raw := os.Getenv("SKILLS_FAKE_REGISTRY"); raw != "" {
		if body, err := os.ReadFile(raw); err == nil {
			var fake struct {
				UserProgramFiles string           `json:"userProgramFiles"`
				LocalAppData     string           `json:"localAppData"`
				Uninstall        []UninstallEntry `json:"uninstall"`
			}
			if json.Unmarshal(body, &fake) == nil {
				if fake.UserProgramFiles != "" {
					detector.UserProgramFiles = fake.UserProgramFiles
				}
				if fake.LocalAppData != "" {
					detector.LocalAppData = fake.LocalAppData
				}
				detector.Uninstall = fake.Uninstall
			}
		}
	}
	return detector
}

// readUninstall lists the uninstall entries under HKCU/HKLM (including
// WOW6432Node). Read-only; keys that cannot be opened are skipped.
func readUninstall() []UninstallEntry {
	paths := []struct {
		root registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
	}
	var entries []UninstallEntry
	for _, item := range paths {
		key, err := registry.OpenKey(item.root, item.path, registry.READ)
		if err != nil {
			continue
		}
		names, err := key.ReadSubKeyNames(-1)
		if err != nil {
			_ = key.Close()
			continue
		}
		for _, name := range names {
			sub, err := registry.OpenKey(key, name, registry.READ)
			if err != nil {
				continue
			}
			display, _, _ := sub.GetStringValue("DisplayName")
			location, _, _ := sub.GetStringValue("InstallLocation")
			if display != "" {
				entries = append(entries, UninstallEntry{DisplayName: display, InstallLocation: location})
			}
			_ = sub.Close()
		}
		_ = key.Close()
	}
	return entries
}

// Detect resolves every registry row.
func (d *Detector) Detect() []DetectedTool {
	out := make([]DetectedTool, 0, len(Registry))
	for _, tool := range Registry {
		out = append(out, d.detect(tool))
	}
	return out
}

func (d *Detector) detect(tool Tool) DetectedTool {
	install := d.ExpandPath(tool.InstallDir)
	appDir := d.appInstallDir(tool)
	scan := make([]string, 0, len(tool.ScanDirs)+len(tool.ScanRelative))
	for _, template := range tool.ScanDirs {
		scan = append(scan, d.ExpandPath(template))
	}
	builtin := make([]string, 0, len(tool.BuiltinDirs)+len(tool.AppBuiltinRelative))
	for _, template := range tool.BuiltinDirs {
		for _, dir := range ExpandWildcard(d.ExpandPath(template)) {
			builtin = append(builtin, dir)
		}
	}
	if install != "" {
		for _, rel := range tool.ScanRelative {
			scan = append(scan, filepath.Join(install, strings.ReplaceAll(rel, `\`, string(filepath.Separator))))
		}
	}
	if appDir != "" {
		for _, rel := range tool.AppBuiltinRelative {
			joined := filepath.Join(appDir, strings.ReplaceAll(rel, `\`, string(filepath.Separator)))
			for _, dir := range ExpandWildcard(joined) {
				builtin = append(builtin, dir)
			}
		}
	}
	status := StatusMissing
	switch {
	case tool.Shared:
		// The shared directory is a target, not something to detect (13.1).
		status = StatusDetected
	case d.hasEvidence(tool):
		status = StatusDetected
	default:
		for _, path := range append(append(scan, builtin...), install) {
			if path != "" && !strings.Contains(path, "*") && pathExists(path) {
				status = StatusLeftover
				break
			}
		}
	}
	return DetectedTool{
		ID: tool.ID, Name: tool.Name, IconKey: tool.IconKey, Shared: tool.Shared,
		Status: status, InstallDir: install, AppDir: appDir, ScanDirs: scan, BuiltinDirs: builtin, Readers: tool.Readers,
		Official: tool.Official, Enabled: tool.Enabled,
	}
}

// appInstallDir resolves where the tool itself is installed: the parent of a
// known executable under UserProgramFiles, else the uninstall entry's
// InstallLocation. Builtin skill dirs live under this location (13.2).
func (d *Detector) appInstallDir(tool Tool) string {
	for _, rel := range tool.KnownExes {
		exe := filepath.Join(d.UserProgramFiles, rel)
		if pathExists(exe) {
			return filepath.Dir(exe)
		}
	}
	for _, prefix := range tool.Uninstall {
		for _, entry := range d.Uninstall {
			if strings.HasPrefix(strings.ToLower(entry.DisplayName), strings.ToLower(prefix)) && entry.InstallLocation != "" {
				return entry.InstallLocation
			}
		}
	}
	return ""
}

// hasEvidence applies the three detection rules from 技术设计 13.3.
func (d *Detector) hasEvidence(tool Tool) bool {
	if d.cliFound(tool.CLIs) {
		return true
	}
	if d.uninstallFound(tool.Uninstall) {
		return true
	}
	for _, rel := range tool.KnownExes {
		if pathExists(filepath.Join(d.UserProgramFiles, rel)) {
			return true
		}
	}
	return false
}

func (d *Detector) cliFound(names []string) bool {
	if len(names) == 0 {
		return false
	}
	dirs := append(append([]string{}, d.CLIDirs...), d.PathDirs...)
	extensions := []string{"", ".exe", ".cmd", ".bat", ".ps1"}
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		for _, name := range names {
			for _, ext := range extensions {
				if pathExists(filepath.Join(dir, name+ext)) {
					return true
				}
			}
		}
	}
	return false
}

func (d *Detector) uninstallFound(prefixes []string) bool {
	for _, prefix := range prefixes {
		for _, entry := range d.Uninstall {
			if strings.HasPrefix(strings.ToLower(entry.DisplayName), strings.ToLower(prefix)) {
				return true
			}
		}
	}
	return false
}

// ExpandPath resolves "~" and "%LOCALAPPDATA%" in a registry template.
func (d *Detector) ExpandPath(template string) string {
	if template == "" {
		return ""
	}
	out := template
	if out == "~" || strings.HasPrefix(out, "~/") || strings.HasPrefix(out, `~\`) {
		out = filepath.Join(d.Home, strings.TrimLeft(out[1:], `/\`))
	}
	out = strings.ReplaceAll(out, "%LOCALAPPDATA%", d.LocalAppData)
	return filepath.FromSlash(out)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
