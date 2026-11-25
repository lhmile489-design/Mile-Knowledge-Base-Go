package service

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// scanCacheFreshFor breaks the E12 loop: a scan that just finished writes the
// cache, the page refreshes, and asking again inside this window must not
// start another scan (nor report scanning=true, which used to stick).
const scanCacheFreshFor = 2 * time.Second

// SkillService is the binding layer of R2 skill management (M4, API.md §12).
// T04 wires the read side: the tool registry (M76) and the scan (M77), with a
// background refresh and E12. Write operations arrive in T05–T08.
type SkillService struct {
	app          *App
	mu           sync.Mutex
	scanning     bool
	seq          int64
	now          func() time.Time
	detector     func() *skills.Detector
	githubClient func(version string) *skills.GithubClient
	openDialog   func(ctx context.Context, title string, extensions []string) ([]string, error)
	// checking guards one update round at a time (M93).
	checking bool
	// rateLimitResetAt remembers the quota reset of the last round (M93).
	rateLimitResetAt string
	// targetPicks holds M98 folder tokens (10 minutes, cleared on save).
	targetPicksMu sync.Mutex
	targetPicks   map[string]targetPick
	// opMu serializes write operations so a registration cannot be removed
	// while a distribute/uninstall/install is in flight (技术设计 17.6).
	opMu sync.Mutex
	// manifestStale remembers a skipped vault-manifest write so M102 can
	// report it (API §15.3); guarded by mu.
	manifestStale bool
	// scanGen counts scan-cache invalidations; a scan that began before one
	// does not write its result into the cache (guarded by mu).
	scanGen int64
	// beforeSaveScan is a test seam between building a scan and caching it.
	beforeSaveScan func()
}

// NewSkillService wires the service; tests replace detector and now.
func NewSkillService(app *App) *SkillService {
	svc := newSkillService(app)
	app.OnSkillsIndexed = svc.indexUpdated
	return svc
}

// indexUpdated runs after the index took in a change under skills/: the cached
// scan was built from the older index, so it is dropped and the page is told
// to scan again.
func (s *SkillService) indexUpdated(string) {
	s.dropScanCache()
	s.emitChanged()
}

// dropScanCache invalidates the cached scan: whatever is on disk goes, and a
// scan already running will not put its older result back.
func (s *SkillService) dropScanCache() {
	s.mu.Lock()
	s.scanGen++
	s.mu.Unlock()
	_ = os.Remove(s.cachePath())
}

func newSkillService(app *App) *SkillService {
	return &SkillService{
		app:          app,
		now:          time.Now,
		detector:     skills.NewSystemDetector,
		githubClient: func(version string) *skills.GithubClient { return skills.NewGithubClient(version) },
		openDialog:   autoOpenDialog,
		targetPicks:  map[string]targetPick{},
	}
}

func (s *SkillService) cachePath() string {
	return filepath.Join(s.app.DataDir, "skills", "scan-cache.json")
}

// Registry is M76: the built-in tools, the registered custom tools and
// projects plus detection, with redacted paths (API §14.1).
func (s *SkillService) Registry(ctx context.Context) contract.SkillRegistryResult {
	detector := s.detector()
	tools := s.skillTools()
	out := make([]contract.SkillTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, contract.SkillTool{
			ID:         tool.ID,
			Name:       tool.Name,
			IconKey:    tool.IconKey,
			InstallDir: maskPath(tool.InstallDir, detector),
			ScanDirs:   maskPaths(tool.ScanDirs, detector),
			Readers:    tool.Readers,
			Status:     tool.Status,
			Kind:       tool.Kind,
			Missing:    tool.Missing,
			InstallRel: tool.InstallRel,
			Official:   tool.Official,
			Enabled:    tool.Enabled,
		})
	}
	return contract.SkillRegistryResult{Tools: out}
}

// Scan is M77: cached-first with a background refresh (API.md §12.2).
func (s *SkillService) Scan(ctx context.Context, vaultID string) contract.SkillScanResult {
	if vaultID != "" {
		if f := validateID(vaultID); f != nil {
			return contract.SkillScanResult{Failure: f}
		}
		if _, f := s.app.Vaults.Root(vaultID); f != nil {
			return contract.SkillScanResult{Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
		}
	}
	cachePath := s.cachePath()
	if cached := skills.LoadScanCache(cachePath); cached != nil {
		if info, err := os.Stat(cachePath); err == nil && time.Since(info.ModTime()) < scanCacheFreshFor {
			// Just scanned (usually the E12 refresh); answer from the cache.
			return contract.SkillScanResult{Scan: cached.ToScan(false)}
		}
		s.startBackgroundScan(vaultID, cachePath)
		return contract.SkillScanResult{Scan: cached.ToScan(true)}
	}
	// First open: scan synchronously so the panel is not empty.
	scan := s.runScan(ctx, vaultID)
	return contract.SkillScanResult{Scan: scan}
}

func (s *SkillService) startBackgroundScan(vaultID, cachePath string) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return
	}
	s.scanning = true
	s.mu.Unlock()
	go func() {
		defer func() {
			s.mu.Lock()
			s.scanning = false
			s.mu.Unlock()
		}()
		s.runScan(context.Background(), vaultID)
		s.seq++
		if window := s.app.Window(); window != nil {
			_ = SkillsChanged.Emit(window, contract.SkillsChangedEvent{Sequence: s.seq})
		}
	}()
}

