package service

import (
	"net/http"
	"path"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// imageExtensions is the display whitelist (FR-DOC). SVG and ICO are refused:
// they can carry script or are not real photos.
var imageExtensions = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
}

// ImageHandler answers the kbimg protocol:
//
//	http://kbimg.localhost/<url-escaped vault-relative path>?vault=<id>
//
// Only files inside a registered vault, under a whitelisted extension and
// passing the two-layer path check are served. Everything else is a 404/403
// with no body details.
func ImageHandler(app *App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serveImage(app, w, r)
	}
}

func serveImage(app *App, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	vaultID := r.URL.Query().Get("vault")
	if vaultID == "" {
		vaultID = app.Vaults.Active()
	}
	if f := validateID(vaultID); f != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	root, f := app.Vaults.Root(vaultID)
	if f != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rel := strings.TrimPrefix(r.URL.Path, "/")
	rel = strings.TrimSpace(rel)
	if rel == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// URL paths arrive percent-decoded by net/http for the path component.
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	ext := strings.ToLower(path.Ext(abs))
	contentType, ok := imageExtensions[ext]
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, abs)
}
