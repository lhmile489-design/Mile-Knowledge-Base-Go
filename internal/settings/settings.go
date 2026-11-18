// Package settings persists the application settings and per-vault recent
// documents, and imports the legacy Electron app's data once.
//
// Files live in the application data directory (identifier dev.mile.kb):
//
//	settings.json            AppSettings (versioned, unknown fields preserved)
//	recent-documents.json    vaultID -> recent documents
//	import-legacy.done       marker written after a successful legacy import
//
// Writes are atomic (temp file + rename); a corrupt file is renamed to
// "<name>.<ts>.corrupt" and defaults are used instead of crashing.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const (
	FileName        = "settings.json"
	RecentFileName  = "recent-documents.json"
	ImportMarker    = "import-legacy.done"
	Version         = 1
	MaxRecentVaults = 12
	MaxRecentDocs   = 12
)

// Width bounds (shared/settings.ts in the legacy app).
const (
	SidebarMin, SidebarDef, SidebarMax    = 192, 232, 320
	DocMin, DocDef, DocMax                = 320, 560, 800
	ComposerMin, ComposerDef, ComposerMax = 72, 96, 400
)

// Defaults returns the settings a fresh install starts with.
func Defaults() contract.AppSettings {
	return contract.AppSettings{
		Version:       Version,
		Theme:         "system",
		CloseBehavior: "ask",
		Layout: contract.LayoutSettings{
			SidebarWidth:           SidebarDef,
			DocWidth:               DocDef,
			DocCollapsed:           true,
			SidebarCollapsed:       false,
			AgnesControlsCollapsed: false,
			AgnesHistoryOpen:       true,
			AgnesComposerHeight:    ComposerDef,
		},
		RecentVaults: []contract.Vault{},
	}
}

// Store owns the settings and recent-document files.
type Store struct {
	dir       string
	legacyDir string

	mu        sync.Mutex
	settings  contract.AppSettings
	unknown   map[string]any
	recent    map[string][]contract.RecentDocument
	recentRaw map[string]any
}

// Open loads the files (or defaults). legacyDir is the old Electron userData
// directory, used only by the one-shot import.
func Open(dir, legacyDir string) (*Store, *contract.Failure) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, contract.Fail(contract.CodeIOFailure, "无法创建应用数据目录")
	}
	s := &Store{dir: dir, legacyDir: legacyDir, unknown: map[string]any{}, recentRaw: map[string]any{}}
	s.settings = Defaults()
	if f := s.load(); f != nil {
		return nil, f
	}
	if f := s.loadRecent(); f != nil {
		return nil, f
	}
	return s, nil
}

// legacyImportNeeded reports whether the one-shot import should run: no marker
// and no settings file of our own yet.
func (s *Store) legacyImportNeeded() bool {
	if s.legacyDir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(s.dir, ImportMarker)); err == nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(s.dir, FileName)); err == nil {
		return false
	}
	return true
}

// ImportResult summarises a legacy import for the UI notice.
type ImportResult struct {
	Ran      bool
	Imported bool
	Vaults   int
	Reason   string
}

// ImportLegacy performs the one-shot import described in 数据库设计 §5. It only
// reads the legacy directory. A missing legacy file is not an error: the
// marker is still written so the check does not repeat on every launch.
func (s *Store) ImportLegacy() ImportResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.legacyImportNeeded() {
		return ImportResult{Ran: false, Reason: "already imported or first launch not applicable"}
	}
	res := ImportResult{Ran: true}

	type legacySettings struct {
		Theme         string                   `json:"theme"`
		CloseBehavior string                   `json:"closeBehavior"`
		Layout        *contract.LayoutSettings `json:"layout"`
		RecentVaults  []contract.Vault         `json:"recentVaults"`
		LastVaultID   string                   `json:"lastVaultId"`
	}
	var legacy legacySettings
	raw, err := os.ReadFile(filepath.Join(s.legacyDir, FileName))
	if err == nil {
		if json.Unmarshal(raw, &legacy) == nil {
			if isTheme(legacy.Theme) {
				s.settings.Theme = legacy.Theme
			}
			if isClose(legacy.CloseBehavior) {
				s.settings.CloseBehavior = legacy.CloseBehavior
			}
			if legacy.Layout != nil {
				s.settings.Layout = clampLayout(*legacy.Layout)
			}
			if n := len(legacy.RecentVaults); n > 0 {
				if n > MaxRecentVaults {
					legacy.RecentVaults = legacy.RecentVaults[:MaxRecentVaults]
				}
				s.settings.RecentVaults = legacy.RecentVaults
				res.Vaults = n
			}
			if legacy.LastVaultID != "" {
				s.settings.LastVaultID = legacy.LastVaultID
			}
			res.Imported = true
		} else {
			res.Reason = "legacy settings.json 无法解析，按默认值运行"
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		res.Reason = "legacy settings.json 读取失败，按默认值运行"
	}

	// Recent documents: same shape as ours, keep only entries whose vault is
	// registered in the imported list.
	if recentRaw, err := os.ReadFile(filepath.Join(s.legacyDir, RecentFileName)); err == nil {
		var docs map[string][]contract.RecentDocument
		if json.Unmarshal(recentRaw, &docs) == nil {
			known := map[string]bool{}
			for _, v := range s.settings.RecentVaults {
				known[v.ID] = true
			}
			for id, list := range docs {
				if !known[id] || len(list) == 0 {
					continue
				}
				if len(list) > MaxRecentDocs {
					list = list[:MaxRecentDocs]
				}
				s.recent[id] = list
			}
		}
	}

	if f := s.saveLocked(); f != nil {
		res.Reason = "导入后写设置失败：" + f.Message
	}
	if f := s.saveRecentLocked(); f != nil && res.Reason == "" {
		res.Reason = "导入后写最近文档失败：" + f.Message
	}
	marker := map[string]any{
		"importedAt": time.Now().UTC().Format(time.RFC3339),
		"vaults":     res.Vaults,
		"source":     s.legacyDir,
		"ok":         res.Reason == "",
	}
	if b, err := marshalJSON(marker); err == nil {
		_ = os.WriteFile(filepath.Join(s.dir, ImportMarker), b, 0o644)
	}
	return res
}

