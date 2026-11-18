// Package contract holds the types that cross the Go ↔ TypeScript boundary:
// binding arguments, results, events and error codes. It must stay free of
// behaviour so that every module can depend on it without cycles.
//
// Field names follow docs/API.md; JSON tags are the wire contract.
package contract

import "encoding/json"

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
	CodeNameConflict        = "name-conflict"
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

// UnresolvedLinkItem is one broken wiki/internal link (API.md M28).
type UnresolvedLinkItem struct {
	From       string `json:"from"`
	Target     string `json:"target"`
	Candidates int    `json:"candidates"`
}

// DuplicateNameItem groups files that share a name in different directories.
type DuplicateNameItem struct {
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

// OrphanItem is a document with neither incoming nor outgoing links.
type OrphanItem struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

// UntitledItem is a document whose title fell back to its file name.
type UntitledItem struct {
	Path     string `json:"path"`
	FileName string `json:"fileName"`
}

// KbHealthReport is the M28 response.
type KbHealthReport struct {
	VaultID         string               `json:"vaultId"`
	UnresolvedLinks []UnresolvedLinkItem `json:"unresolvedLinks"`
	Orphans         []OrphanItem         `json:"orphans"`
	Duplicates      []DuplicateNameItem  `json:"duplicates"`
	Untitled        []UntitledItem       `json:"untitled"`
	IndexRevision   int64                `json:"indexRevision"`
	Partial         bool                 `json:"partial"`
	Failure         *Failure             `json:"failure,omitempty"`
}

// ResolvedLink is one outgoing edge (API.md M27).
type ResolvedLink struct {
	Target     string  `json:"target"`
	Text       string  `json:"text"`
	Kind       string  `json:"kind"`
	Path       *string `json:"path"`
	Candidates int     `json:"candidates"`
}

// LinkGraphResult is the M27 response: outgoing edges plus backlinks.
type LinkGraphResult struct {
	Path      string         `json:"path"`
	Outgoing  []ResolvedLink `json:"outgoing"`
	Backlinks []string       `json:"backlinks"`
	Failure   *Failure       `json:"failure,omitempty"`
}

// AppConfigResult is the M62/M66 response: vault-level settings the UI reads.
type AppConfigResult struct {
	VaultID    string            `json:"vaultId,omitempty"`
	Aliases    map[string]string `json:"aliases"`
	Categories json.RawMessage   `json:"categories,omitempty"`
	Failure    *Failure          `json:"failure,omitempty"`
}

// ProvenanceRecordShape is one copy -> source relationship (API.md M34).
type ProvenanceRecordShape struct {
	CopyPath     string               `json:"copyPath"`
	SourcePath   string               `json:"sourcePath"`
	BaseRevision int64                `json:"baseRevision"`
	BaseHash     string               `json:"baseHash"`
	CopiedAt     string               `json:"copiedAt"`
	Assessment   ProvenanceAssessment `json:"assessment"`
}

// ProvenanceAssessment is the runtime state of one relationship.
type ProvenanceAssessment struct {
	State             string `json:"state"` // up-to-date | source-changed | copy-modified | both-changed | source-missing | copy-missing | unknown
	CanSyncFromSource bool   `json:"canSyncFromSource"`
	Reason            string `json:"reason"`
}

// ProvenanceListResult is the M34 response.
type ProvenanceListResult struct {
	Entries []ProvenanceRecordShape `json:"entries"`
	Failure *Failure                `json:"failure,omitempty"`
}

// ProvenanceStateResult is the M35 response; Entry nil means no relationship.
type ProvenanceStateResult struct {
	Entry   *ProvenanceRecordShape `json:"entry"`
	Failure *Failure               `json:"failure,omitempty"`
}

// AuditChangedEvent is E03: the audit ledger gained records.
type AuditChangedEvent struct {
	VaultID  string `json:"vaultId,omitempty"`
	Sequence int64  `json:"sequence"`
}

// TransactionSummary is one transaction row in the audit panel.
type TransactionSummary struct {
	ID         string `json:"id"`
	PlanID     string `json:"planId"`
	CreatedAt  string `json:"createdAt"`
	Status     string `json:"status"`
	EntryCount int    `json:"entryCount"`
}

// AuditListResult is the M31 response.
type AuditListResult struct {
	Records      []ActionRecord       `json:"records"`
	Transactions []TransactionSummary `json:"transactions"`
	Failure      *Failure             `json:"failure,omitempty"`
}

// RestorePreviewEntry is one row of a transaction preview.
type RestorePreviewEntry struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Restorable bool   `json:"restorable"`
	Note       string `json:"note,omitempty"`
}

// RestorePreview is the M32 response.
type RestorePreview struct {
	TransactionID string                `json:"transactionId"`
	VaultID       string                `json:"vaultId"`
	Entries       []RestorePreviewEntry `json:"entries"`
	Failure       *Failure              `json:"failure,omitempty"`
}

// RestoreResult is the M33 response.
type RestoreResult struct {
	TransactionID      string   `json:"transactionId"`
	Restored           []string `json:"restored"`
	Conflicted         []string `json:"conflicted"`
	TouchedDirectories []string `json:"touchedDirectories"`
	Failure            *Failure `json:"failure,omitempty"`
}

