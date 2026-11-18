package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/yuin/goldmark/text"
)

// 渲染入口：goldmark 解析 + 自定义渲染 + bluemonday 清理。
// 断言：不 panic、输出不含 <script、on\w+=、javascript:。
func FuzzRender(f *testing.F) {
	seeds := []string{
		"# t\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n\n![i](javascript:alert(1))\n",
		"| a |\n| - |\n| <img src=x onerror=alert(1)> |\n",
		"[[x]] [[a|b]] [y](../out) ![p](../../etc/passwd.png)\n",
		"<div onclick='x'>text</div> 前文 <em>斜</em>\n",
		"```\n<script>\n```\n",
		"%2e%2e/x.png data:image/svg+xml;base64,AAA //cdn/a.png",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		for _, variant := range []string{"reader", "chat"} {
			md := newMarkdown(nil, variant)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic variant=%s src=%q: %v", variant, src[:min(80, len(src))], r)
					}
				}()
				doc := md.Parser().Parse(text.NewReader(src))
				if variant == "chat" {
					transformChatRefs(doc, src)
				}
				var buf bytes.Buffer
				if err := md.Renderer().Render(&buf, src, doc); err != nil {
					t.Fatalf("render err: %v", err)
				}
				out := newPolicy().Sanitize(buf.String())
				low := strings.ToLower(out)
				if strings.Contains(low, "<script") {
					t.Fatalf("script leaked variant=%s src=%q out=%q", variant, src[:min(80, len(src))], out[:min(400, len(out))])
				}
				// on* 属性泄漏：在标签内（<tag ... onx=）才算；文本里的 "on0=" 不算。
				if regexp.MustCompile(`<[a-z][^>]*\son\w+\s*=`).MatchString(low) {
					t.Fatalf("on* leaked variant=%s out=%q", variant, out[:min(400, len(out))])
				}
				// javascript: 只在可执行位置（href/src/data-target/data-src/data-vault/data-vault-path）算泄漏；
				// 文本位置输出 "javascript:" 不可点，不算。
				if regexp.MustCompile(`(?:href|src|data-target|data-src|data-vault|data-vault-path)\s*=\s*"?\s*javascript:`).MatchString(low) {
					t.Fatalf("javascript: leaked variant=%s out=%q", variant, out[:min(400, len(out))])
				}
			}()
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
