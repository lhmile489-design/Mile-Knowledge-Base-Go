// Package service is the binding layer: it validates inputs, wires the domain
// packages together and exposes the methods the React app calls.
//
// Only this package (and main) imports mygo.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/conversation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/mutation"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/render"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/settings"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/watch"
)

// Events pushed to the page. Event names match API.md §2.3.
var (
	VaultChanged    = mygo.NewEvent[contract.VaultChangedEvent]("vault-changed")
	SettingsChanged = mygo.NewEvent[contract.AppSettings]("settings-changed")
	ThemeChanged    = mygo.NewEvent[map[string]string]("theme-changed")
	WindowState     = mygo.NewEvent[contract.WindowStateEvent]("window-state")
	AppNotice       = mygo.NewEvent[contract.AppNoticeEvent]("app-notice")
	AuditChanged    = mygo.NewEvent[contract.AuditChangedEvent]("audit-changed")
	// E07–E10 (API.md §2.3).
	ConversationChanged = mygo.NewEvent[contract.ConversationChangedEvent]("conversation-changed")
	AppConfigChanged    = mygo.NewEvent[contract.AppConfigChangedEvent]("app-config-changed")
	ContextChanged      = mygo.NewEvent[contract.SharedContext]("context-changed")
	EngineStatusEvent   = mygo.NewEvent[contract.EngineStatus]("engine-status")
)

// App holds the shared state of the running application.
type App struct {
	DataDir  string
	Settings *settings.Store
	Vaults   *vault.Registry
	Audit    *mutation.AuditLedger
	Keys     *ai.KeyStore
	// Context is the shared context hub (M45–M49, T08).
	Context *ai.ContextHub
	// Conversations is the conversation history store (M50–M59, T09).
	Conversations *conversation.Store
	// Agents keeps one session per (agent, vault) for the turn loop (T10).
	Agents *ai.Registry
	// AgentStream replaces the real provider stream in tests.
	AgentStream ai.StreamOptions
	// Blocks caches the streaming chat block HTML (M37).
	Blocks *render.BlockCache
	// AgentBusy reports whether a turn is running for the vault (M56 guard,
	// wired by the turn loop in T10).
	AgentBusy func(vaultID string) bool
	// Version is the packaged app version (M74); main.go fills it.
	Version string

	mu             sync.Mutex
	win            *mygo.Window
	tray           *mygo.Tray
	trayOK         bool
	quitting       bool
	dirty          bool
	picks          map[string]pickToken
	skillPreviews  map[string]*vault.SkillSourcePreview
	watchers       map[string]context.CancelFunc
	indexes        map[string]*indexHandle
	vaultRevisions map[string]int64
	recaps         map[string]string
	engineStatus   map[string]string
	auditSeq       int64
	auditErr       string
	resolvers      map[string]*vaultResolver
}

type pickToken struct {
	path      string
	expiresAt time.Time
}

// New builds the app state. dataDir is the application's user-data directory.
func New(dataDir, legacyDir string) (*App, error) {
	audit, auditErr := mutation.OpenAudit(filepath.Join(dataDir, "audit.jsonl"))
	if auditErr != nil {
		audit = nil
	}
	store, f := settings.Open(dataDir, legacyDir)
	if f != nil {
		return nil, errFromFailure(*f)
	}
	app := &App{
		DataDir:        dataDir,
		Settings:       store,
		Audit:          audit,
		Keys:           ai.NewKeyStore(ai.NewSecretsBackend("")),
		Context:        ai.NewContextHub(),
		Conversations:  conversation.NewStore(dataDir),
		Agents:         ai.NewRegistry(),
		Blocks:         render.NewBlockCache(),
		auditErr:       auditErrText(auditErr),
		picks:          map[string]pickToken{},
		skillPreviews:  map[string]*vault.SkillSourcePreview{},
		watchers:       map[string]context.CancelFunc{},
		indexes:        map[string]*indexHandle{},
		vaultRevisions: map[string]int64{},
		recaps:         map[string]string{},
		engineStatus:   map[string]string{},
		resolvers:      map[string]*vaultResolver{},
	}
	app.Vaults = vault.New(store)
	app.Vaults.Load()
	// M56: switching the active conversation is refused while a turn runs.
	app.AgentBusy = app.Agents.AnyBusy
	return app, nil
}

