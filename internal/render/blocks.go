package render

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"
)

// BlockCache renders the streaming chat text with a per-block HTML cache
// (M37): the text is parsed on every call, but only top-level blocks whose
// source changed are rendered again. Parsing the whole document keeps real
// block boundaries — a blank line inside a loose list does not split it.
//
// The cache belongs to the chat variant without a vault resolver: the output
// depends on the block source alone, which is what makes the key sound.
type BlockCache struct {
	mu     sync.Mutex
	blocks map[string]string
	order  []string
	hits   int
}

// blockCacheLimit bounds the cache; a long answer has far fewer blocks.
const blockCacheLimit = 2000

// NewBlockCache creates an empty cache.
func NewBlockCache() *BlockCache {
	return &BlockCache{blocks: map[string]string{}}
}

func (b *BlockCache) get(source string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	html, ok := b.blocks[source]
	if ok {
		b.hits++
	}
	return html, ok
}

func (b *BlockCache) put(source, html string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.blocks[source]; !exists {
		b.order = append(b.order, source)
		if len(b.order) > blockCacheLimit {
			oldest := b.order[0]
			b.order = b.order[1:]
			delete(b.blocks, oldest)
		}
	}
	b.blocks[source] = html
}

// RenderChat renders one streaming chat message. An unfinished trailing table
// or link is folded away first (TrimStreamingText) and replaced by the
// "still generating" placeholder, exactly like the legacy chat renderer.
func (b *BlockCache) RenderChat(markdownText string) (Result, error) {
	trimmed := TrimStreamingText(markdownText, true)
	src := []byte(trimmed.Text)
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))

	r := &kbRenderer{opts: Options{Variant: VariantChat}}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, &wikilink.Extender{Resolver: wikilinkResolver{r: r}}),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(r, 100))),
	)
	doc := md.Parser().Parse(text.NewReader(src))
	transformChatRefs(doc, src, r)

	var out bytes.Buffer
	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		source, ok := blockSource(child, src)
		if ok {
			if cached, hit := b.get(source); hit {
				out.WriteString(cached)
				continue
			}
		}
		var block bytes.Buffer
		if err := md.Renderer().Render(&block, src, child); err != nil {
			return Result{}, fmt.Errorf("render block: %w", err)
		}
		html := normalizeEntities(sanitize(block.String()))
		if ok {
			b.put(source, html)
		}
		out.WriteString(html)
	}

	html := out.String()
	if trimmed.PendingKind != "" {
		label := "正在生成链接…"
		if trimmed.PendingKind == "table" {
			label = "正在生成表格…"
		}
		html += `<div class="stream-pending" data-kind="` + trimmed.PendingKind + `" role="status">` + label + `</div>` + "\n"
	}
	return Result{HTML: html, HasImages: r.hasImages}, nil
}

// blockSource returns the exact source bytes of one top-level block. Nodes
// without source lines (there should be none for rendered blocks) fall back
// to an uncached render.
func blockSource(node interface{ Lines() *text.Segments }, src []byte) (string, bool) {
	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return "", false
	}
	var buf bytes.Buffer
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		buf.Write(segment.Value(src))
	}
	return buf.String(), true
}
