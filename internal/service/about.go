package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// About & diagnostics (M74, M75; PRD FR-SET). The page is a troubleshooting
// aid: version, engine state, fixed capabilities, and where things live. It
// must never receive an absolute path — displays are redacted and copying a
// real path is done here in Go (D-013).

const aboutTechStack = "Go 1.27 · React 19 · mygo 0.10 · SQLite FTS5"

// aboutRepoURL is shown on the page; the UI opens it in the system browser.
const aboutRepoURL = "https://github.com/lhmile489-design/Mile-Knowledge-Base-Go"

// writeClipboard is replaceable so tests never touch the real clipboard.
var writeClipboard = func(text string) error {
	return mygo.Clipboard.Write(transfer.TextData(text))
}

type storageEntry struct {
	kind     string
	label    string
	path     string
	display  string
	copyable bool
}

// AboutInfo returns the about page payload (M74).
func (s *SettingsService) AboutInfo(ctx context.Context) contract.AboutInfoResult {
	engine := contract.EngineStatus{Phase: "offline", Message: "没有活动的知识库"}
	if vaultID := s.app.Vaults.Active(); vaultID != "" {
		engine = s.app.computeEngineStatus(vaultID)
	}
	version := s.app.Version
	if version == "" {
		version = "开发构建"
	}
	locations := make([]contract.StorageLocation, 0, 6)
	for _, entry := range s.storageEntries() {
		locations = append(locations, contract.StorageLocation{
			Kind: entry.kind, Label: entry.label, Display: entry.display, Copyable: entry.copyable,
		})
	}
	return contract.AboutInfoResult{
		Version:      version,
		TechStack:    aboutTechStack,
		Engine:       engine,
		Capabilities: engineCapabilities(),
		RestartCount: 0,
		Storage:      locations,
	}
}

// CopyStoragePath writes a real path to the system clipboard (M75). The
// credential store has no path to copy.
func (s *SettingsService) CopyStoragePath(ctx context.Context, kind string) contract.CopyStoragePathResult {
	for _, entry := range s.storageEntries() {
		if entry.kind != kind {
			continue
		}
		if entry.kind == "credentials" {
			return contract.CopyStoragePathResult{Failure: contract.Fail(
				contract.CodeUnsupported, "API Key 由 Windows 凭据管理器保存，没有可复制的路径")}
		}
		if !entry.copyable || entry.path == "" {
			return contract.CopyStoragePathResult{Failure: contract.Fail(
				contract.CodeInvalidInput, "没有可复制的路径")}
		}
		if err := writeClipboard(entry.path); err != nil {
			return contract.CopyStoragePathResult{Failure: contract.Fail(
				contract.CodeIOFailure, "写入剪贴板失败")}
		}
		return contract.CopyStoragePathResult{OK: true}
	}
	return contract.CopyStoragePathResult{Failure: contract.Fail(contract.CodeInvalidInput, "未知的存储位置")}
}

// storageEntries builds the six rows once for both bindings. Vault rows show
// "未打开" (and cannot be copied) when there is no active vault.
func (s *SettingsService) storageEntries() []storageEntry {
	home, _ := os.UserHomeDir()
	dataDir := s.app.DataDir
	entries := []storageEntry{
		{kind: "vault-root", label: "知识库", display: "未打开"},
		{kind: "vault-settings", label: "库内配置", display: "未打开"},
		{kind: "providers", label: "Provider", display: "未打开"},
		{kind: "app-data", label: "应用数据", path: dataDir, display: redactHome(dataDir, home), copyable: true},
		{kind: "logs", label: "日志", path: filepath.Join(dataDir, "logs"), display: redactHome(filepath.Join(dataDir, "logs"), home), copyable: true},
		{kind: "credentials", label: "API Key", display: "Windows 凭据管理器"},
	}
	if vaultID := s.app.Vaults.Active(); vaultID != "" {
		if root, f := s.app.Vaults.Root(vaultID); f == nil {
			settingsDir := filepath.Join(root, ".mile-kb", "settings")
			entries[0] = storageEntry{kind: "vault-root", label: "知识库", path: root, display: "<vault>", copyable: true}
			entries[1] = storageEntry{kind: "vault-settings", label: "库内配置", path: settingsDir, display: "<vault>/.mile-kb/settings", copyable: true}
			entries[2] = storageEntry{kind: "providers", label: "Provider", path: filepath.Join(settingsDir, "providers.json"), display: "<vault>/.mile-kb/settings/providers.json", copyable: true}
		}
	}
	return entries
}

// redactHome replaces a path under the home directory with <home> (PRD 10.2).
// Paths outside it become <app-data> so the page never receives an absolute
// path, and the match requires a separator boundary (a sibling whose name
// starts with the home name must not be swallowed).
func redactHome(path, home string) string {
	if path == "" {
		return path
	}
	if home != "" {
		lowerPath, lowerHome := strings.ToLower(path), strings.ToLower(home)
		if strings.HasPrefix(lowerPath, lowerHome) {
			rest := path[len(home):]
			if rest == "" || rest[0] == '\\' || rest[0] == '/' {
				return "<home>" + rest
			}
		}
	}
	return "<app-data>"
}
