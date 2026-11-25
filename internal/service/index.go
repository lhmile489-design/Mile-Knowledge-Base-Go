package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/index"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// IndexStatusEvent is E02: the index phase or progress changed.
var IndexStatusEvent = mygo.NewEvent[contract.IndexStatus]("index-status")

// IndexService exposes the search index (API.md M25, M26).
type IndexService struct{ app *App }

// NewIndexService builds the index service.
func NewIndexService(app *App) *IndexService { return &IndexService{app: app} }

// Status returns the current index state for a vault (M25).
func (s *IndexService) Status(ctx context.Context, vaultID string) contract.IndexStatus {
	if f := validateID(vaultID); f != nil {
		return contract.IndexStatus{Failure: contract.Fail(contract.CodeInvalidInput, "知识库 id 不合法")}
	}
	return s.app.indexStatus(vaultID)
}

// Search runs a query against a vault's index (M24). While the first build is
// still running the result is marked partial.
func (s *IndexService) Search(ctx context.Context, query contract.SearchQuery) contract.SearchResult {
	if f := validateID(query.VaultID); f != nil {
		return contract.SearchResult{VaultID: query.VaultID, Hits: []contract.SearchHit{},
			Failure: contract.Fail(contract.CodeInvalidInput, "知识库 id 不合法")}
	}
	a := s.app
	a.mu.Lock()
	handle := a.indexes[query.VaultID]
	a.mu.Unlock()
	if handle == nil {
		if _, f := a.Vaults.Root(query.VaultID); f != nil {
			return contract.SearchResult{VaultID: query.VaultID, Hits: []contract.SearchHit{},
				Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")}
		}
		// Not opened yet: report partial instead of failing the panel.
		return contract.SearchResult{VaultID: query.VaultID, Query: query.Text,
			Hits: []contract.SearchHit{}, Partial: true}
	}
	handle.mu.Lock()
	store := handle.store
	phase := handle.status.Phase
	handle.mu.Unlock()
	if store == nil {
		return contract.SearchResult{VaultID: query.VaultID, Query: query.Text,
			Hits: []contract.SearchHit{}, Partial: true}
	}
	result, err := index.Search(ctx, store, query)
	if err != nil {
		return contract.SearchResult{VaultID: query.VaultID, Query: query.Text,
			Hits:    []contract.SearchHit{},
			Failure: contract.Fail(contract.CodeIOFailure, "检索失败："+err.Error())}
	}
	if phase != "ready" {
		result.Partial = true
	}
	return result
}

// Health returns the knowledge-base health report (M28), cached per index
// revision; partial while the first build is still running.
func (s *IndexService) Health(ctx context.Context, vaultID string) contract.KbHealthReport {
	empty := contract.KbHealthReport{
		VaultID: vaultID, UnresolvedLinks: []contract.UnresolvedLinkItem{},
		Orphans: []contract.OrphanItem{}, Duplicates: []contract.DuplicateNameItem{},
		Untitled: []contract.UntitledItem{}, Partial: true,
	}
	if f := validateID(vaultID); f != nil {
		empty.Failure = contract.Fail(contract.CodeInvalidInput, "知识库 id 不合法")
		return empty
	}
	handle := s.app.indexHandle(vaultID)
	if handle == nil {
		if _, f := s.app.Vaults.Root(vaultID); f != nil {
			empty.Failure = contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		}
		return empty
	}
	handle.mu.Lock()
	store := handle.store
	phase := handle.status.Phase
	cached := handle.healthReport
	cachedRev := handle.healthRev
	handle.mu.Unlock()
	if store == nil {
		empty.Failure = contract.Fail(contract.CodeIOFailure, "索引不可用")
		return empty
	}

	revision, err := store.Revision(ctx)
	if err != nil {
		empty.Failure = contract.Fail(contract.CodeIOFailure, "索引不可用")
		return empty
	}
	if cached != nil && cachedRev == revision {
		report := *cached
		report.Partial = phase != "ready"
		return report
	}

	unresolved, orphans, duplicates, untitled, rev, err := index.Health(ctx, store)
	if err != nil {
		empty.Failure = contract.Fail(contract.CodeIOFailure, "读取健康报告失败")
		return empty
	}
	report := contract.KbHealthReport{
		VaultID:         vaultID,
		UnresolvedLinks: unresolved,
		Orphans:         orphans,
		Duplicates:      duplicates,
		Untitled:        untitled,
		IndexRevision:   rev,
		Partial:         phase != "ready",
	}
	handle.mu.Lock()
	handle.healthReport = &report
	handle.healthRev = rev
	handle.mu.Unlock()
	return report
}

// Links returns one document's outgoing edges and backlinks (M27).
func (s *IndexService) Links(ctx context.Context, vaultID, path string) contract.LinkGraphResult {
	result := contract.LinkGraphResult{Path: path}
	if f := validateID(vaultID); f != nil {
		result.Failure = contract.Fail(contract.CodeInvalidInput, "知识库 id 不合法")
		return result
	}
	norm, f := pathguard.ValidateRel(path)
	if f != nil {
		result.Failure = f
		return result
	}
	handle := s.app.indexHandle(vaultID)
	if handle == nil {
		result.Failure = contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
		return result
	}
	handle.mu.Lock()
	store := handle.store
	handle.mu.Unlock()
	if store == nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "索引不可用")
		return result
	}
	graph, ok, err := index.Links(ctx, store, norm)
	if err != nil {
		result.Failure = contract.Fail(contract.CodeIOFailure, "读取链接失败")
		return result
	}
	if !ok {
		result.Failure = contract.Fail(contract.CodeNotFound, "文档不在索引中")
		return result
	}
	result.Path = graph.Path
	result.Backlinks = graph.Backlinks
	for _, link := range graph.Outgoing {
		result.Outgoing = append(result.Outgoing, contract.ResolvedLink{
			Target: link.Target, Text: link.Text, Kind: link.Kind,
			Path: link.Path, Candidates: link.Candidates,
		})
	}
	return result
}

