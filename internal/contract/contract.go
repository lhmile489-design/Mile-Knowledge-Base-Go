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
	CodeSkillConflict       = "skill-conflict"
	CodeSkillProtected      = "skill-protected"
	CodeBackupMissing       = "backup-missing"
	CodeNetworkFailure      = "network-failure"
	CodeRateLimited         = "rate-limited"
	CodeUnsafeArchive       = "unsafe-archive"
)

// Skills fake-environment switches (技术设计 15.7, API.md §12.5). Only e2e
// builds read them; production ignores them.
const (
	EnvSkillsFakeHome     = "SKILLS_FAKE_HOME"
	EnvSkillsFakePath     = "SKILLS_FAKE_PATH"
	EnvSkillsFakeRegistry = "SKILLS_FAKE_REGISTRY"
	EnvSkillsFakeGithub   = "SKILLS_FAKE_GITHUB" // M5: base URL of the fake GitHub (e2e only)
	EnvSkillsFakeZip      = "SKILLS_FAKE_ZIP"    // M5: zip the picker returns without a dialog (e2e only)
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

// PickSkillSourceResult is M14: the wizard's source step. Candidates are the
// skills found one level under the picked folder (or the folder itself).
type PickSkillSourceResult struct {
	Cancelled  bool             `json:"cancelled"`
	Token      string           `json:"token,omitempty"`
	Candidates []SkillCandidate `json:"candidates"`
	Failure    *Failure         `json:"failure,omitempty"`
}

// SkillCandidate is one importable skill.
type SkillCandidate struct {
	Name      string `json:"name"`
	FileCount int    `json:"fileCount"`
}

// ImportSkillsResult is M15. Reason is exists | unknown | too-large | failed.
type ImportSkillsResult struct {
	Imported []string    `json:"imported"`
	Skipped  []SkillSkip `json:"skipped"`
	Failure  *Failure    `json:"failure,omitempty"`
}

// SkillSkip is one skill the import left alone.
type SkillSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// AboutInfoResult is M74: the about & diagnostics page. Every display value
// here is already redacted (<vault>, <home>); the page never receives an
// absolute path (API §1, D-013).
type AboutInfoResult struct {
	Version      string              `json:"version"`
	TechStack    string              `json:"techStack"`
	Engine       EngineStatus        `json:"engine"`
	Capabilities *EngineCapabilities `json:"capabilities,omitempty"`
	RestartCount int                 `json:"restartCount"`
	Storage      []StorageLocation   `json:"storage"`
	Failure      *Failure            `json:"failure,omitempty"`
}

// StorageLocation is one row of M74's storage list.
type StorageLocation struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Display  string `json:"display"`
	Copyable bool   `json:"copyable"`
}