type errFromFailure contract.Failure

func (e errFromFailure) Error() string { return e.Message }

// SetWindow records the main window once it exists.
func (a *App) SetWindow(w *mygo.Window) {
	a.mu.Lock()
	a.win = w
	errText := a.auditErr
	a.mu.Unlock()
	if errText != "" {
		_ = AppNotice.Emit(w, contract.AppNoticeEvent{Text: "操作日志不可用：" + errText, Level: "warn"})
	}
}

func auditErrText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Window returns the main window.
func (a *App) Window() *mygo.Window {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.win
}

// ---- SettingsService (API.md M60, M61) ----

// SettingsService binds the application settings.
type SettingsService struct{ app *App }

// NewSettingsService binds one app.
func NewSettingsService(app *App) *SettingsService { return &SettingsService{app: app} }

// Get returns the current settings.
func (s *SettingsService) Get() contract.SettingsResult {
	cfg := s.app.Settings.Get()
	return contract.SettingsResult{Settings: &cfg}
}

// Patch applies a partial update and broadcasts the result.
func (s *SettingsService) Patch(patch contract.SettingsPatch) contract.SettingsResult {
	cfg, f := s.app.Settings.Patch(patch)
	if f != nil {
		return contract.SettingsResult{Failure: f}
	}
	if patch.Theme != nil {
		switch *patch.Theme {
		case "light":
			mygo.Theme.SetSource(mygo.ThemeLight)
		case "dark":
			mygo.Theme.SetSource(mygo.ThemeDark)
		default:
			mygo.Theme.SetSource(mygo.ThemeSystem)
		}
	}
	if w := s.app.Window(); w != nil {
		_ = SettingsChanged.Emit(w, cfg)
	}
	return contract.SettingsResult{Settings: &cfg}
}

// ---- VaultService (API.md M06–M13, M72, M73) ----

// VaultService binds vault registration and the wizard.
type VaultService struct{ app *App }

// NewVaultService binds one app.
func NewVaultService(app *App) *VaultService { return &VaultService{app: app} }

// List returns the registered vaults.
func (s *VaultService) List() contract.VaultListResult { return s.app.Vaults.List() }

// Open makes a registered vault active.
func (s *VaultService) Open(vaultID string) contract.VaultListResult {
	if f := validateID(vaultID); f != nil {
		return contract.VaultListResult{Failure: f}
	}
	if f := s.app.Vaults.Open(vaultID); f != nil {
		return contract.VaultListResult{Failure: f}
	}
	s.app.loadAliases(vaultID)
	s.app.startWatch(vaultID)
	s.app.startIndex(vaultID)
	s.app.syncContextVault(vaultID)
	return s.app.Vaults.List()
}

// Forget removes a vault and its recent documents.
func (s *VaultService) Forget(vaultID string) contract.VaultListResult {
	if f := validateID(vaultID); f != nil {
		return contract.VaultListResult{Failure: f}
	}
	s.app.stopWatch(vaultID)
	s.app.indexStop(vaultID, true)
	root, _ := s.app.Vaults.Root(vaultID)
	if f := s.app.Vaults.Forget(vaultID); f != nil {
		return contract.VaultListResult{Failure: f}
	}
	_ = s.app.Settings.ForgetVault(vaultID)
	if root != "" {
		vault.ResetRevisions(root)
	}
	result := s.app.Vaults.List()
	return result
}

// Retry re-checks an unavailable vault.
func (s *VaultService) Retry(vaultID string) contract.VaultListResult {
	if f := validateID(vaultID); f != nil {
		return contract.VaultListResult{Failure: f}
	}
	s.app.Vaults.Retry(vaultID)
	return s.app.Vaults.List()
}

// dialogs may block; mygo runs every call on its own goroutine.