// ActionRecord is one executed (or skipped) action (API.md M30/M31).
type ActionRecord struct {
	ID            string `json:"id"`
	PlanID        string `json:"planId"`
	VaultID       string `json:"vaultId"`
	Origin        string `json:"origin"`
	Kind          string `json:"kind"`
	Source        string `json:"source,omitempty"`
	Target        string `json:"target,omitempty"`
	BeforeHash    string `json:"beforeHash,omitempty"`
	AfterHash     string `json:"afterHash,omitempty"`
	Status        string `json:"status"` // planned | approved | completed | failed | reverted | conflicted | skipped
	TransactionID string `json:"transactionId,omitempty"`
	ErrorCode     string `json:"errorCode,omitempty"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	CreatedAt     string `json:"createdAt"`
	CompletedAt   string `json:"completedAt,omitempty"`
}

// MutationReport is the M30 response: completed, failed and skipped separately
// so the UI can show what is left (PRD 8.3).
type MutationReport struct {
	PlanID             string         `json:"planId"`
	Status             string         `json:"status"` // completed | partial | failed
	Completed          []ActionRecord `json:"completed"`
	Failed             []ActionRecord `json:"failed"`
	Remaining          []ActionRecord `json:"remaining"`
	TransactionID      string         `json:"transactionId,omitempty"`
	TouchedDirectories []string       `json:"touchedDirectories"`
	Failure            *Failure       `json:"failure,omitempty"`
}

// Mutation kinds (shared/mutation.ts MUTATION_KINDS).
const (
	MutCreate          = "create"
	MutCreateDirectory = "createDirectory"
	MutEdit            = "edit"
	MutMove            = "move"
	MutCopy            = "copy"
	MutArchive         = "archive"
	MutTrash           = "trash"
	MutPermanentDelete = "permanentDelete"
)

// MutationAction is one step of a plan (API.md M29).
type MutationAction struct {
	ID                     string  `json:"id"`
	Kind                   string  `json:"kind"`
	Source                 string  `json:"source,omitempty"`
	Target                 string  `json:"target,omitempty"`
	ExpectedSourceRevision *int64  `json:"expectedSourceRevision,omitempty"`
	ExpectedTargetRevision *int64  `json:"expectedTargetRevision,omitempty"`
	Text                   *string `json:"text,omitempty"`
	Reason                 string  `json:"reason"`
}

// MutationPlan is what the plan card shows and what execute receives.
type MutationPlan struct {
	ID            string           `json:"id"`
	VaultID       string           `json:"vaultId"`
	Origin        string           `json:"origin"`
	Actions       []MutationAction `json:"actions"`
	Risk          string           `json:"risk"` // low | medium | high
	FreshApproval bool             `json:"freshApproval,omitempty"`
}

// PlanRequest is the M29 input.
type PlanRequest struct {
	VaultID       string           `json:"vaultId"`
	Origin        string           `json:"origin"`
	Actions       []MutationAction `json:"actions"`
	Title         string           `json:"title,omitempty"`
	FreshApproval bool             `json:"freshApproval,omitempty"`
}

// PlanResult is the M20–M23 and M29 response.
type PlanResult struct {
	Plan    *MutationPlan `json:"plan,omitempty"`
	Failure *Failure      `json:"failure,omitempty"`
}

// SearchFilters narrows a search (API.md M24).
type SearchFilters struct {
	Areas         []string `json:"areas,omitempty"`
	Extensions    []string `json:"extensions,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	UpdatedAfter  int64    `json:"updatedAfter,omitempty"` // ms since epoch
	IncludeSystem bool     `json:"includeSystem,omitempty"`
}

// SearchQuery is the M24 request.
type SearchQuery struct {
	VaultID string         `json:"vaultId"`
	Text    string         `json:"text"`
	Filters *SearchFilters `json:"filters,omitempty"`
	Limit   int            `json:"limit,omitempty"`
	Sort    string         `json:"sort,omitempty"` // relevance | updated | path
}

// SearchHit is one result row.
type SearchHit struct {
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	Area          string   `json:"area"`
	MtimeMs       int64    `json:"mtimeMs"`
	Size          int64    `json:"size"`
	Score         float64  `json:"score"`
	MatchedFields []string `json:"matchedFields"`
	Excerpt       string   `json:"excerpt"`
	Tags          []string `json:"tags"`
	Ignored       bool     `json:"ignored"`
}

// SearchResult is the M24 response.
type SearchResult struct {
	VaultID string      `json:"vaultId"`
	Query   string      `json:"query"`
	Hits    []SearchHit `json:"hits"`
	Total   int         `json:"total"`
	Partial bool        `json:"partial"`
	TookMs  int64       `json:"tookMs"`
	Failure *Failure    `json:"failure,omitempty"`
}

// IndexStatus is what IndexService.Status (M25) returns and what the
// index-status event (E02) carries.
type IndexStatus struct {
	VaultID       string   `json:"vaultId,omitempty"`
	Phase         string   `json:"phase"` // idle | scanning | ready | error
	DocumentCount int      `json:"documentCount"`
	TokenCount    int      `json:"tokenCount"`
	Scanned       int      `json:"scanned"`
	Skipped       int      `json:"skipped"`
	Revision      int64    `json:"revision"`
	LastBuiltAt   string   `json:"lastBuiltAt,omitempty"`
	ErrorMessage  string   `json:"errorMessage,omitempty"`
	Failure       *Failure `json:"failure,omitempty"`
}

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
