package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/index"
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
			Hits: []contract.SearchHit{},
			Failure: contract.Fail(contract.CodeIOFailure, "检索失败："+err.Error())}
	}
	if phase != "ready" {
		result.Partial = true
	}
	return result
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

	mu      sync.Mutex
	store   *index.Store
	status  contract.IndexStatus
	cancel  context.CancelFunc
	updates chan []string
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