// Choose opens the directory picker and registers the chosen vault.
func (s *VaultService) Choose(ctx context.Context) contract.ChooseVaultResult {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     "选择知识库目录",
		Directory: true,
	})
	if err != nil || len(paths) == 0 {
		return contract.ChooseVaultResult{Cancelled: true}
	}
	return s.registerChosen(paths[0])
}

// PickVaultDir is the "choose directory" button on the welcome page; it does
// not register anything by itself (same as Choose for M0's UI).
func (s *WindowService) PickVaultDir(ctx context.Context) contract.ChooseVaultResult {
	return s.vaults.Choose(ctx)
}

func (s *VaultService) registerChosen(root string) contract.ChooseVaultResult {
	v, f := s.app.Vaults.Register(root)
	if f != nil {
		return contract.ChooseVaultResult{Failure: f}
	}
	var scaffolded []string
	if vault.NeedsScaffold(v.Root) {
		created, err := vault.CreateScaffold(v.Root)
		if err == nil {
			scaffolded = created
		}
	}
	s.app.loadAliases(v.ID)
	s.app.startWatch(v.ID)
	s.app.startIndex(v.ID)
	s.app.syncContextVault(v.ID)
	return contract.ChooseVaultResult{Vault: &v, ScaffoldedDir: scaffolded}
}

// PickFolder is the wizard's first step: it returns an opaque token instead of
// the absolute path.
func (s *VaultService) PickFolder(ctx context.Context) contract.PickFolderResult {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     "选择知识库目录",
		Directory: true,
	})
	if err != nil || len(paths) == 0 {
		return contract.PickFolderResult{Cancelled: true}
	}
	root, err := pathguard.Canon(paths[0])
	if err != nil {
		return contract.PickFolderResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "目录不可用")}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return contract.PickFolderResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "无法读取目录")}
	}
	empty := true
	count := 0
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if name == ".ds_store" || name == "thumbs.db" || name == "desktop.ini" {
			continue
		}
		empty = false
		count++
	}
	token := s.app.newPickToken(root)
	willCreate := []string{}
	if vault.NeedsScaffold(root) {
		willCreate = append(willCreate, "raws", "_archives", "projects", "tools", "AI Knowledge base", "dev-constraints")
	}
	return contract.PickFolderResult{
		Token:       token,
		DisplayName: filepath.Base(root),
		Empty:       empty,
		EntryCount:  count,
		WillCreate:  willCreate,
	}
}

// PickSkillSource is the skill wizard's first step (M14): pick a folder, list
// the skills one level down and hand back an opaque token.
func (s *VaultService) PickSkillSource(ctx context.Context) contract.PickSkillSourceResult {
	result := contract.PickSkillSourceResult{Candidates: []contract.SkillCandidate{}}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    mygo.CallerWindow(ctx),
		Title:     "选择技能所在的文件夹",
		Directory: true,
	})
	if err != nil || len(paths) == 0 {
		result.Cancelled = true
		return result
	}
	preview, err := vault.PreviewSkillSource(paths[0])
	if err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "无法读取技能目录")
		return result
	}
	app := s.app
	// Drop previews whose token already expired, so the map cannot grow forever.
	app.mu.Lock()
	for token := range app.skillPreviews {
		if item, ok := app.picks[token]; !ok || time.Now().After(item.expiresAt) {
			delete(app.skillPreviews, token)
		}
	}
	app.mu.Unlock()

	token := app.newPickToken(paths[0])
	app.mu.Lock()
	app.skillPreviews[token] = preview
	app.mu.Unlock()

	for _, candidate := range preview.Candidates {
		result.Candidates = append(result.Candidates, contract.SkillCandidate{
			Name: candidate.Name, FileCount: candidate.FileCount,
		})
	}
	result.Token = token
	return result
}

