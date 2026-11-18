// Package contract holds the types that cross the Go ↔ TypeScript boundary:
// binding arguments, results, events and error codes. It must stay free of
// behaviour so that every module can depend on it without cycles.
//
// Field names follow docs/API.md; JSON tags are the wire contract.
package contract

// Failure is the predictable-failure arm of a result union. Go methods return
// these instead of Go errors for anything the UI can act on (API.md §1.2).
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Limit   int64  `json:"limit,omitempty"`
}

// Error codes (API.md §1.3).
const (
	CodeInvalidInput        = "invalid-input"
	CodePathEscape          = "path-escape"
	CodeNotFound            = "not-found"
	CodeNotAFile            = "not-a-file"
	CodeNotADirectory       = "not-a-directory"
	CodeTooLarge            = "too-large"
	CodeUnsupportedEncoding = "unsupported-encoding"
	CodeRevisionMismatch    = "revision-mismatch"
	CodeVaultUnavailable    = "vault-unavailable"
	CodePermissionDenied    = "permission-denied"
	CodeIOFailure           = "io-failure"
	CodeUnsupported         = "unsupported"
	CodeEngineUnavailable   = "engine-unavailable"
	CodeProviderError       = "provider-error"
	CodeApprovalExpired     = "approval-expired"
	CodeSecretUnavailable   = "secret-unavailable"
)

// Fail builds a Failure.
func Fail(code, message string) *Failure { return &Failure{Code: code, Message: message} }

// WithDetail adds a safe detail line.
func (f *Failure) WithDetail(detail string) *Failure { f.Detail = detail; return f }

// WithLimit records the offending size and the limit that rejected it.
func (f *Failure) WithLimit(size, limit int64) *Failure {
	f.Size, f.Limit = size, limit
	return f
}

// ---- Vaults (API.md §4.1) ----

// Vault is one registered, usable knowledge base.
type Vault struct {
	ID           string `json:"id"`
	Root         string `json:"root"`
	DisplayName  string `json:"displayName"`
	CreatedAt    string `json:"createdAt,omitempty"`
	LastOpenedAt string `json:"lastOpenedAt,omitempty"`
}

// UnavailableVault is a registered vault that cannot be opened right now.
type UnavailableVault struct {
	ID          string `json:"id"`
	Root        string `json:"root"`
	DisplayName string `json:"displayName"`
	Reason      string `json:"reason"` // missing | no-permission
}

// VaultListResult is returned by list/open/forget/retry.
type VaultListResult struct {
	Vaults        []Vault            `json:"vaults"`
	ActiveVaultID string             `json:"activeVaultId,omitempty"`
	Unavailable   []UnavailableVault `json:"unavailable"`
	Failure       *Failure           `json:"failure,omitempty"`
}

// ChooseVaultResult is returned by choose/createVault.
type ChooseVaultResult struct {
	Cancelled     bool     `json:"cancelled"`
	Vault         *Vault   `json:"vault,omitempty"`
	ScaffoldedDir []string `json:"scaffoldedDirs,omitempty"`
	Failure       *Failure `json:"failure,omitempty"`
}

// PickFolderResult is the wizard's first step; the token replaces the path.
type PickFolderResult struct {
	Cancelled   bool     `json:"cancelled"`
	Token       string   `json:"token,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Empty       bool     `json:"empty"`
	EntryCount  int      `json:"entryCount"`
	WillCreate  []string `json:"willCreate"`
	Failure     *Failure `json:"failure,omitempty"`
}

// ---- Tree and documents (API.md §4.2) ----

// FileEntry is one row of the lazily loaded tree.
type FileEntry struct {
	VaultID    string `json:"vaultId"`
	Path       string `json:"path"`
	Name       string `json:"name"`
	Kind       string `json:"kind"` // file | directory
	Area       string `json:"area"`
	Size       int64  `json:"size,omitempty"`
	MtimeMs    int64  `json:"mtimeMs,omitempty"`
	Revision   int64  `json:"revision,omitempty"`
	Ignored    bool   `json:"ignored,omitempty"`
	ChildCount *int   `json:"childCount,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Alias      string `json:"alias,omitempty"` // semantic icon key, mapped to a component by the UI
}

// TreeResult is one directory level.
type TreeResult struct {
	VaultID            string      `json:"vaultId"`
	Path               string      `json:"path"`
	Entries            []FileEntry `json:"entries"`
	SkillCategoryCount int         `json:"skillCategoryCount,omitempty"`
	Failure            *Failure    `json:"failure,omitempty"`
}