// CopyStoragePathResult is M75 (Go writes the clipboard, D-013).
type CopyStoragePathResult struct {
	OK      bool     `json:"ok"`
	Failure *Failure `json:"failure,omitempty"`
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

// AccentTokens is the ten theme-color tokens one theme applies (DESIGN.md
// 12.1, API.md §11). Every value is a lowercase #rrggbb string.
type AccentTokens struct {
	Accent    string `json:"accent"`
	Hover     string `json:"hover"`
	Active    string `json:"active"`
	Soft      string `json:"soft"`
	Text      string `json:"text"`
	Btn       string `json:"btn"`
	BtnHover  string `json:"btnHover"`
	BtnActive string `json:"btnActive"`
	OnAccent  string `json:"onAccent"`
	Focus     string `json:"focus"`
}

// AccentCache carries the palettes the frontend derived for a custom color,
// so the next launch can paint the first frame without recomputing.
type AccentCache struct {
	Light AccentTokens `json:"light"`
	Dark  AccentTokens `json:"dark"`
}

// AccentSetting is the theme color (PRD §16, API.md §11).
type AccentSetting struct {
	Preset string       `json:"preset"` // orange | indigo | blue | green | teal | violet | custom
	Hex    string       `json:"hex,omitempty"`
	Cache  *AccentCache `json:"cache,omitempty"`
}

// AppSettings is the whole persisted settings document.
type AppSettings struct {
	Version       int            `json:"version"`
	Theme         string         `json:"theme"`         // light | dark | system
	CloseBehavior string         `json:"closeBehavior"` // ask | tray | quit
	Accent        AccentSetting  `json:"accent"`
	Layout        LayoutSettings `json:"layout"`
	RecentVaults  []Vault        `json:"recentVaults"`
	LastVaultID   string         `json:"lastVaultId,omitempty"`
	Unknown       map[string]any `json:"-"` // fields written by other versions, preserved verbatim
}

// SettingsPatch is a partial update; nil fields are untouched.
type SettingsPatch struct {
	Theme         *string         `json:"theme,omitempty"`
	CloseBehavior *string         `json:"closeBehavior,omitempty"`
	Accent        *AccentSetting  `json:"accent,omitempty"`
	Layout        *LayoutSettings `json:"layout,omitempty"`

	accentUnknown bool
}

// HasUnknownAccentFields reports whether the wire payload carried keys inside
// "accent" or its "cache" that this version does not know. API.md §11.2 makes
// those an invalid-input instead of silently dropping them.
func (p *SettingsPatch) HasUnknownAccentFields() bool { return p.accentUnknown }

// UnmarshalJSON records unknown accent keys; every other field decodes as
// before.
func (p *SettingsPatch) UnmarshalJSON(data []byte) error {
	var wire struct {
		Theme         *string         `json:"theme"`
		CloseBehavior *string         `json:"closeBehavior"`
		Accent        json.RawMessage `json:"accent"`
		Layout        *LayoutSettings `json:"layout"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	p.Theme, p.CloseBehavior, p.Layout = wire.Theme, wire.CloseBehavior, wire.Layout
	p.Accent = nil
	p.accentUnknown = false
	if len(wire.Accent) == 0 || string(wire.Accent) == "null" {
		return nil
	}
	var a AccentSetting
	if err := json.Unmarshal(wire.Accent, &a); err != nil {
		return err
	}
	p.Accent = &a
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(wire.Accent, &keys); err == nil {
		for k := range keys {
			if k != "preset" && k != "hex" && k != "cache" {
				p.accentUnknown = true
			}
		}
	}
	var cache struct {
		Cache json.RawMessage `json:"cache"`
	}
	if err := json.Unmarshal(wire.Accent, &cache); err == nil && len(cache.Cache) > 0 && string(cache.Cache) != "null" {
		var sides map[string]json.RawMessage
		if err := json.Unmarshal(cache.Cache, &sides); err == nil {
			for k := range sides {
				if k != "light" && k != "dark" {
					p.accentUnknown = true
				}
			}
		}
	}
	return nil
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

// DiffLine is one line of an approval-card preview.
type DiffLine struct {
	Kind string `json:"kind"` // same | added | removed
	Text string `json:"text"`
}

// WritePreview is the approval card's write preview (API §6).
type WritePreview struct {
	Kind         string     `json:"kind"` // created | modified | summary
	Lines        []DiffLine `json:"lines,omitempty"`
	OldLineCount int        `json:"oldLineCount,omitempty"`
	NewLineCount int        `json:"newLineCount,omitempty"`
	OldBytes     int64      `json:"oldBytes"`
	NewBytes     int64      `json:"newBytes"`
	Reason       string     `json:"reason,omitempty"` // too-large | not-text | unchanged
}

// Usage is one usage group (turn or cumulative). Fields follow the legacy
// shared/usage.ts:22-50; unknown cost/ratio stay nil and render as an em dash.
type Usage struct {
	TotalTokens      int64    `json:"totalTokens"`
	PromptTokens     int64    `json:"promptTokens"`
	CompletionTokens int64    `json:"completionTokens"`
	ReasoningTokens  int64    `json:"reasoningTokens"`
	CacheHitTokens   int64    `json:"cacheHitTokens"`
	CacheMissTokens  int64    `json:"cacheMissTokens"`
	Estimated        bool     `json:"estimated,omitempty"`
	CacheHitRatio    *float64 `json:"cacheHitRatio,omitempty"`
	EstimatedCost    *float64 `json:"estimatedCost,omitempty"`
	Currency         string   `json:"currency,omitempty"`
	CostComplete     *bool    `json:"costComplete,omitempty"`
	DisplayStatus    string   `json:"displayStatus,omitempty"`
	UsageSource      string   `json:"usageSource,omitempty"`
}

// ProviderDraft is one providers.json entry as the settings page sees it
// (API M67/M68). The key value is never part of it.
type ProviderDraft struct {
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	BaseURL       string   `json:"baseUrl"`
	Models        []string `json:"models"`
	DefaultModel  string   `json:"defaultModel"`
	APIKeyEnv     string   `json:"apiKeyEnv"`
	ContextWindow *int     `json:"contextWindow,omitempty"`
}

// ProviderCredentialState says whether a key is configured, never what it is.
type ProviderCredentialState struct {
	EnvName string `json:"envName"`
	IsSet   bool   `json:"isSet"`
}

// ProvidersResult is the M67/M68/M69 payload.
type ProvidersResult struct {
	Providers   []ProviderDraft           `json:"providers"`
	Credentials []ProviderCredentialState `json:"credentials"`
	Failure     *Failure                  `json:"failure,omitempty"`
}

// FetchModelsResult is the M70 payload.
type FetchModelsResult struct {
	Models  []string `json:"models"`
	Failure *Failure `json:"failure,omitempty"`
}

// AppConfigResult is the M62/M66 response: vault-level settings the UI reads.
type AppConfigResult struct {
	VaultID    string                 `json:"vaultId,omitempty"`
	Aliases    map[string]string      `json:"aliases"`
	Categories []CategoryConfig       `json:"categories"`
	Agents     map[string]AgentConfig `json:"agents,omitempty"`
	Failure    *Failure               `json:"failure,omitempty"`
}

// CategoryConfig is one categories.json entry as the settings page sees it
// (M65). The Go side drops invalid rows instead of reporting them.
type CategoryConfig struct {
	Directory string  `json:"directory"`
	Alias     string  `json:"alias"`
	Area      *string `json:"area"`
	Indexed   bool    `json:"indexed"`
	Order     int     `json:"order"`
}

// AgentConfig is one agents.json entry as the settings page sees it (M63).
type AgentConfig struct {
	Model               string   `json:"model"`
	Effort              string   `json:"effort"`
	Approval            string   `json:"approval"`
	SkillPaths          []string `json:"skillPaths"`
	Steering            string   `json:"steering"`
	SteeringBaseVersion int      `json:"steeringBaseVersion"`
}

// AgentPatch carries only the keys the caller wants to change (M63).
type AgentPatch struct {
	Model      *string   `json:"model,omitempty"`
	Effort     *string   `json:"effort,omitempty"`
	Approval   *string   `json:"approval,omitempty"`
	SkillPaths *[]string `json:"skillPaths,omitempty"`
	Steering   *string   `json:"steering,omitempty"`
}

// SkillEntry is one assignable skill (M71).
type SkillEntry struct {
	Path     string `json:"path"`
	Category string `json:"category"`
	Name     string `json:"name"`
}

// SkillCatalogResult is the M71 payload.
type SkillCatalogResult struct {
	Skills  []SkillEntry `json:"skills"`
	Failure *Failure     `json:"failure,omitempty"`
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
	MutImportFile      = "importFile" // M5: write one file staged under skills/tmp (user plans only)
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
	// importFile only (API §13.4): the staged file this action writes.
	StageID   string `json:"stageId,omitempty"`
	StagePath string `json:"stagePath,omitempty"`
	StageHash string `json:"stageHash,omitempty"`
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

// ---- Shared context (API.md M45–M49) ----

// VaultContext mirrors shared/context.ts:85-91: the label is the display name,
// never an absolute path.
type VaultContext struct {
	ID            string `json:"id"`
	RootLabel     string `json:"rootLabel"`
	Revision      int64  `json:"revision"`
	IndexRevision int64  `json:"indexRevision"`
}

// ActiveDocumentContext is what the user is looking at right now.
type ActiveDocumentContext struct {
	Path      string `json:"path"`
	Revision  int64  `json:"revision"`
	Dirty     bool   `json:"dirty"`
	Selection string `json:"selection,omitempty"` // ≤200 characters
}

// Reference is one user-pinned file. ID is opaque and never encodes the path.
type Reference struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Revision int64  `json:"revision"`
	Note     string `json:"note,omitempty"` // ≤500 characters
	PinnedAt string `json:"pinnedAt"`
}

// TaskStatus values (shared/context.ts:21).
const (
	TaskQueued    = "queued"
	TaskRunning   = "running"
	TaskBlocked   = "blocked"
	TaskDone      = "done"
	TaskCancelled = "cancelled"
)

// TaskItem is one handoff task on the shared board.
type TaskItem struct {
	ID          string   `json:"id"`
	Owner       string   `json:"owner"`
	Status      string   `json:"status"`
	Goal        string   `json:"goal"`
	Inputs      []string `json:"inputs"`
	Constraints []string `json:"constraints"`
	Outputs     []string `json:"outputs"`
	CreatedBy   string   `json:"createdBy"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// Decision is one confirmed decision on the shared log.
type Decision struct {
	ID        string   `json:"id"`
	Summary   string   `json:"summary"`
	DecidedBy string   `json:"decidedBy"` // agent id or "user"
	DecidedAt string   `json:"decidedAt"`
	Paths     []string `json:"paths,omitempty"`
}

// ContextActionSummary is the trimmed action record injected into context.
type ContextActionSummary struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Source string `json:"source,omitempty"`
	Target string `json:"target,omitempty"`
	Status string `json:"status"`
	At     string `json:"at"`
}

// RetrievalItem is one retrieved snippet with its relevance and reason.
type RetrievalItem struct {
	ID       string  `json:"id"`
	Path     string  `json:"path"`
	Revision int64   `json:"revision"`
	Excerpt  string  `json:"excerpt"`
	Score    float64 `json:"score"`
	Reason   string  `json:"reason"`
	Area     string  `json:"area"`
	Stale    bool    `json:"stale,omitempty"`
}

// SharedContext is M45's snapshot; E09 broadcasts the same shape.
type SharedContext struct {
	SnapshotID       string                 `json:"snapshotId"`
	Sequence         int64                  `json:"sequence"`
	Vault            VaultContext           `json:"vault"`
	ActiveDocument   *ActiveDocumentContext `json:"activeDocument,omitempty"`
	PinnedReferences []Reference            `json:"pinnedReferences"`
	TaskBoard        []TaskItem             `json:"taskBoard"`
	DecisionLog      []Decision             `json:"decisionLog"`
	ActionLedger     []ContextActionSummary `json:"actionLedger"`
	RetrievalBundle  []RetrievalItem        `json:"retrievalBundle"`
}

// SharedContextResult is the M45–M48 reply.
type SharedContextResult struct {
	Context *SharedContext `json:"context,omitempty"`
	Failure *Failure       `json:"failure,omitempty"`
}

// ---- Conversations (API.md M50–M59, 数据库设计 4.4) ----

// ConversationMeta is one row of index.json.
type ConversationMeta struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
	TurnCount int      `json:"turnCount"`
	Agents    []string `json:"agents"`
	Pinned    bool     `json:"pinned"`
}

// ConversationIndex is index.json; ActiveID nil means null.
type ConversationIndex struct {
	ActiveID      *string            `json:"activeId"`
	Conversations []ConversationMeta `json:"conversations"`
}

// ConversationEvent is one jsonl line. Text/ID/Title/Status/StopReason/Message
// are pointers so each event type serialises only its own fields while keeping
// the legacy presence checks on read.
type ConversationEvent struct {
	Type       string  `json:"t"` // user|message|thought|tool|turn-end|error
	At         string  `json:"at"`
	Agent      string  `json:"agent"`
	Text       *string `json:"text,omitempty"`
	ID         *string `json:"id,omitempty"`
	Title      *string `json:"title,omitempty"`
	Kind       string  `json:"kind,omitempty"`
	Status     *string `json:"status,omitempty"`
	Detail     string  `json:"detail,omitempty"`
	StopReason *string `json:"stopReason,omitempty"`
	Message    *string `json:"message,omitempty"`
}

// ConversationListResult is M50.
type ConversationListResult struct {
	Index   *ConversationIndex `json:"index,omitempty"`
	Failure *Failure           `json:"failure,omitempty"`
}

// ConversationMetaResult is M52–M54.
type ConversationMetaResult struct {
	Conversation *ConversationMeta `json:"conversation,omitempty"`
	Failure      *Failure          `json:"failure,omitempty"`
}

// ConversationReadResult is M51 and M56.
type ConversationReadResult struct {
	Meta      *ConversationMeta   `json:"meta,omitempty"`
	Events    []ConversationEvent `json:"events,omitempty"`
	Skipped   int                 `json:"skipped"`
	Truncated bool                `json:"truncated"`
	Failure   *Failure            `json:"failure,omitempty"`
}

// ConversationSearchHit is one M57 hit.
type ConversationSearchHit struct {
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
	Excerpt        string `json:"excerpt"`
	MatchedAt      string `json:"matchedAt"`
}

// ConversationSearchResult is M57.
type ConversationSearchResult struct {
	VaultID string                  `json:"vaultId"`
	Query   string                  `json:"query"`
	Hits    []ConversationSearchHit `json:"hits"`
	Failure *Failure                `json:"failure,omitempty"`
}

// ConversationExportResult is M58; Saved false means the user cancelled.
type ConversationExportResult struct {
	Saved   bool     `json:"saved"`
	Name    string   `json:"name"`
	Failure *Failure `json:"failure,omitempty"`
}

// ConversationClearResult is M59.
type ConversationClearResult struct {
	Removed int      `json:"removed"`
	Failure *Failure `json:"failure,omitempty"`
}

// ---- Agent engine (API.md §4.6, §6) ----

// AgentModelOption is one selectable model in the settings dropdown.
type AgentModelOption struct {
	ModelID     string `json:"modelId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// AgentSessionInfo is M38's session part.
type AgentSessionInfo struct {
	Agent            string             `json:"agent"`
	SessionID        string             `json:"sessionId"`
	VaultID          string             `json:"vaultId"`
	Model            string             `json:"model"`
	Effort           string             `json:"effort"`
	Approval         string             `json:"approval"`
	AvailableModels  []AgentModelOption `json:"availableModels"`
	AvailableEfforts []string           `json:"availableEfforts"` // always empty (D-002)
	Busy             bool               `json:"busy"`
}

// EngineCapabilities is the fixed capability set of the in-process engine.
type EngineCapabilities struct {
	PromptImage            bool    `json:"promptImage"`
	PromptAudio            bool    `json:"promptAudio"`
	PromptEmbeddedContext  bool    `json:"promptEmbeddedContext"`
	ExtensionSurface       bool    `json:"extensionSurface"`
	SteerMethod            *string `json:"steerMethod"`
	ReloadExtensionsMethod *string `json:"reloadExtensionsMethod"`
}

// EngineStatus is M38's engine part and E10's payload.
type EngineStatus struct {
	Phase        string              `json:"phase"` // starting | ready | offline
	Message      string              `json:"message,omitempty"`
	Capabilities *EngineCapabilities `json:"capabilities,omitempty"`
}

// AgentListResult is M38.
type AgentListResult struct {
	Session *AgentSessionInfo `json:"session,omitempty"`
	Engine  *EngineStatus     `json:"engine,omitempty"`
	Failure *Failure          `json:"failure,omitempty"`
}

// PromptResult is M39's return value.
type PromptResult struct {
	StopReason string   `json:"stopReason"`
	Failure    *Failure `json:"failure,omitempty"`
}

// RoutePreviewResult is M44.
type RoutePreviewResult struct {
	Agent        string   `json:"agent"`
	Reason       string   `json:"reason"`
	MatchedTerms []string `json:"matchedTerms"`
}

// ToolCallFrame is one tool call status line (§6).
type ToolCallFrame struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status"` // pending | in-progress | completed | failed
	Detail string `json:"detail,omitempty"`
}

// StatusFrame is a turn-phase or retry hint (§6).
type StatusFrame struct {
	State  string `json:"state"`
	Phase  string `json:"phase,omitempty"`
	Model  string `json:"model,omitempty"`
	Effort string `json:"effort,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// UsageFrame is the per-step usage receipt (§6).
type UsageFrame struct {
	Turn       Usage `json:"turn"`
	Cumulative Usage `json:"cumulative"`
}

// PermFrame is the approval-card lifecycle frame (§6).
type PermFrame struct {
	RequestID  string             `json:"requestId"`
	ToolName   string             `json:"toolName"`
	Detail     string             `json:"detail"`
	TargetPath string             `json:"targetPath,omitempty"`
	Preview    *WritePreview      `json:"preview,omitempty"`
	Approval   string             `json:"approval"`
	Options    []PermissionOption `json:"options"`
	Status     string             `json:"status"` // pending | approved | rejected | expired
}

// StreamFrame is the M39 channel union (§6); the frontend narrows on Kind.
type StreamFrame struct {
	Kind       string         `json:"kind"` // delta | thought | tool | permission | status | usage | turn-end | error
	Seq        int            `json:"seq"`
	Agent      string         `json:"agent"`
	SessionID  string         `json:"sessionId"`
	Text       string         `json:"text,omitempty"`
	ToolCall   *ToolCallFrame `json:"toolCall,omitempty"`
	Permission *PermFrame     `json:"permission,omitempty"`
	Status     *StatusFrame   `json:"status,omitempty"`
	Usage      *UsageFrame    `json:"usage,omitempty"`
	StopReason string         `json:"stopReason,omitempty"`
	Message    string         `json:"message,omitempty"`
	Finish     bool           `json:"finish,omitempty"`
}

// SkillsChangedEvent is E12: background scan or skill operation finished.
type SkillsChangedEvent struct {
	Sequence int64 `json:"sequence"`
}

// ConversationChangedEvent is E07.
type ConversationChangedEvent struct {
	VaultID string `json:"vaultId"`
}

// AppConfigChangedEvent is E08.
type AppConfigChangedEvent struct {
	VaultID string `json:"vaultId"`
}

// PermissionOption is one approval-card button; always allow then reject (§6).
type PermissionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// ---- R2 skills (M4, API.md §12) ----

// SkillTool is one entry of the built-in tool registry plus its detection
// result (FR-SKM-TOOL/DETECT; paths are already redacted for display).
type SkillTool struct {
	ID         string   `json:"id"`      // stable id, e.g. "claude-code", "agents"
	Name       string   `json:"name"`    // display name
	IconKey    string   `json:"iconKey"` // brand svg key; "globe" for the shared dir, "" for a letter placeholder
	InstallDir string   `json:"installDir,omitempty"`
	ScanDirs   []string `json:"scanDirs"`
	Readers    []string `json:"readers,omitempty"` // tools that read the shared ~/.agents/skills
	Status     string   `json:"status"`            // detected | leftover | missing
	Official   bool     `json:"official"`          // false = official support not verified
	Enabled    bool     `json:"enabled"`           // MiniMax Code defaults to false
}

// KbSkillDistribution is one tool this knowledge-base skill reached (M78).
type KbSkillDistribution struct {
	ToolID string `json:"toolId"`
	Mode   string `json:"mode"`           // link | copy
	Sync   string `json:"sync,omitempty"` // copies only
}

// KbSkill is one knowledge-base skill in the scan result.
type KbSkill struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Path        string                `json:"path"`
	Category    string                `json:"category"`
	Distributed []KbSkillDistribution `json:"distributed,omitempty"`
	Remote      *SkillRemote          `json:"remote,omitempty"` // M5: GitHub origin and update state
}

// ToolSkill is one skill found in a tool directory (FR-SKM-SCAN).
type ToolSkill struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Path        string       `json:"path"`
	ToolID      string       `json:"toolId"`
	Origin      string       `json:"origin"`               // linked | link-broken | copied | independent | builtin | external-link
	Sync        string       `json:"sync,omitempty"`       // in-sync | vault-updated | tool-updated | both-updated | missing
	Target      string       `json:"target,omitempty"`     // redacted target of a link
	SourcePath  string       `json:"sourcePath,omitempty"` // copied: the vault skill it came from
	Builtin     bool         `json:"builtin,omitempty"`
	Base        string       `json:"base,omitempty"`   // install | scan:<n> | builtin:<n> (API §12.3)
	Rel         string       `json:"rel,omitempty"`    // path inside that root, slash-separated
	Remote      *SkillRemote `json:"remote,omitempty"` // M5: GitHub origin and update state
}

// SkillCategory groups knowledge-base skills by category.
type SkillCategory struct {
	Category string    `json:"category"`
	Skills   []KbSkill `json:"skills"`
}

// SkillToolScan groups tool skills by tool.
type SkillToolScan struct {
	ToolID string      `json:"toolId"`
	Skills []ToolSkill `json:"skills"`
}

// SkillScanWarning is a degraded scan step (unreadable dir, cache rebuilt).
type SkillScanWarning struct {
	ToolID string `json:"toolId,omitempty"`
	Text   string `json:"text"`
}

// SkillScan is the M77 payload: cached first, refreshed in the background.
type SkillScan struct {
	CachedAt string             `json:"cachedAt"`
	Scanning bool               `json:"scanning"`
	Kb       []SkillCategory    `json:"kb"`
	Tools    []SkillToolScan    `json:"tools"`
	Warnings []SkillScanWarning `json:"warnings,omitempty"`
}

// SkillPreviewItem is one target of a distribute preview (M78).
type SkillPreviewItem struct {
	TargetID   string `json:"targetId"`
	ToolID     string `json:"toolId"`
	Name       string `json:"name"`
	Action     string `json:"action"` // create | exists | overwrite | conflict | duplicate | protected
	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// SkillOpItem is one target's result of a distribute/uninstall (M79/M81).
type SkillOpItem struct {
	TargetID string `json:"targetId"`
	ToolID   string `json:"toolId"`
	Result   string `json:"result"` // linked | copied | skipped | failed
	Detail   string `json:"detail,omitempty"`
	BackupID string `json:"backupId,omitempty"`
}

// SkillUninstallPreview is M80's answer before removing anything.
type SkillUninstallPreview struct {
	Kind          string   `json:"kind"` // link | copy | independent
	Target        string   `json:"target,omitempty"`
	BackupPath    string   `json:"backupPath,omitempty"`
	AffectedTools []string `json:"affectedTools,omitempty"`
}

// SkillLogRecord is one line of skills/log.jsonl (FR-SKM-LOG).
type SkillLogRecord struct {
	ID         string `json:"id"`
	At         string `json:"at"`
	Op         string `json:"op"` // distribute | uninstall | overwrite | adopt | restore | clean
	Skill      string `json:"skill"`
	ToolID     string `json:"toolId,omitempty"`
	Location   string `json:"location"`
	Mode       string `json:"mode,omitempty"` // link | copy | copy-adopt
	Result     string `json:"result"`         // ok | partial | failed
	BackupID   string `json:"backupId,omitempty"`
	Restorable bool   `json:"restorable"`
	Error      string `json:"error,omitempty"`
}

// SkillBackup is one entry of skills/backups (Q-SKM-3).
type SkillBackup struct {
	ID           string `json:"id"`
	ToolID       string `json:"toolId"`
	OriginalPath string `json:"originalPath"`
	Kind         string `json:"kind"` // uninstall | overwrite | adopt
	CreatedAt    string `json:"createdAt"`
	Size         int64  `json:"size"`
	Files        int    `json:"files"`
	Restorable   bool   `json:"restorable"`
}

// ---- R2 install sources and updates (M5, API.md §13) ----

// SkillOrigin is where a skill came from in a GitHub repository.
type SkillOrigin struct {
	Owner  string `json:"owner"`
	Repo   string `json:"repo"`
	Ref    string `json:"ref"`
	Path   string `json:"path"`   // repository sub-path; "" means the repository root
	Commit string `json:"commit"` // head commit the download was pinned to
}

// SkillRemote is the M5 source record plus its latest checked update state.
type SkillRemote struct {
	SkillOrigin
	PathCommit       string `json:"pathCommit"`
	InstalledAt      string `json:"installedAt"`
	UpdateState      string `json:"updateState"` // up-to-date | update-available | both-changed | local-modified | unchecked | error
	LatestPathCommit string `json:"latestPathCommit,omitempty"`
	CheckedAt        string `json:"checkedAt,omitempty"`
	Error            string `json:"error,omitempty"`
}

// SkillStageCandidate is one skill found inside a staged download.
type SkillStageCandidate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Files       int    `json:"files"`
	Bytes       int64  `json:"bytes"`
	State       string `json:"state"` // ok | exists | too-large | duplicate | invalid-name
	Existing    string `json:"existing,omitempty"`
}

// SkillStage is the M88/M89 answer: a staged download and its candidates.
type SkillStage struct {
	StageID    string                `json:"stageId"`
	Kind       string                `json:"kind"` // github | zip
	Origin     *SkillOrigin          `json:"origin,omitempty"`
	Label      string                `json:"label"`
	Candidates []SkillStageCandidate `json:"candidates"`
	Skipped    []SkillStageSkip      `json:"skipped,omitempty"`
	ExpiresAt  string                `json:"expiresAt"`
}

// SkillStageSkip is one entry the unpacker refused to create.
type SkillStageSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"` // symlink | special
}

// SkillStageResult is M88/M89.
type SkillStageResult struct {
	Stage     *SkillStage `json:"stage,omitempty"`
	Cancelled bool        `json:"cancelled,omitempty"`
	Failure   *Failure    `json:"failure,omitempty"`
}

// SkillInstallTarget chooses where M91 installs (API §13.1).
type SkillInstallTarget struct {
	Kind     string                  `json:"kind"` // kb | tools
	Category string                  `json:"category,omitempty"`
	Targets  []SkillDistributeTarget `json:"targets,omitempty"`
}

// SkillUpdateCheckResult is M93.
type SkillUpdateCheckResult struct {
	CheckedAt        string `json:"checkedAt"`
	Checking         bool   `json:"checking"`
	Groups           int    `json:"groups"`
	Unchecked        int    `json:"unchecked"`
	RateLimitResetAt string `json:"rateLimitResetAt,omitempty"`
}

// SkillUpdateTarget is one skill M94 previews an update for.
type SkillUpdateTarget struct {
	Kind      string `json:"kind"` // kb | tool
	SkillPath string `json:"skillPath,omitempty"`
	ToolID    string `json:"toolId,omitempty"`
	Path      string `json:"path,omitempty"` // wire spec (§12.3)
}

// SkillUpdateFile is one entry of the update diff.
type SkillUpdateFile struct {
	Path   string `json:"path"`
	Change string `json:"change"` // added | modified | removed
	Bytes  int64  `json:"bytes"`
	Binary bool   `json:"binary,omitempty"`
}

// SkillUpdatePreviewResult is M94.
type SkillUpdatePreviewResult struct {
	StageID    string            `json:"stageId,omitempty"`
	From       string            `json:"from,omitempty"`
	To         string            `json:"to,omitempty"`
	Files      []SkillUpdateFile `json:"files,omitempty"`
	Plan       *MutationPlan     `json:"plan,omitempty"`
	BackupPath string            `json:"backupPath,omitempty"`
	Failure    *Failure          `json:"failure,omitempty"`
}

// SkillConfirmUpdateResult is M95.
type SkillConfirmUpdateResult struct {
	Updated bool     `json:"updated"`
	Failure *Failure `json:"failure,omitempty"`
}

// SkillStageProgressEvent is E13: download/unpack progress (≤ 1 frame / 200 ms).
type SkillStageProgressEvent struct {
	RequestID string `json:"requestId"`
	Phase     string `json:"phase"` // resolving | downloading | unpacking | scanning
	Received  int64  `json:"received"`
	Total     int64  `json:"total,omitempty"`
}

// SkillRegistryResult is M76.
type SkillRegistryResult struct {
	Tools   []SkillTool `json:"tools"`
	Failure *Failure    `json:"failure,omitempty"`
}

// SkillScanResult is M77.
type SkillScanResult struct {
	Scan    SkillScan `json:"scan"`
	Failure *Failure  `json:"failure,omitempty"`
}

// SkillPreviewResult is M78.
type SkillPreviewResult struct {
	Items   []SkillPreviewItem `json:"items"`
	Failure *Failure           `json:"failure,omitempty"`
}

// SkillOpResult is M79 and M81.
type SkillOpResult struct {
	Items   []SkillOpItem `json:"items"`
	Failure *Failure      `json:"failure,omitempty"`
}

// SkillUninstallResult is M80.
type SkillUninstallResult struct {
	Preview SkillUninstallPreview `json:"preview"`
	Failure *Failure              `json:"failure,omitempty"`
}

// SkillAdoptResult is M82.
type SkillAdoptResult struct {
	VaultPath string   `json:"vaultPath"`
	Linked    bool     `json:"linked"`
	BackupIDs []string `json:"backupIds,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Failure   *Failure `json:"failure,omitempty"`
}

// SkillRestoreResult is M83.
type SkillRestoreResult struct {
	RestoredTo string   `json:"restoredTo"`
	Failure    *Failure `json:"failure,omitempty"`
}

// SkillLogResult is M84.
type SkillLogResult struct {
	Records []SkillLogRecord `json:"records"`
	Failure *Failure         `json:"failure,omitempty"`
}

// SkillBackupsResult is M85/M86.
type SkillBackupsResult struct {
	Items   []SkillBackup `json:"items"`
	Failure *Failure      `json:"failure,omitempty"`
}

// SkillDistributeTarget is one destination of M78/M79.
type SkillDistributeTarget struct {
	TargetID       string `json:"targetId"`
	AllowOverwrite bool   `json:"allowOverwrite,omitempty"`
}

// SkillToolRef is one extra tool copy of the same skill (M82 extras).
type SkillToolRef struct {
	ToolID string `json:"toolId"`
	Path   string `json:"path"`
}
