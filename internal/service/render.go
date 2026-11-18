package service

import (
	"context"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/index"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/render"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/vault"
)

// RenderService renders Markdown into the sanitised HTML the reader shows.
type RenderService struct{ app *App }

// NewRenderService binds one app.
func NewRenderService(app *App) *RenderService { return &RenderService{app: app} }

// RenderResult is the reader payload (API.md M36).
type RenderResult struct {
	HTML      string                  `json:"html,omitempty"`
	Revision  int64                   `json:"revision,omitempty"`
	HasImages bool                    `json:"hasImages,omitempty"`
	Links     []contract.ResolvedLink `json:"links,omitempty"`
	Failure   *contract.Failure       `json:"failure,omitempty"`
}

// BlockResult is the M37 reply.
type BlockResult struct {
	HTML    string            `json:"html"`
	Failure *contract.Failure `json:"failure,omitempty"`
}

// maxBlockChars matches the conversation event text limit; blocks above it
// are rejected rather than truncated mid-structure.
const maxBlockChars = 64_000

// RenderBlock renders one streaming chat message (M37). Sealed top-level
// blocks come from the cache; only the growing tail is parsed again. The
// trailing unfinished table or link is folded into a "still generating"
// placeholder by the renderer.
func (s *RenderService) RenderBlock(ctx context.Context, text, kind string) BlockResult {
	switch kind {
	case "message", "thought", "user":
	default:
		return BlockResult{Failure: contract.Fail(contract.CodeInvalidInput, "kind 不合法")}
	}
	if count := len([]rune(text)); count > maxBlockChars {
		return BlockResult{Failure: contract.Fail(contract.CodeTooLarge, "块超过 64000 字符").WithLimit(int64(count), maxBlockChars)}
	}
	if strings.TrimSpace(text) == "" {
		return BlockResult{HTML: ""}
	}
	result, err := s.app.Blocks.RenderChat(text)
	if err != nil {
		return BlockResult{Failure: contract.Fail(contract.CodeUnsupported, "渲染失败")}
	}
	return BlockResult{HTML: result.HTML}
}

// RenderDocument renders one document for reading mode. The result is not
// cached here: the frontend caches by revision.
func (s *RenderService) RenderDocument(ctx context.Context, vaultID, rel string, revision int64) RenderResult {
	root, f := s.app.Vaults.Root(vaultID)
	if f != nil {
		return RenderResult{Failure: f}
	}
	snap, f := vault.Read(root, rel)
	if f != nil {
		return RenderResult{Failure: f}
	}
	if revision != 0 && snap.Revision != revision {
		return RenderResult{Failure: contract.Fail(contract.CodeRevisionMismatch, "文档已变化，请重新打开")}
	}
	if snap.Degraded == "large-read-only" {
		return RenderResult{Failure: contract.Fail(contract.CodeTooLarge, "文件超过 2 MiB，只提供只读预览")}
	}

	resolver := s.app.resolverFor(ctx, vaultID, root)
	result, err := render.Render([]byte(snap.Text), render.Options{
		Variant:  render.VariantReader,
		Resolver: resolver,
		ImageURL: func(p string) string {
			return "http://kbimg.localhost/" + escapeURLPath(p) + "?vault=" + url.QueryEscape(vaultID)
		},
	})
	if err != nil {
		return RenderResult{Failure: contract.Fail(contract.CodeUnsupported, "渲染失败")}
	}
	out := RenderResult{HTML: result.HTML, Revision: snap.Revision, HasImages: result.HasImages}
	// The reader's event delegation needs the classification too; M36 already
	// promises it. Titles resolve here as well, so Ctrl+click can open a
	// document whose title differs from its file name.
	if resolver, ok := resolver.(*indexedLinkResolver); ok {
		seen := map[string]bool{}
		for _, link := range index.ExtractLinks(snap.Text) {
			if link.Kind != index.LinkWiki && link.Kind != index.LinkInternal {
				continue
			}
			if seen[link.Target] {
				continue
			}
			seen[link.Target] = true
			path, candidates := resolver.index.Resolve(link.Target)
			resolved := contract.ResolvedLink{
				Target: link.Target, Text: link.Text, Kind: link.Kind, Candidates: candidates,
			}
			if candidates == 1 {
				p := path
				resolved.Path = &p
			}
			out.Links = append(out.Links, resolved)
		}
	}
	return out
}