// runScan builds the payload, writes the cache and returns the fresh scan.
func (s *SkillService) runScan(ctx context.Context, vaultID string) contract.SkillScan {
	s.mu.Lock()
	gen := s.scanGen
	s.mu.Unlock()
	detector := s.detector()
	loaded := s.loadTargets()
	tools := skills.MergeTargets(detector.Detect(), loaded.Targets)
	state := skills.LoadState(filepath.Join(s.app.DataDir, "skills", "state.json"))
	kb, refs, root := s.kbSkills(ctx, vaultID, state.Distributions)
	groups, warnings := skills.ScanTools(skills.ScanOptions{
		VaultID:       vaultID,
		VaultRoot:     root,
		Kb:            kb,
		KbRefs:        refs,
		Tools:         tools,
		Distributions: state.Distributions,
		ToolSources:   skills.LoadToolSources(filepath.Join(s.app.DataDir, "skills")).Records,
		UpdateCache:   s.updateCache(),
	})
	// Tool paths never leave Go unredacted (API §1, §12, §14).
	for i := range groups {
		for j := range groups[i].Skills {
			skill := &groups[i].Skills[j]
			skill.Path = maskPath(skill.Path, detector)
			skill.Target = maskPath(skill.Target, detector)
		}
	}
	for _, warning := range loaded.Warnings {
		warnings = append(warnings, contract.SkillScanWarning{Text: warning})
	}
	cachedAt := s.now().UTC().Format(time.RFC3339)
	scan := contract.SkillScan{CachedAt: cachedAt, Kb: kb, Tools: groups, Warnings: warnings}
	if s.beforeSaveScan != nil {
		s.beforeSaveScan()
	}
	s.mu.Lock()
	current := s.scanGen == gen
	s.mu.Unlock()
	if !current {
		// Something changed while this scan ran; the E12 that follows the change
		// makes the page scan again, so this result is not cached.
		return scan
	}
	if err := skills.SaveScanCache(s.cachePath(), scan, cachedAt); err != nil {
		scan.Warnings = append(scan.Warnings, contract.SkillScanWarning{Text: "扫描缓存写入失败：" + err.Error()})
	}
	return scan
}

// kbSkills reads the knowledge-base skills from the index (FR-SKM-SCAN); the
// description comes from the SKILL.md frontmatter.
func (s *SkillService) kbSkills(ctx context.Context, vaultID string, distributions []skills.Distribution) ([]contract.SkillCategory, []skills.KbSkillRef, string) {
	empty := []contract.SkillCategory{}
	if vaultID == "" {
		return empty, nil, ""
	}
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return empty, nil, ""
	}
	handle := s.app.indexHandle(vaultID)
	if handle == nil {
		return empty, nil, root
	}
	handle.mu.Lock()
	store := handle.store
	handle.mu.Unlock()
	if store == nil {
		return empty, nil, root
	}
	docs, err := store.SkillDocuments(ctx)
	if err != nil {
		return empty, nil, root
	}
	list := make([]ai.SkillDoc, 0, len(docs))
	for _, doc := range docs {
		list = append(list, ai.SkillDoc{Path: doc.Path, Title: doc.Title})
	}
	byCategory := map[string][]contract.KbSkill{}
	order := []string{}
	var refs []skills.KbSkillRef
	for _, entry := range ai.BuildSkillCatalog(list) {
		dir := path.Dir(entry.Path) // skills/<分类>/<技能>/SKILL.md -> directory
		skill := contract.KbSkill{
			Name:        entry.Name,
			Description: skills.SkillDescription(filepath.Join(root, filepath.FromSlash(dir))),
			Path:        dir,
			Category:    entry.Category,
		}
		for _, dist := range distributions {
			if dist.VaultID != vaultID || dist.SkillPath != dir {
				continue
			}
			out := contract.KbSkillDistribution{ToolID: dist.ToolID, Mode: dist.Mode}
			if dist.Mode == "copy" {
				out.Sync = skills.CopySyncState(root, dist, dist.TargetPath)
			}
			skill.Distributed = append(skill.Distributed, out)
		}
		if _, ok := byCategory[entry.Category]; !ok {
			order = append(order, entry.Category)
		}
		byCategory[entry.Category] = append(byCategory[entry.Category], skill)
		refs = append(refs, skills.KbSkillRef{Category: entry.Category, Name: entry.Name, Path: dir})
	}
	// GitHub provenance lives in the vault (.mile-kb/skill-sources.json) and
	// is merged into the scan payload with the latest check (M5 T04/T08).
	sources := skills.LoadKbSources(root)
	cache := s.updateCache()
	for _, category := range order {
		skills.AttachKbRemotes(root, byCategory[category], sources, cache)
	}
	groups := make([]contract.SkillCategory, 0, len(order))
	for _, category := range order {
		groups = append(groups, contract.SkillCategory{Category: category, Skills: byCategory[category]})
	}
	return groups, refs, root
}

// redactToolPath hides the user's home and local app data (API §1, §12).
func redactToolPath(value string, detector *skills.Detector) string {
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	for _, pair := range []struct {
		prefix string
		label  string
	}{
		{strings.ToLower(detector.LocalAppData), "<local-app-data>"},
		{strings.ToLower(detector.Home), "<home>"},
	} {
		if pair.prefix != "" && strings.HasPrefix(lower, pair.prefix) {
			return pair.label + value[len(pair.prefix):]
		}
	}
	return value
}

func redactToolPaths(values []string, detector *skills.Detector) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, redactToolPath(value, detector))
	}
	return out
}