// RecentVaults returns the persisted vault list (vault.Prefs).
func (s *Store) RecentVaults() []contract.Vault {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]contract.Vault, len(s.settings.RecentVaults))
	copy(out, s.settings.RecentVaults)
	return out
}

// LastVaultID returns the vault that was open when the app last ran.
func (s *Store) LastVaultID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.LastVaultID
}

// Get returns a copy of the current settings.
func (s *Store) Get() contract.AppSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// Patch applies a partial update and persists it.
func (s *Store) Patch(p contract.SettingsPatch) (contract.AppSettings, *contract.Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.Theme != nil {
		if !isTheme(*p.Theme) {
			return s.settings, contract.Fail(contract.CodeInvalidInput, "主题值不合法")
		}
		s.settings.Theme = *p.Theme
	}
	if p.CloseBehavior != nil {
		if !isClose(*p.CloseBehavior) {
			return s.settings, contract.Fail(contract.CodeInvalidInput, "关窗行为值不合法")
		}
		s.settings.CloseBehavior = *p.CloseBehavior
	}
	if p.Layout != nil {
		s.settings.Layout = clampLayout(*p.Layout)
	}
	if f := s.saveLocked(); f != nil {
		return s.settings, f
	}
	return s.settings, nil
}

// SaveVaults replaces the recent-vault list and the active id (vault registry).
func (s *Store) SaveVaults(vaults []contract.Vault, activeID string) *contract.Failure {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(vaults) > MaxRecentVaults {
		vaults = vaults[:MaxRecentVaults]
	}
	s.settings.RecentVaults = vaults
	s.settings.LastVaultID = activeID
	return s.saveLocked()
}

// RecentDocuments lists one vault's recent documents.
func (s *Store) RecentDocuments(vaultID string) []contract.RecentDocument {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.recent[vaultID]
	out := make([]contract.RecentDocument, len(list))
	copy(out, list)
	return out
}

// NoteDocumentOpened moves path to the front of the vault's list.
func (s *Store) NoteDocumentOpened(vaultID, path, openedAt string) ([]contract.RecentDocument, *contract.Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.recent[vaultID]
	next := []contract.RecentDocument{{Path: path, OpenedAt: openedAt}}
	for _, item := range list {
		if item.Path != path {
			next = append(next, item)
		}
	}
	if len(next) > MaxRecentDocs {
		next = next[:MaxRecentDocs]
	}
	s.recent[vaultID] = next
	if f := s.saveRecentLocked(); f != nil {
		return next, f
	}
	out := make([]contract.RecentDocument, len(next))
	copy(out, next)
	return out, nil
}

// ForgetVault drops a vault's recent documents (PRD FR-VLT).
func (s *Store) ForgetVault(vaultID string) *contract.Failure {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recent, vaultID)
	delete(s.recentRaw, vaultID)
	return s.saveRecentLocked()
}

// ---- files ----

