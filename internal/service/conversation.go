package service

import (
	"context"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/conversation"
)

// ConversationService binds the conversation history (API.md M50–M59).
// Recording itself is driven by the turn loop through App.Conversations.
type ConversationService struct {
	app   *App
	store *conversation.Store
}

// NewConversationService binds one app and subscribes the store to E07.
func NewConversationService(app *App) *ConversationService {
	store := app.Conversations
	store.SetOnWarning(func(message string) {
		if window := app.Window(); window != nil {
			_ = AppNotice.Emit(window, contract.AppNoticeEvent{Text: message, Level: "warn"})
		}
	})
	store.SetOnChanged(func(vaultID string) {
		if window := app.Window(); window != nil {
			_ = ConversationChanged.Emit(window, contract.ConversationChangedEvent{VaultID: vaultID})
		}
	})
	return &ConversationService{app: app, store: store}
}

// ensureVault checks the vault id and that the vault is still registered; a
// forgotten vault keeps its history on disk but the binding refuses to use it.
func (s *ConversationService) ensureVault(vaultID string) *contract.Failure {
	if f := validateID(vaultID); f != nil {
		return f
	}
	if _, f := s.app.Vaults.Root(vaultID); f != nil {
		return contract.Fail(contract.CodeVaultUnavailable, "知识库不可用")
	}
	return nil
}

// List is M50.
func (s *ConversationService) List(vaultID string) contract.ConversationListResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationListResult{Failure: f}
	}
	return s.store.List(vaultID)
}

// Get is M51.
func (s *ConversationService) Get(vaultID, id string) contract.ConversationReadResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationReadResult{Failure: f}
	}
	return s.store.Get(vaultID, id)
}

// Create is M52.
func (s *ConversationService) Create(vaultID string) contract.ConversationMetaResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	return s.store.Create(vaultID)
}

// Rename is M53.
func (s *ConversationService) Rename(vaultID, id, title string) contract.ConversationMetaResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	return s.store.Rename(vaultID, id, title)
}

// Pin is M54.
func (s *ConversationService) Pin(vaultID, id string, pinned bool) contract.ConversationMetaResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationMetaResult{Failure: f}
	}
	return s.store.Pin(vaultID, id, pinned)
}

// Remove is M55; the two-step confirmation lives in the frontend.
func (s *ConversationService) Remove(vaultID, id string) contract.Failure {
	if f := s.ensureVault(vaultID); f != nil {
		return *f
	}
	if f := s.store.Remove(vaultID, id); f != nil {
		return *f
	}
	return contract.Failure{}
}

// Activate is M56: make an existing conversation active, return its events and
// prepare the recap block for the next prompt. Switching is refused while an
// agent turn is running (the busy hook is wired by the turn loop in T10).
func (s *ConversationService) Activate(vaultID, id string) contract.ConversationReadResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationReadResult{Failure: f}
	}
	if s.app.AgentBusy != nil && s.app.AgentBusy(vaultID) {
		return contract.ConversationReadResult{Failure: contract.Fail(contract.CodeInvalidInput, "Agent 正在运行，无法切换对话")}
	}
	read := s.store.Get(vaultID, id)
	if read.Failure != nil {
		return read
	}
	if result := s.store.Activate(vaultID, id); result.Failure != nil {
		return contract.ConversationReadResult{Failure: result.Failure}
	}
	if recap := conversation.BuildRecap(read.Events, conversation.RecapMaxChars); recap != "" {
		s.app.SetRecap(vaultID, recap)
	}
	return read
}

// takeRecap hands the recap block to the next prompt and forgets it. It is
// unexported on purpose: the renderer must not be able to consume it.
func (s *ConversationService) takeRecap(vaultID string) string { return s.app.TakeRecap(vaultID) }

// putBackRecap restores a recap when the prompt it was meant for never ran
// (the legacy "failed run puts the recap back" rule).
func (s *ConversationService) putBackRecap(vaultID, recap string) { s.app.PutBackRecap(vaultID, recap) }

// Search is M57.
func (s *ConversationService) Search(vaultID, query string) contract.ConversationSearchResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationSearchResult{VaultID: vaultID, Hits: []contract.ConversationSearchHit{}, Failure: f}
	}
	return s.store.Search(vaultID, query)
}

// Export is M58: the destination comes from the system save dialog; the
// renderer never supplies a path.
func (s *ConversationService) Export(ctx context.Context, vaultID, id string) contract.ConversationExportResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationExportResult{Failure: f}
	}
	read := s.store.Get(vaultID, id)
	if read.Failure != nil {
		return contract.ConversationExportResult{Failure: read.Failure}
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      mygo.CallerWindow(ctx),
		Title:       "导出对话",
		DefaultPath: conversation.ExportFileName(read.Meta.Title),
		Filters:     []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
	})
	if err != nil {
		return contract.ConversationExportResult{Failure: contract.Fail(contract.CodeIOFailure, "无法打开保存对话框")}
	}
	if path == "" {
		return contract.ConversationExportResult{Saved: false}
	}
	markdown := conversation.ConversationToMarkdown(*read.Meta, read.Events)
	if err := os.WriteFile(path, []byte(markdown), 0o644); err != nil {
		return contract.ConversationExportResult{Failure: contract.Fail(contract.CodeIOFailure, "无法写入导出文件")}
	}
	return contract.ConversationExportResult{Saved: true, Name: filepath.Base(path)}
}

// Clear is M59: remove every conversation of the vault, including stray files.
func (s *ConversationService) Clear(vaultID string) contract.ConversationClearResult {
	if f := s.ensureVault(vaultID); f != nil {
		return contract.ConversationClearResult{Failure: f}
	}
	s.app.SetRecap(vaultID, "")
	return s.store.ClearAll(vaultID)
}