// indexHandle returns a vault's index handle, if it has been opened.
func (a *App) indexHandle(vaultID string) *indexHandle {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.indexes[vaultID]
}

// indexResolver returns the resolver for the current index revision, or nil
// when the index is not open yet. Cached per revision.
func (a *App) indexResolver(ctx context.Context, vaultID string) *index.Resolver {
	handle := a.indexHandle(vaultID)
	if handle == nil {
		return nil
	}
	handle.mu.Lock()
	store := handle.store
	handle.mu.Unlock()
	if store == nil {
		return nil
	}
	revision, err := store.Revision(ctx)
	if err != nil {
		return nil
	}
	handle.mu.Lock()
	if handle.resolver != nil && handle.resolverRev == revision {
		resolver := handle.resolver
		handle.mu.Unlock()
		return resolver
	}
	handle.mu.Unlock()

	resolver, err := index.ResolverFor(ctx, store)
	if err != nil {
		return nil
	}
	handle.mu.Lock()
	handle.resolver = resolver
	handle.resolverRev = revision
	handle.mu.Unlock()
	return resolver
}

// Rebuild drops the index file and starts a background rebuild (M26).
func (s *IndexService) Rebuild(ctx context.Context, vaultID string) contract.IndexStatus {
	if f := validateID(vaultID); f != nil {
		return contract.IndexStatus{Failure: contract.Fail(contract.CodeInvalidInput, "知识库 id 不合法")}
	}
	return s.app.indexRebuild(vaultID)
}

// indexHandle owns one vault's index: the store, the last status and the
// goroutine that serialises every write.
type indexHandle struct {
	path string

	mu           sync.Mutex
	store        *index.Store
	status       contract.IndexStatus
	cancel       context.CancelFunc
	updates      chan []string
	resolver     *index.Resolver
	resolverRev  int64
	healthRev    int64
	healthReport *contract.KbHealthReport
}

func indexPath(dataDir, vaultID string) string {
	return filepath.Join(dataDir, "index", vaultID+".db")
}