func escapeURLPath(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

// resolverFor returns a resolver bound to a vault root, cached per vault. When
// the index has a resolver snapshot, link targets go through it (three layers,
// including titles); images always use the filesystem resolver.
func (a *App) resolverFor(ctx context.Context, vaultID, root string) render.LinkResolver {
	a.mu.Lock()
	if a.resolvers == nil {
		a.resolvers = map[string]*vaultResolver{}
	}
	r, ok := a.resolvers[vaultID]
	if !ok || r.root != root {
		r = &vaultResolver{root: root, names: map[string][]string{}}
		a.resolvers[vaultID] = r
	}
	a.mu.Unlock()
	r.refreshIfStale()
	if indexed := a.indexResolver(ctx, vaultID); indexed != nil {
		return &indexedLinkResolver{index: indexed, vault: r}
	}
	return r
}

// indexedLinkResolver lets the index answer link targets while the filesystem
// resolver keeps handling images and anything the index does not know yet.
type indexedLinkResolver struct {
	index *index.Resolver
	vault *vaultResolver
}

func (r *indexedLinkResolver) ResolveLink(target string) (string, string, bool) {
	clean, blocked := cleanLinkTarget(target)
	if blocked {
		return "", "blocked", false
	}
	if clean == "" {
		return "", "missing", false
	}
	path, candidates := r.index.Resolve(clean)
	switch {
	case candidates == 1:
		return path, "internal", true
	case candidates > 1:
		return "", "ambiguous", false
	}
	return r.vault.ResolveLink(clean)
}

func (r *indexedLinkResolver) ResolveImage(src string) (string, bool) {
	return r.vault.ResolveImage(src)
}

// cleanLinkTarget applies the same URL cleanup the M0 resolver used.
func cleanLinkTarget(target string) (string, bool) {
	clean := strings.TrimSpace(target)
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if decoded, err := url.PathUnescape(clean); err == nil {
		clean = decoded
	}
	clean = strings.ReplaceAll(clean, "\\", "/")
	if strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
		return "", true
	}
	return clean, false
}

// vaultResolver resolves links through two layers in M0: the exact relative
// path, then a unique file name. Title matching arrives with the M1 index.
type vaultResolver struct {
	root string

	mu        sync.Mutex
	names     map[string][]string // lowercase base name without extension -> paths
	builtAt   time.Time
	buildOnce sync.Once
}

func (v *vaultResolver) refreshIfStale() {
	v.mu.Lock()
	stale := time.Since(v.builtAt) > 3*time.Second
	v.mu.Unlock()
	if !stale {
		return
	}
	v.build()
}

func (v *vaultResolver) build() {
	names := map[string][]string{}
	_ = filepath.WalkDir(v.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".mile-kb" {
				return filepath.SkipDir
			}
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".markdown") {
			return nil
		}
		rel, relErr := filepath.Rel(v.root, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		base := strings.ToLower(strings.TrimSuffix(d.Name(), path.Ext(d.Name())))
		names[base] = append(names[base], rel)
		return nil
	})
	v.mu.Lock()
	v.names = names
	v.builtAt = time.Now()
	v.mu.Unlock()
}

// ResolveLink implements render.LinkResolver.
func (v *vaultResolver) ResolveLink(target string) (string, string, bool) {
	clean := strings.TrimSpace(target)
	if clean == "" {
		return "", "missing", false
	}
	// Strip URL fragments and queries: the legacy reader treats them as noise
	// for resolution.
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if decoded, err := url.PathUnescape(clean); err == nil {
		clean = decoded
	}
	clean = strings.ReplaceAll(clean, "\\", "/")
	if strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
		return "", "blocked", false
	}

	if norm, f := pathguard.ValidateRel(clean); f == nil && norm != "" {
		if info, err := os.Stat(filepath.Join(v.root, filepath.FromSlash(norm))); err == nil && !info.IsDir() {
			return norm, "internal", true
		}
		// Try adding the Markdown extension.
		withExt := norm + ".md"
		if info, err := os.Stat(filepath.Join(v.root, filepath.FromSlash(withExt))); err == nil && !info.IsDir() {
			return withExt, "internal", true
		}
	}

	base := strings.ToLower(strings.TrimSuffix(clean, path.Ext(clean)))
	v.mu.Lock()
	matches := append([]string{}, v.names[base]...)
	v.mu.Unlock()
	switch len(matches) {
	case 1:
		return matches[0], "internal", true
	case 0:
		return "", "missing", false
	default:
		return "", "ambiguous", false
	}
}

// ResolveImage implements render.LinkResolver.
func (v *vaultResolver) ResolveImage(src string) (string, bool) {
	clean := strings.TrimSpace(src)
	if clean == "" {
		return "", false
	}
	if i := strings.IndexAny(clean, "?#"); i >= 0 {
		clean = clean[:i]
	}
	if decoded, err := url.PathUnescape(clean); err == nil {
		clean = decoded
	}
	clean = strings.ReplaceAll(clean, "\\", "/")
	if strings.HasPrefix(clean, "/") || strings.Contains(clean, "..") {
		return "", false
	}
	norm, f := pathguard.ValidateRel(clean)
	if f != nil || norm == "" {
		return "", false
	}
	switch strings.ToLower(path.Ext(norm)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".bmp":
	default:
		return "", false
	}
	if info, err := os.Stat(filepath.Join(v.root, filepath.FromSlash(norm))); err != nil || info.IsDir() {
		return "", false
	}
	return norm, true
}