// ImportSkills copies the selected skills into skills/<category>/ (M15). The
// token is not consumed: a name that already exists can be retried under
// another category until the token expires.
func (s *VaultService) ImportSkills(ctx context.Context, vaultID, token, category string, names []string) contract.ImportSkillsResult {
	result := contract.ImportSkillsResult{Imported: []string{}, Skipped: []contract.SkillSkip{}}
	if f := validateID(vaultID); f != nil {
		result.Failure = f
		return result
	}
	if f := validateID(token); f != nil {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "技能来源已过期，请重新选择")
		return result
	}
	if len(names) == 0 {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "names 不能为空")
		return result
	}
	if len(names) > maxImportNames {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "一次导入的技能过多")
		return result
	}
	for _, name := range names {
		if len([]rune(name)) > 200 {
			result.Failure = contract.Fail(contract.CodeInvalidInput, "技能名过长")
			return result
		}
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		result.Failure = contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		return result
	}
	source, f := s.app.peekPickToken(token)
	if f != nil {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "技能来源已过期，请重新选择")
		return result
	}
	s.app.mu.Lock()
	preview := s.app.skillPreviews[token]
	s.app.mu.Unlock()
	if preview == nil {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "技能来源已过期，请重新选择")
		return result
	}

	unique := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}

	outcome, f := vault.ImportSkills(root, source, preview, category, unique)
	if f != nil {
		result.Failure = f
		return result
	}
	result.Imported = outcome.Imported
	for _, skip := range outcome.Skipped {
		result.Skipped = append(result.Skipped, contract.SkillSkip{Name: skip.Name, Reason: skip.Reason})
	}
	if len(outcome.Imported) > 0 {
		s.app.enqueueIndex(vaultID, []string{"skills/" + category})
	}
	return result
}

// maxImportNames caps one import request (legacy MAX_IMPORT_NAMES).
const maxImportNames = 200

// peekPickToken reads a token without consuming it (M15 may retry).
func (a *App) peekPickToken(token string) (string, *contract.Failure) {
	a.mu.Lock()
	defer a.mu.Unlock()
	item, ok := a.picks[token]
	if !ok || time.Now().After(item.expiresAt) {
		return "", contract.Fail(contract.CodeInvalidInput, "技能来源已过期，请重新选择")
	}
	return item.path, nil
}

// CreateVault finishes the wizard.
func (s *VaultService) CreateVault(token string, confirmNonEmpty bool) contract.ChooseVaultResult {
	root, f := s.app.consumePickToken(token)
	if f != nil {
		return contract.ChooseVaultResult{Failure: f}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return contract.ChooseVaultResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "无法读取目录")}
	}
	nonEmpty := false
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if name == ".ds_store" || name == "thumbs.db" || name == "desktop.ini" {
			continue
		}
		nonEmpty = true
		break
	}
	if nonEmpty && !confirmNonEmpty {
		return contract.ChooseVaultResult{Failure: contract.Fail(contract.CodePermissionDenied, "目录非空，需要确认")}
	}
	return s.registerChosen(root)
}

func (a *App) newPickToken(path string) string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	a.mu.Lock()
	a.picks[token] = pickToken{path: path, expiresAt: time.Now().Add(10 * time.Minute)}
	a.mu.Unlock()
	return token
}

func (a *App) consumePickToken(token string) (string, *contract.Failure) {
	a.mu.Lock()
	defer a.mu.Unlock()
	item, ok := a.picks[token]
	if !ok {
		return "", contract.Fail(contract.CodeInvalidInput, "选择已过期，请重新选择目录")
	}
	delete(a.picks, token)
	if time.Now().After(item.expiresAt) {
		return "", contract.Fail(contract.CodeInvalidInput, "选择已过期，请重新选择目录")
	}
	return item.path, nil
}

func (a *App) loadAliases(vaultID string) {
	root, f := a.Vaults.Root(vaultID)
	if f != nil {
		return
	}
	raw, err := os.ReadFile(filepath.Join(root, ".mile-kb", "settings", "aliases.json"))
	if err != nil {
		a.Vaults.SetAliases(vaultID, nil)
		return
	}
	var aliases map[string]string
	if json.Unmarshal(raw, &aliases) != nil {
		a.Vaults.SetAliases(vaultID, nil)
		return
	}
	a.Vaults.SetAliases(vaultID, aliases)
}