// startIndex opens (or reuses) a vault's index and starts a first build in the
// background when the database is empty. Safe to call repeatedly.
func (a *App) startIndex(vaultID string) {
	root, f := a.Vaults.Root(vaultID)
	if f != nil {
		return
	}
	a.mu.Lock()
	if a.indexes == nil {
		a.indexes = map[string]*indexHandle{}
	}
	if _, ok := a.indexes[vaultID]; ok {
		a.mu.Unlock()
		return
	}
	handle := &indexHandle{
		path:    indexPath(a.DataDir, vaultID),
		updates: make(chan []string, 64),
		status:  contract.IndexStatus{VaultID: vaultID, Phase: "idle"},
	}
	a.indexes[vaultID] = handle
	a.mu.Unlock()

	store, err := index.OpenIndex(handle.path)
	if err != nil {
		handle.mu.Lock()
		handle.status.Phase = "error"
		handle.status.ErrorMessage = err.Error()
		handle.mu.Unlock()
		a.emitIndexStatus(handle)
		_ = AppNotice.Emit(a.Window(), contract.AppNoticeEvent{Text: "检索索引不可用：" + err.Error(), Level: "warn"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	handle.mu.Lock()
	handle.store = store
	handle.cancel = cancel
	handle.mu.Unlock()

	// Reuse what is on disk: only an empty index needs a full first build.
	if count, err := store.DocumentCount(ctx); err == nil && count > 0 {
		revision, _ := store.Revision(ctx)
		handle.mu.Lock()
		handle.status = contract.IndexStatus{
			VaultID: vaultID, Phase: "ready", DocumentCount: count, Revision: revision,
		}
		handle.mu.Unlock()
		a.emitIndexStatus(handle)
	} else {
		go a.buildIndex(ctx, handle, root)
	}
	go a.indexWorker(ctx, handle, root)
}

// buildIndex runs the full scan in the background and reports progress.
func (a *App) buildIndex(ctx context.Context, handle *indexHandle, root string) {
	handle.mu.Lock()
	store := handle.store
	handle.status.Phase = "scanning"
	handle.mu.Unlock()
	a.emitIndexStatus(handle)

	err := index.Build(ctx, store, root, func(st index.Status) {
		a.updateIndexStatus(handle, st)
	})
	if err != nil && ctx.Err() == nil {
		handle.mu.Lock()
		handle.status.Phase = "error"
		handle.status.ErrorMessage = err.Error()
		handle.mu.Unlock()
		a.emitIndexStatus(handle)
	}
}

// indexWorker serialises incremental updates, one batch at a time.
func (a *App) indexWorker(ctx context.Context, handle *indexHandle, root string) {
	for {
		select {
		case <-ctx.Done():
			return
		case dirs := <-handle.updates:
			handle.mu.Lock()
			store := handle.store
			handle.mu.Unlock()
			if store == nil {
				continue
			}
			if err := index.Update(ctx, store, root, dirs, func(st index.Status) {
				a.updateIndexStatus(handle, st)
			}); err != nil && ctx.Err() == nil {
				handle.mu.Lock()
				handle.status.Phase = "error"
				handle.status.ErrorMessage = err.Error()
				handle.mu.Unlock()
				a.emitIndexStatus(handle)
			} else if err == nil && a.OnSkillsIndexed != nil && touchesSkills(dirs) {
				// The skills page reads the index, so a scan that ran before this
				// batch landed shows the old list (M9 T06, AC-UX-5).
				handle.mu.Lock()
				vaultID := handle.status.VaultID
				handle.mu.Unlock()
				a.OnSkillsIndexed(vaultID)
			}
		}
	}
}

func (a *App) updateIndexStatus(handle *indexHandle, st index.Status) {
	handle.mu.Lock()
	vaultID := handle.status.VaultID
	handle.status = contract.IndexStatus{
		VaultID:       vaultID,
		Phase:         st.Phase,
		DocumentCount: st.DocumentCount,
		TokenCount:    st.TokenCount,
		Scanned:       st.Scanned,
		Skipped:       st.Skipped,
		Revision:      st.Revision,
		LastBuiltAt:   st.LastBuiltAt,
		ErrorMessage:  st.ErrorMessage,
	}
	handle.mu.Unlock()
	a.emitIndexStatus(handle)
	// Keep the shared context's index revision current; the hub skips
	// sequence bumps when the value repeats.
	a.syncContextVault(vaultID)
}

// touchesSkills reports whether a batch of changed directories includes the
// skills area; the empty directory stands for the whole vault.
func touchesSkills(dirs []string) bool {
	for _, dir := range dirs {
		if dir == "" || dir == "skills" || strings.HasPrefix(dir, "skills/") {
			return true
		}
	}
	return false
}

// enqueueIndex hands a watcher batch to the index worker without blocking.
func (a *App) enqueueIndex(vaultID string, dirs []string) {
	a.mu.Lock()
	handle := a.indexes[vaultID]
	a.mu.Unlock()
	if handle == nil {
		return
	}
	select {
	case handle.updates <- dirs:
	default:
		// Overflow: one full pass covers everything that was dropped.
		select {
		case handle.updates <- []string{""}:
		default:
		}
	}
}

// indexStatus returns the last reported status.
func (a *App) indexStatus(vaultID string) contract.IndexStatus {
	a.mu.Lock()
	handle := a.indexes[vaultID]
	a.mu.Unlock()
	if handle == nil {
		if _, f := a.Vaults.Root(vaultID); f != nil {
			return contract.IndexStatus{
				VaultID: vaultID, Phase: "error",
				Failure: contract.Fail(contract.CodeVaultUnavailable, "知识库不可用"),
			}
		}
		return contract.IndexStatus{VaultID: vaultID, Phase: "idle"}
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	return handle.status
}

// indexRebuild deletes the database and starts over.
func (a *App) indexRebuild(vaultID string) contract.IndexStatus {
	a.indexStop(vaultID, true)
	a.startIndex(vaultID)
	return a.indexStatus(vaultID)
}

// indexStop cancels the worker, closes the store and optionally deletes the
// database file (rebuild, vault removal).
func (a *App) indexStop(vaultID string, remove bool) {
	a.mu.Lock()
	handle := a.indexes[vaultID]
	delete(a.indexes, vaultID)
	a.mu.Unlock()

	if handle != nil {
		handle.mu.Lock()
		cancel := handle.cancel
		store := handle.store
		handle.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if store != nil {
			_ = store.Close()
		}
	}
	if remove {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			_ = os.Remove(indexPath(a.DataDir, vaultID) + suffix)
		}
	}
}

func (a *App) emitIndexStatus(handle *indexHandle) {
	win := a.Window()
	if win == nil {
		return
	}
	handle.mu.Lock()
	status := handle.status
	handle.mu.Unlock()
	_ = IndexStatusEvent.Emit(win, status)
}