func (s *Store) load() *contract.Failure {
	path := filepath.Join(s.dir, FileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return contract.Fail(contract.CodeIOFailure, "读取设置失败")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		s.backupCorrupt(path)
		return nil
	}
	var parsed contract.AppSettings
	if err := json.Unmarshal(raw, &parsed); err != nil {
		s.backupCorrupt(path)
		return nil
	}
	known := map[string]bool{"version": true, "theme": true, "closeBehavior": true, "layout": true, "recentVaults": true, "lastVaultId": true}
	for k, v := range doc {
		if !known[k] {
			s.unknown[k] = v
		}
	}
	s.settings = clampSettings(parsed)
	return nil
}

func (s *Store) loadRecent() *contract.Failure {
	path := filepath.Join(s.dir, RecentFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			s.recent = map[string][]contract.RecentDocument{}
			return nil
		}
		return contract.Fail(contract.CodeIOFailure, "读取最近文档失败")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		s.backupCorrupt(path)
		s.recent = map[string][]contract.RecentDocument{}
		return nil
	}
	s.recentRaw = doc
	s.recent = map[string][]contract.RecentDocument{}
	var typed map[string][]contract.RecentDocument
	if err := json.Unmarshal(raw, &typed); err == nil {
		for id, list := range typed {
			if len(list) > MaxRecentDocs {
				list = list[:MaxRecentDocs]
			}
			s.recent[id] = list
		}
	}
	return nil
}

func (s *Store) backupCorrupt(path string) {
	ts := time.Now().UTC().Format("20060102T150405")
	_ = os.Rename(path, fmt.Sprintf("%s.%s.corrupt", path, ts))
}

func (s *Store) saveLocked() *contract.Failure {
	payload := map[string]any{}
	for k, v := range s.unknown {
		payload[k] = v
	}
	payload["version"] = Version
	payload["theme"] = s.settings.Theme
	payload["closeBehavior"] = s.settings.CloseBehavior
	payload["layout"] = s.settings.Layout
	payload["recentVaults"] = s.settings.RecentVaults
	if s.settings.LastVaultID != "" {
		payload["lastVaultId"] = s.settings.LastVaultID
	}
	b, err := marshalJSON(payload)
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "序列化设置失败")
	}
	return writeAtomic(filepath.Join(s.dir, FileName), b)
}

func (s *Store) saveRecentLocked() *contract.Failure {
	payload := map[string]any{}
	for k, v := range s.recentRaw {
		payload[k] = v
	}
	for id, list := range s.recent {
		payload[id] = list
	}
	b, err := marshalJSON(payload)
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "序列化最近文档失败")
	}
	return writeAtomic(filepath.Join(s.dir, RecentFileName), b)
}

// marshalJSON matches the legacy writer: two-space indent, trailing newline,
// no HTML escaping (Go would otherwise turn < > & into <...).
func marshalJSON(v any) ([]byte, error) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}

func writeAtomic(path string, data []byte) *contract.Failure {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入设置")
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return contract.Fail(contract.CodeIOFailure, "无法写入设置")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(name)
		return contract.Fail(contract.CodeIOFailure, "无法写入设置")
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return contract.Fail(contract.CodeIOFailure, "无法写入设置")
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return contract.Fail(contract.CodeIOFailure, "无法写入设置")
	}
	return nil
}

// ---- validation ----

func isTheme(v string) bool { return v == "light" || v == "dark" || v == "system" }
func isClose(v string) bool { return v == "ask" || v == "tray" || v == "quit" }

func clampInt(v, min, max, def int) int {
	if v == 0 {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampLayout(l contract.LayoutSettings) contract.LayoutSettings {
	l.SidebarWidth = clampInt(l.SidebarWidth, SidebarMin, SidebarMax, SidebarDef)
	l.DocWidth = clampInt(l.DocWidth, DocMin, DocMax, DocDef)
	l.AgnesComposerHeight = clampInt(l.AgnesComposerHeight, ComposerMin, ComposerMax, ComposerDef)
	return l
}

func clampSettings(s contract.AppSettings) contract.AppSettings {
	if s.Version == 0 {
		s.Version = Version
	}
	if !isTheme(s.Theme) {
		s.Theme = "system"
	}
	if !isClose(s.CloseBehavior) {
		s.CloseBehavior = "ask"
	}
	s.Layout = clampLayout(s.Layout)
	if len(s.RecentVaults) > MaxRecentVaults {
		s.RecentVaults = s.RecentVaults[:MaxRecentVaults]
	}
	if s.RecentVaults == nil {
		s.RecentVaults = []contract.Vault{}
	}
	return s
}

// SortedRecentVaults returns the persisted list ordered for display: most
// recently opened first.
func SortedRecentVaults(list []contract.Vault) []contract.Vault {
	out := make([]contract.Vault, len(list))
	copy(out, list)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LastOpenedAt > out[j].LastOpenedAt
	})
	return out
}