// ---- DocService (API.md M16–M18, M72, M73) ----

// DocService binds tree and document operations.
type DocService struct{ app *App }

// NewDocService binds one app.
func NewDocService(app *App) *DocService { return &DocService{app: app} }

// Tree returns one directory level.
func (s *DocService) Tree(vaultID, path string) contract.TreeResult {
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.TreeResult{Failure: f}
	}
	result, f := vault.ListDir(root, path, s.app.Vaults.Aliases(vaultID))
	if f != nil {
		return contract.TreeResult{Failure: f}
	}
	result.VaultID = vaultID
	for i := range result.Entries {
		result.Entries[i].VaultID = vaultID
	}
	return result
}

// Read loads a document.
func (s *DocService) Read(vaultID, path string) contract.ReadResult {
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.ReadResult{Failure: f}
	}
	snap, f := vault.Read(root, path)
	if f != nil {
		return contract.ReadResult{Failure: f}
	}
	snap.VaultID = vaultID
	return contract.ReadResult{Doc: &snap}
}

// Write saves a document unless the disk moved on.
func (s *DocService) Write(vaultID, path string, expectedRevision int64, text string) contract.WriteOutcome {
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return contract.WriteOutcome{Failure: f}
	}
	out, f := vault.Write(root, path, expectedRevision, text)
	if f != nil {
		out.Failure = f
	}
	return out
}

// RecentDocuments lists the vault's recent documents.
func (s *DocService) RecentDocuments(vaultID string) contract.RecentDocumentsResult {
	docs := s.app.Settings.RecentDocuments(vaultID)
	return contract.RecentDocumentsResult{VaultID: vaultID, Documents: docs}
}

// NoteDocumentOpened records an open for the welcome page and overview.
func (s *DocService) NoteDocumentOpened(vaultID, path string) contract.RecentDocumentsResult {
	if _, f := s.app.Vaults.Root(vaultID); f != nil {
		return contract.RecentDocumentsResult{Failure: f}
	}
	docs, f := s.app.Settings.NoteDocumentOpened(vaultID, path, time.Now().UTC().Format(time.RFC3339))
	if f != nil {
		return contract.RecentDocumentsResult{Failure: f}
	}
	return contract.RecentDocumentsResult{VaultID: vaultID, Documents: docs}
}

// ---- WindowService (API.md M01–M06) ----

// WindowService binds window chrome and the close flow.
type WindowService struct {
	app    *App
	vaults *VaultService
}

// NewWindowService binds one app.
func NewWindowService(app *App, vaults *VaultService) *WindowService {
	return &WindowService{app: app, vaults: vaults}
}

// Minimize minimizes the calling window.
func (s *WindowService) Minimize(ctx context.Context) error {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Minimize()
	}
	return nil
}

// ToggleMaximize toggles between maximized and restored.
func (s *WindowService) ToggleMaximize(ctx context.Context) error {
	if w := mygo.CallerWindow(ctx); w != nil {
		if w.IsMaximized() {
			w.Unmaximize()
		} else {
			w.Maximize()
		}
	}
	return nil
}

// Close asks the window to close; the close guard decides what happens.
func (s *WindowService) Close(ctx context.Context) error {
	if w := mygo.CallerWindow(ctx); w != nil {
		w.Close()
	}
	return nil
}

// Ready is the page's boot handshake; it is idempotent and lets queued events
// flow.
func (s *WindowService) Ready(ctx context.Context) error { return nil }

// OpenExternal opens an http(s) link in the system browser.
func (s *WindowService) OpenExternal(ctx context.Context, url string) contract.Failure {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		f := contract.Fail(contract.CodeInvalidInput, "只允许 http(s) 链接")
		return *f
	}
	go mygo.Shell.OpenExternal(url)
	return contract.Failure{}
}

// ReportDocument mirrors the editor's dirty state for the close guard.
func (s *WindowService) ReportDocument(vaultID, path string, revision int64, dirty bool) error {
	s.app.mu.Lock()
	s.app.dirty = dirty
	s.app.mu.Unlock()
	return nil
}