// DocumentSnapshot is a document as the editor sees it.
type DocumentSnapshot struct {
	VaultID  string `json:"vaultId"`
	Path     string `json:"path"`
	Text     string `json:"text"` // always LF; the writer restores the original style
	Revision int64  `json:"revision"`
	Hash     string `json:"hash"`
	Encoding string `json:"encoding"` // utf8
	Size     int64  `json:"size"`
	EOL      string `json:"eol"` // lf | crlf
	BOM      bool   `json:"bom"`
	Degraded string `json:"degraded,omitempty"`
}

// ReadResult is the read outcome union.
type ReadResult struct {
	Doc     *DocumentSnapshot `json:"doc,omitempty"`
	Failure *Failure          `json:"failure,omitempty"`
}

// WriteOutcome is the save outcome union (API.md M18).
type WriteOutcome struct {
	Status       string   `json:"status"` // written | conflict
	Revision     int64    `json:"revision,omitempty"`
	Hash         string   `json:"hash,omitempty"`
	Size         int64    `json:"size,omitempty"`
	DiskText     string   `json:"diskText,omitempty"`
	DiskRevision int64    `json:"diskRevision,omitempty"`
	DiskHash     string   `json:"diskHash,omitempty"`
	DiskSize     int64    `json:"diskSize,omitempty"`
	BaseRevision int64    `json:"baseRevision,omitempty"`
	Failure      *Failure `json:"failure,omitempty"`
}

// SaveImageResult points at the content-addressed attachment.
type SaveImageResult struct {
	Path    string   `json:"path,omitempty"`
	Existed bool     `json:"existed,omitempty"`
	Failure *Failure `json:"failure,omitempty"`
}

// ---- Settings (API.md §4.8, 数据库设计 §4.1) ----

// LayoutSettings mirrors the persisted workbench geometry.
type LayoutSettings struct {
	SidebarWidth           int  `json:"sidebarWidth"`
	DocWidth               int  `json:"docWidth"`
	DocCollapsed           bool `json:"docCollapsed"`
	SidebarCollapsed       bool `json:"sidebarCollapsed"`
	AgnesControlsCollapsed bool `json:"agnesControlsCollapsed"`
	AgnesHistoryOpen       bool `json:"agnesHistoryOpen"`
	AgnesComposerHeight    int  `json:"agnesComposerHeight"`
}

// AppSettings is the whole persisted settings document.
type AppSettings struct {
	Version       int            `json:"version"`
	Theme         string         `json:"theme"`         // light | dark | system
	CloseBehavior string         `json:"closeBehavior"` // ask | tray | quit
	Layout        LayoutSettings `json:"layout"`
	RecentVaults  []Vault        `json:"recentVaults"`
	LastVaultID   string         `json:"lastVaultId,omitempty"`
	Unknown       map[string]any `json:"-"` // fields written by other versions, preserved verbatim
}

// SettingsPatch is a partial update; nil fields are untouched.
type SettingsPatch struct {
	Theme         *string         `json:"theme,omitempty"`
	CloseBehavior *string         `json:"closeBehavior,omitempty"`
	Layout        *LayoutSettings `json:"layout,omitempty"`
}

// SettingsResult wraps the current settings.
type SettingsResult struct {
	Settings *AppSettings `json:"settings,omitempty"`
	Failure  *Failure     `json:"failure,omitempty"`
}

// RecentDocument is one entry of the per-vault recent list.
type RecentDocument struct {
	Path     string `json:"path"`
	OpenedAt string `json:"openedAt"`
}

// RecentDocumentsResult is the recent list for one vault.
type RecentDocumentsResult struct {
	VaultID   string           `json:"vaultId"`
	Documents []RecentDocument `json:"documents"`
	Failure   *Failure         `json:"failure,omitempty"`
}

// Event names (API.md §2.3).
const (
	EventVaultChanged    = "vault-changed"
	EventSettingsChanged = "settings-changed"
	EventThemeChanged    = "theme-changed"
	EventWindowState     = "window-state"
	EventAppNotice       = "app-notice"
)

// VaultChangedEvent tells the page which directory levels need a refresh.
type VaultChangedEvent struct {
	VaultID     string   `json:"vaultId"`
	Directories []string `json:"directories"`
	Sequence    int64    `json:"sequence"`
}

// WindowStateEvent mirrors maximize/focus changes for the self-drawn chrome.
type WindowStateEvent struct {
	Maximized bool `json:"maximized"`
	Focused   bool `json:"focused"`
}

// AppNoticeEvent is a non-blocking notice (import fallback, watcher limits).
type AppNoticeEvent struct {
	Text  string `json:"text"`
	Level string `json:"level"` // info | warn | fault
}
