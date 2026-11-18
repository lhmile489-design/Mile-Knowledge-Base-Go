package main

import (
	"strings"
	"testing"
	"time"

	"github.com/yuin/goldmark/text"
)

// 生成约 size 字节的混合 markdown。
func makeDoc(size int) []byte {
	var b strings.Builder
	chunk := `# 标题一

正文段落，包含 **粗体**、*斜体*、` + "`行内代码`" + `、[外链](https://example.com)、[[wikilink]]、库内 [路径](skills/a.md)。

| 列一 | 列二 | 列三 |
| --- | --- | --- |
| a | b | c |

- [x] 任务一
- [ ] 任务二

` + "```go\nfunc main() { fmt.Println(1) }\n```" + `

> 引用一段。

![图](raws/a.png)

`
	for b.Len() < size {
		b.WriteString(chunk)
	}
	return []byte(b.String()[:size])
}

func benchRender(src []byte, variant string) time.Duration {
	md := newMarkdown(nil, variant)
	start := time.Now()
	doc := md.Parser().Parse(text.NewReader(src))
	if variant == "chat" {
		transformChatRefs(doc, src)
	}
	var sb strings.Builder
	w := &sw{&sb}
	_ = md.Renderer().Render(w, src, doc)
	_ = newPolicy().Sanitize(sb.String())
	return time.Since(start)
}

type sw struct{ b *strings.Builder }

func (s *sw) Write(p []byte) (int, error)       { return s.b.Write(p) }
func (s *sw) WriteByte(c byte) error            { return s.b.WriteByte(c) }
func (s *sw) WriteString(v string) (int, error) { return s.b.WriteString(v) }
func (s *sw) WriteRune(r rune) (int, error)     { return s.b.WriteRune(r) }
func (s *sw) Available() int                    { return 0 }
func (s *sw) Buffered() int                     { return 0 }
func (s *sw) Flush() error                      { return nil }

func TestPerf(t *testing.T) {
	for _, sz := range []int{100 * 1024, 1024 * 1024} {
		src := makeDoc(sz)
		// warmup
		benchRender(src, "reader")
		best := time.Hour
		var total time.Duration
		const N = 5
		for i := 0; i < N; i++ {
			d := benchRender(src, "reader")
			if d < best {
				best = d
			}
			total += d
		}
		t.Logf("render %d KB reader: best=%v avg=%v", sz/1024, best, total/N)
	}
	// 流式：1 万字 chat，按块追加只重渲染末尾块。模拟 200 次渲染末尾块（每帧）。
	full := string(makeDoc(10000))
	var lastBlock string
	// 取末尾 ~800 字节作为「正在生成的块」
	lastBlock = full[len(full)-800:]
	md := newMarkdown(nil, "chat")
	best := time.Hour
	var total time.Duration
	const N = 50
	for i := 0; i < N; i++ {
		start := time.Now()
		doc := md.Parser().Parse(text.NewReader([]byte(lastBlock)))
		transformChatRefs(doc, []byte(lastBlock))
		var sb strings.Builder
		w := &sw{&sb}
		_ = md.Renderer().Render(w, []byte(lastBlock), doc)
		_ = newPolicy().Sanitize(sb.String())
		d := time.Since(start)
		if d < best {
			best = d
		}
		total += d
	}
	t.Logf("stream tail-block 800B render: best=%v avg=%v over %d iters", best, total/N, N)
	// 整篇 1 万字 chat 单帧（对比旧版每帧全解析）
	src2 := makeDoc(10000)
	best = time.Hour
	total = 0
	for i := 0; i < N; i++ {
		d := benchRender(src2, "chat")
		if d < best {
			best = d
		}
		total += d
	}
	t.Logf("full 10KB chat render: best=%v avg=%v", best, total/N)
}