// ---- close guard ----

// CloseAction is what the close guard decided to do.
type CloseAction string

// Possible close outcomes.
const (
	CloseAsk  CloseAction = "ask"
	CloseHide CloseAction = "tray"
	CloseQuit CloseAction = "quit"
	CloseStay CloseAction = "stay"
)

// DecideClose implements the window-close matrix. It is pure so that it can be
// unit tested without windows or dialogs.
func DecideClose(behavior string, dirty bool, trayOK bool, unsavedChoice string, askResult string) CloseAction {
	if dirty && unsavedChoice != "discard" {
		return CloseStay
	}
	action := behavior
	switch behavior {
	case "ask":
		switch askResult {
		case "tray":
			action = "tray"
		case "quit":
			action = "quit"
		default:
			action = "tray"
		}
	case "":
		action = "ask"
	}
	if action == "tray" && !trayOK {
		return CloseQuit
	}
	return CloseAction(action)
}

// MarkQuitting lets the OnClose handler permit a real close.
func (a *App) MarkQuitting() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
}

// IsQuitting reports whether the app is on its way out.
func (a *App) IsQuitting() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.quitting
}

// IsDirty reports the editor's unsaved state.
func (a *App) IsDirty() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dirty
}

// SetTray records whether the tray icon was created.
func (a *App) SetTray(t *mygo.Tray, ok bool) {
	a.mu.Lock()
	a.tray = t
	a.trayOK = ok
	a.mu.Unlock()
}

// TrayOK reports whether the tray exists.
func (a *App) TrayOK() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.trayOK
}

// ShowWindow reveals the main window from the tray.
func (a *App) ShowWindow() {
	if w := a.Window(); w != nil {
		w.Show()
		w.Focus()
	}
}

// ---- validation helpers ----

func validateID(id string) *contract.Failure {
	if id == "" || len(id) > 128 {
		return contract.Fail(contract.CodeInvalidInput, "id 不合法")
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return contract.Fail(contract.CodeInvalidInput, "id 不合法")
		}
	}
	return nil
}

// ---- watcher wiring (T08) ----

func (a *App) startWatch(vaultID string) {
	root, f := a.Vaults.Root(vaultID)
	if f != nil {
		return
	}
	a.startIndex(vaultID)
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	if old, ok := a.watchers[vaultID]; ok {
		old()
	}
	a.watchers[vaultID] = cancel
	win := a.win
	seq := int64(0)
	a.mu.Unlock()

	watcher, err := watch.New(watch.MygoBackend{}, root, func(dirs []string) {
		a.enqueueIndex(vaultID, dirs)
		a.bumpVaultRevision(vaultID)
		a.syncContextVault(vaultID)
		if win == nil {
			return
		}
		seq++
		_ = VaultChanged.Emit(win, contract.VaultChangedEvent{VaultID: vaultID, Directories: dirs, Sequence: seq})
	})
	if err != nil {
		_ = AppNotice.Emit(win, contract.AppNoticeEvent{Text: "文件监听不可用，外部修改需要手动刷新", Level: "warn"})
		return
	}
	go func() {
		if err := watcher.Run(ctx); err != nil && ctx.Err() == nil {
			if win != nil {
				_ = AppNotice.Emit(win, contract.AppNoticeEvent{Text: "文件监听已停止：" + err.Error(), Level: "warn"})
			}
		}
	}()
}

func (a *App) stopWatch(vaultID string) {
	a.mu.Lock()
	cancel, ok := a.watchers[vaultID]
	if ok {
		delete(a.watchers, vaultID)
	}
	a.mu.Unlock()
	if ok {
		cancel()
	}
}

// StartActiveWatch begins watching the persisted active vault, if any.
func (a *App) StartActiveWatch() {
	if id := a.Vaults.Active(); id != "" {
		a.loadAliases(id)
		a.startWatch(id)
		a.startIndex(id)
		a.syncContextVault(id)
		a.emitEngineStatus(id)
	}
}
