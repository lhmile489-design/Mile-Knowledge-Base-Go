package service

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// ImageHandler answers the kbimg protocol:
//
//	GET  http://kbimg.localhost/<url-escaped vault-relative path>?vault=<id>
//	POST http://kbimg.localhost/__upload?vault=<id>   (raw image bytes)
//
// Every request must come from one of this app's pages and is re-checked on
// disk. Images are served by their magic bytes, never by the extension.
func ImageHandler(app *App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if mygo.CallerWindow(r.Context()) == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.URL.Path == "/__upload" {
			uploadImage(app, w, r)
			return
		}
		serveImage(app, w, r)
	}
}

func vaultRootForRequest(app *App, r *http.Request) (string, string, bool) {
	vaultID := r.URL.Query().Get("vault")
	if vaultID == "" {
		vaultID = app.Vaults.Active()
	}
	if f := validateID(vaultID); f != nil {
		return "", "", false
	}
	root, f := app.Vaults.Root(vaultID)
	if f != nil {
		return "", "", false
	}
	return root, vaultID, true
}

func serveImage(app *App, w http.ResponseWriter, r *http.Request) {
	root, _, ok := vaultRootForRequest(app, r)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	rel := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/"))
	if rel == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if info.Size() > vault.MaxAttachmentBytes {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_, mime, known := vault.SniffImage(body)
	if !known {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", mime)
	_, _ = w.Write(body)
}

func uploadImage(app *App, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"failure": contract.Fail(contract.CodeInvalidInput, "上传只接受 POST"),
		})
		return
	}
	root, _, ok := vaultRootForRequest(app, r)
	if !ok {
		writeImageFailure(w, contract.Fail(contract.CodeVaultUnavailable, "知识库不可用"))
		return
	}
	// Read one byte past the cap so an oversized body is detected without
	// buffering all of it.
	body, err := io.ReadAll(io.LimitReader(r.Body, vault.MaxAttachmentBytes+1))
	if err != nil {
		writeImageFailure(w, contract.Fail(contract.CodeIOFailure, "读取上传内容失败"))
		return
	}
	if len(body) > vault.MaxAttachmentBytes {
		writeImageFailure(w, contract.Fail(contract.CodeTooLarge, "图片超过 8 MiB"))
		return
	}
	rel, existed, f := vault.SaveAttachment(root, body)
	if f != nil {
		writeImageFailure(w, f)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"path": rel, "existed": existed})
}

func writeImageFailure(w http.ResponseWriter, f *contract.Failure) {
	_ = json.NewEncoder(w).Encode(map[string]any{"failure": f})
}
