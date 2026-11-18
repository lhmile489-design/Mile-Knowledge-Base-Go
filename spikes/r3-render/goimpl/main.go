package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
)

func main() {
	// 把 cases.cjs 转为 JSON：用 node 先生成 cases.json（见 run 脚本）。
	casesPath := filepath.Join("..", "cases", "cases.json")
	data, err := os.ReadFile(casesPath)
	if err != nil {
		fatal("read cases.json: %v", err)
	}
	var cases []testCase
	if err := json.Unmarshal(data, &cases); err != nil {
		fatal("parse cases: %v", err)
	}
	outRoot := filepath.Join("..", "go-out")
	for _, v := range []string{"reader", "chat"} {
		os.MkdirAll(filepath.Join(outRoot, v), 0o755)
	}
	var manifest []map[string]any
	for _, c := range cases {
		variants := []string{}
		if c.Variant == "both" {
			variants = []string{"reader", "chat"}
		} else {
			variants = []string{c.Variant}
		}
		for _, variant := range variants {
			md := newMarkdown(c.Vault, variant)
			src := c.MD
			var pendingKind string
			if variant == "chat" && c.Streaming {
				tr := trimStreamingText(src, true)
				src = tr.text
				if tr.pending {
					pendingKind = tr.pendingKind
				}
			}
			var body string
			if strings.TrimSpace(src) != "" {
				out, err := render(md, []byte(src), c.Vault, variant)
				if err != nil {
					fatal("render %s/%s: %v", variant, c.ID, err)
				}
				body = out
			}
			if pendingKind != "" {
				label := ""
				if pendingKind == "table" {
					label = "正在生成表格…"
				} else {
					label = "正在生成链接…"
				}
				body += fmt.Sprintf(`<div class="stream-pending" data-kind=%q role="status">%s</div>`, pendingKind, label)
			}
			outPath := filepath.Join(outRoot, variant, c.ID+".html")
			os.WriteFile(outPath, []byte(body), 0o644)
			manifest = append(manifest, map[string]any{
				"id": c.ID, "variant": variant, "streaming": c.Streaming,
				"pendingKind": pendingKind, "bytes": len(body),
			})
		}
	}
	mj, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(outRoot, "manifest.json"), mj, 0o644)
	fmt.Printf("rendered %d outputs\n", len(manifest))
}

// 供 bench 使用：单次渲染耗时。
func renderOnce(md goldmark.Markdown, src []byte, v *vaultStub, variant string) (time.Duration, error) {
	start := time.Now()
	doc := md.Parser().Parse(text.NewReader(src))
	if variant == "chat" {
		transformChatRefs(doc, src)
	}
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return 0, err
	}
	_ = newPolicy().Sanitize(buf.String())
	return time.Since(start), nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fatal: "+format+"\n", args...)
	os.Exit(1)
}
