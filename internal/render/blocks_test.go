package render

import (
	"strings"
	"testing"
)

func TestBlockCacheMatchesWholeRender(t *testing.T) {
	docs := []string{
		"第一段。\n\n第二段带 `代码`。\n",
		"# 标题\n\n- 一\n- 二\n\n- 三\n\n结尾\n",
		"```go\nfmt.Println(1)\n```\n\n之后\n",
		"| a | b |\n| --- | --- |\n| 1 | 2 |\n",
		"建议看 skills/react/SKILL.md。\n\n[[note-a]] 也看\n",
		"![图](raws/a.png)\n\n> 引用\n",
	}
	for _, doc := range docs {
		whole, err := Render([]byte(doc), Options{Variant: VariantChat})
		if err != nil {
			t.Fatalf("whole: %v", err)
		}
		blocks, err := NewBlockCache().RenderChat(doc)
		if err != nil {
			t.Fatalf("blocks: %v", err)
		}
		if whole.HTML != blocks.HTML {
			t.Fatalf("block output differs:\nwhole: %q\nblock: %q", whole.HTML, blocks.HTML)
		}
	}
}

func TestBlockCacheReusesSealedBlocks(t *testing.T) {
	cache := NewBlockCache()
	if _, err := cache.RenderChat("第一段。\n\n第二段。"); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	hitsAfterFirst := cache.hits
	cache.mu.Unlock()
	if hitsAfterFirst != 0 {
		t.Fatalf("first render must miss, got %d hits", hitsAfterFirst)
	}
	if _, err := cache.RenderChat("第一段。\n\n第二段。"); err != nil {
		t.Fatal(err)
	}
	cache.mu.Lock()
	hitsAfterSecond := cache.hits
	cache.mu.Unlock()
	if hitsAfterSecond < 2 {
		t.Fatalf("second render reused %d blocks", hitsAfterSecond)
	}
}

func TestBlockCachePendingTableAndLink(t *testing.T) {
	cache := NewBlockCache()
	out, err := cache.RenderChat("先写一段。\n\n| 名称 | 路径 |")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.HTML, "先写一段") {
		t.Fatalf("kept text missing: %q", out.HTML)
	}
	if !strings.Contains(out.HTML, `class="stream-pending"`) || !strings.Contains(out.HTML, "正在生成表格…") {
		t.Fatalf("table placeholder missing: %q", out.HTML)
	}
	if strings.Contains(out.HTML, "| 名称") {
		t.Fatalf("half table rendered: %q", out.HTML)
	}

	out, err = cache.RenderChat("见 [文档](raws/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.HTML, "正在生成链接…") || strings.Contains(out.HTML, "](raws/") {
		t.Fatalf("link placeholder missing: %q", out.HTML)
	}

	out, err = cache.RenderChat("完整表格：\n\n| a | b |\n| --- | --- |\n| 1 | 2 |")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.HTML, "<table>") || strings.Contains(out.HTML, "stream-pending") {
		t.Fatalf("complete table not rendered: %q", out.HTML)
	}
}

func TestBlockCacheChatRefsAreUnknownWithoutVault(t *testing.T) {
	out, err := NewBlockCache().RenderChat("看 skuml.md 与 [[某笔记]]")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.HTML, `data-status="unknown"`) {
		t.Fatalf("refs must stay unknown: %q", out.HTML)
	}
	if strings.Contains(out.HTML, "找不到") {
		t.Fatalf("no existence claim without a click: %q", out.HTML)
	}
}
