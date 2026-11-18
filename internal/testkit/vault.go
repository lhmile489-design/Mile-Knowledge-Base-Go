// Package testkit builds reproducible sample knowledge bases and filesystem
// helpers shared by tests, benchmarks and cmd/genvault.
//
// It never touches a real vault: callers always pass a fresh temporary
// directory they created themselves.
package testkit

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func MustMkdir(root string, elems ...string) error {
	return os.MkdirAll(filepath.Join(append([]string{root}, elems...)...), 0o755)
}

func write(root string, rel string, body string) error {
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(body), 0o644)
}

// tinyPNG is a 1x1 transparent PNG used by the image samples.
var tinyPNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
)

func BuildTypical(root string) error {
	dirs := []string{
		"skills/react", "skills/nodejs", "skills/harmonyos", "skills/go",
		"raws", "raws/attachments", "Clippings", "_archives", "projects/campus-hub",
		"projects/mile-go", "tools", "AI Knowledge base", "dev-constraints",
		".claude", ".mile-kb/settings", "my-notes", "空目录",
	}
	for _, d := range dirs {
		if err := MustMkdir(root, filepath.FromSlash(d)); err != nil {
			return err
		}
	}

	files := map[string]string{
		"skills/react/SKILL.md": "# React Skill\n\n受控组件与状态提升。参考 [[共享笔记]]。\n",
		"skills/react/hooks-notes.md": "---\ntitle: Hooks 笔记\ntags: [react, hooks]\n---\n\n" +
			"## 状态管理\n\n- useState\n- useReducer\n\n| 名称 | 用途 |\n| --- | --- |\n| useMemo | 缓存值 |\n\n> 引用一行。\n\n```js\nconst x = 1;\n```\n",
		"skills/nodejs/SKILL.md":    "# Node.js Skill\n\n事件循环与流。\n",
		"skills/go/SKILL.md":        "# Go Skill\n\nctx 贯穿与 errgroup 的使用。\n",
		"skills/harmonyos/SKILL.md": "## 鸿蒙\n\nArkTS 状态管理。\n",
		"raws/富文本样例.md": "---\ntitle: 富文本样例\ntags: demo, 排版\n---\n\n# 富文本样例\n\n" +
			"一段包含 **加粗**、`代码` 与 ~~删除线~~ 的正文。\n\n- [x] 已完成任务\n- [ ] 待办任务\n\n" +
			"![库内图片](raws/attachments/sample.png)\n\n" +
			"链接：[共享笔记](共享笔记.md)、[[不存在的页面]]、[[共享笔记|别名]]、[[同名文档]]\n\n" +
			"<div class=\"x\">原始 HTML 块</div>\n\n行内 <b>标签</b> 也是原文。\n",
		"raws/共享笔记.md":     "# 共享笔记\n\n被多个文档引用。\n\n- [x] 复用\n",
		"raws/首页.md":       "# 首页\n\n目录页。链接 [[富文本样例]]。\n",
		"raws/孤立文档.md":     "# 孤立文档\n\n没有任何入链或出链。\n",
		"raws/缺标题.md":      "没有标题行的一段正文。\n",
		"raws/同名文档.md":     "# 同名 A\n\n在 raws 下。\n",
		"my-notes/同名文档.md": "# 同名 B\n\n在 my-notes 下。\n",
		"my-notes/一个名字特别长特别长特别长特别长特别长特别长的文档.md": "# 长名\n\n用于测试截断与布局。\n",
		"projects/campus-hub/README.md":  "# campus-hub\n\n项目说明。\n",
		"projects/mile-go/README.md":     "# mile-go\n\n项目说明。\n",
		"tools/脚本清单.md":                  "# 工具\n\n- 生成样本\n",
		"AI Knowledge base/README.md":    "# AI Knowledge base\n\n独立知识库。\n",
		"dev-constraints/约束.md":          "# 约束\n\n- 不越界\n",
		"my-notes/CRLF与BOM.md":           "\ufeff# 换行与编码\r\n\r\n第一行\r\n第二行\r\n",
		"my-notes/普通笔记.md":               "# 普通笔记\n\n内容一行。\n",
		".mile-kb/settings/aliases.json": "{\n  \"raws\": \"暂存\"\n}\n",
	}
	for rel, body := range files {
		if err := write(root, rel, body); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "attachments", "sample.png"), tinyPNG, 0o644); err != nil {
		return err
	}

	// >1 MiB: readable as source only; >2 MiB: read-only preview.
	big := strings.Repeat("这是一个用于测试大文件降级的段落。\n\n", 64*1024)
	if err := write(root, "raws/big-1m.md", "# 大文件 1 MiB\n\n"+big[:1200*1024]); err != nil {
		return err
	}
	huge := strings.Repeat("超大文件内容占位。\n", 200*1024)
	if err := write(root, "raws/huge-2m.md", "# 超大文件\n\n"+huge); err != nil {
		return err
	}
	return nil
}

func BuildBig(root string) error {
	const docs = 10000
	for i := 0; i < docs; i++ {
		dir := filepath.Join("bulk", fmt.Sprintf("组%02d", i%40), fmt.Sprintf("层%d", (i/40)%6))
		rel := filepath.ToSlash(filepath.Join(dir, fmt.Sprintf("文档-%05d.md", i)))
		body := "# 文档 " + strconv.Itoa(i) + "\n\ntags: bulk\n\n" +
			"第 " + strconv.Itoa(i) + " 篇，关键词：知识库、检索、整理。\n\n" +
			"引用 [[文档-" + fmt.Sprintf("%05d", (i+1)%docs) + "]]。\n"
		if i%50 == 0 {
			body += "\n## 小节\n\n| a | b |\n| --- | --- |\n| 1 | 2 |\n"
		}
		if err := write(root, rel, body); err != nil {
			return err
		}
	}
	// A code repository dropped into the vault: ignored by index and watcher,
	// but it still makes the tree and watch limits realistic.
	for i := 0; i < 3000; i++ {
		rel := fmt.Sprintf("code-repo/src/module%03d/file%04d.ts", i%200, i)
		if err := write(root, rel, "export const value = "+strconv.Itoa(i)+";\n"); err != nil {
			return err
		}
	}
	for i := 0; i < 1000; i++ {
		if err := write(root, fmt.Sprintf("code-repo/node_modules/pkg%03d/index.js", i), "module.exports = {};\n"); err != nil {
			return err
		}
		if err := write(root, fmt.Sprintf("code-repo/.git/objects/%02x/%04d", i%256, i), "x"); err != nil {
			return err
		}
	}
	return MustMkdir(root, ".mile-kb")
}
