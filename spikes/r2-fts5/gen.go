// gen.go — 生成 10000 篇中英混合 Markdown 样本（含 frontmatter title/tags、标题行、wikilink）。
// 分布：正文 0.5–20 KB；约 60% 中文为主，40% 中英混合；约 30% 带 wikilink。
package main

import (
	"fmt"
	"math/rand"
	"strings"
)

var zhParas = []string{
	"知识库管理的核心是把分散的信息组织成可检索的结构。",
	"检索系统需要在召回率和精确率之间取得平衡。",
	"二元组分词对中日韩文字是一个工程上成本很低的方案。",
	"索引必须是派生数据，可以随时从原始文件重建。",
	"增量更新只应该重算受影响的部分，避免全量重建。",
	"权限控制要求在写入前检查目标路径是否在允许范围内。",
	"链接图能够揭示文档之间的隐藏关联。",
	"健康检查帮助发现断链、孤立文档和缺标题的问题。",
	"对话历史需要持久化，以便会话恢复后继续上下文。",
	"工具循环要限制最大步数，防止模型陷入无限调用。",
	"配置文件应该支持热重载，避免每次修改都重启进程。",
	"错误信息要脱敏，不能向前端发送调用栈和绝对路径。",
	"高亮区间要与索引使用同一套分词，否则显示会漂移。",
	"预切词写入 FTS5 可以让分词口径完全由应用层控制。",
	"事务提交要保证文档表、索引表和链接边表的一致。",
	"外部修改文件时需要去抖合并，批量落进一个事务。",
	"反向链接只计算已解析的边，并且排除自引用。",
	"功能区过滤让用户把注意力集中在特定目录上。",
	"标签体系让跨目录的主题聚合成为可能。",
	"首建索引进度要通过事件推送给前端显示进度条。",
}

var enParas = []string{
	"The index must be a derived artifact that can be rebuilt at any time.",
	"Incremental updates should only recompute affected edges.",
	"A single writer goroutine avoids SQLITE_BUSY under concurrent load.",
	"Tokenize on the application side keeps FTS5 configuration simple.",
	"Column weights in bm25 make title matches rank above body matches.",
	"Debounce and merge external file changes into one transaction.",
	"The single-writer pattern serializes all mutations through a channel.",
	"Backlinks are computed only from resolved edges in the link graph.",
	"Health reports are derived from the index, never persisted separately.",
	"Frontmatter parsing only supports flat key value pairs by design.",
	"Code fences must be skipped when extracting links and headings.",
	"Query tokens joined with OR give the same recall as the legacy app.",
	"Highlight ranges use literal case-insensitive matching, never regex.",
	"The excerpt window is sixty characters on each side of first match.",
	"System area documents are excluded from results by default.",
}

var zhWords = []string{
	"知识库", "检索", "分词", "索引", "增量", "事务", "链接", "图谱", "权限",
	"高亮", "片段", "标签", "标题", "正文", "路径", "目录", "文件", "扫描",
	"重建", "派生", "快照", "版本", "冲突", "恢复", "归档", "回收站", "草稿",
	"审批", "对话", "历史", "供应商", "密钥", "脱敏", "围栏", "引用", "锚点",
}

var enWords = []string{
	"sqlite", "fts5", "bm25", "tokenize", "bigram", "index", "vault",
	"markdown", "wikilink", "backlink", "excerpt", "highlight", "schema",
	"transaction", "debounce", "goroutine", "webhook", "renderer",
	"provider", "snapshot", "revision", "conflict", "restore", "archive",
}

var tagPool = []string{
	"search", "index", "link", "agent", "ui", "config", "provider",
	"历史", "检索", "知识库", "设计", "后端",
}

var areaPool = []string{"skills", "raws", "projects", "notes", "system"}

func genDocument(rng *rand.Rand, i int) (path, content string) {
	area := areaPool[rng.Intn(len(areaPool))]
	name := fmt.Sprintf("doc-%05d", i)
	if rng.Intn(100) < 12 {
		// 12% 的文件名带版本号或连字符，考验路径分词
		name = fmt.Sprintf("guide-v%d.%d-%03d", rng.Intn(9)+1, rng.Intn(9), i%1000)
	}
	path = fmt.Sprintf("%s/%s.md", area, name)

	var b strings.Builder
	// frontmatter：80% 有 title，60% 有 tags
	hasTitle := rng.Intn(100) < 80
	var tags []string
	if rng.Intn(100) < 60 {
		n := 1 + rng.Intn(3)
		for k := 0; k < n; k++ {
			tags = append(tags, tagPool[rng.Intn(len(tagPool))])
		}
	}
	if hasTitle || len(tags) > 0 {
		b.WriteString("---\n")
		if hasTitle {
			if rng.Intn(100) < 50 {
				fmt.Fprintf(&b, "title: %s%04d\n", zhWords[rng.Intn(len(zhWords))], i)
			} else {
				fmt.Fprintf(&b, "title: %s-%04d\n", enWords[rng.Intn(len(enWords))], i)
			}
		}
		if len(tags) > 0 {
			fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(tags, ", "))
		}
		b.WriteString("---\n\n")
	}

	// 正文 0.5–20 KB
	target := 512 + rng.Intn(19968)
	// 标题行 1-4 个
	nHead := 1 + rng.Intn(4)
	for h := 0; h < nHead && b.Len() < target; h++ {
		lv := 1 + rng.Intn(3)
		fmt.Fprintf(&b, "%s %s %s\n\n", strings.Repeat("#", lv),
			zhWords[rng.Intn(len(zhWords))], enWords[rng.Intn(len(enWords))])
		paras := 1 + rng.Intn(4)
		for p := 0; p < paras && b.Len() < target; p++ {
			mixed := rng.Intn(100) < 40
			pool := zhParas
			if mixed {
				pool = enParas
			}
			b.WriteString(pool[rng.Intn(len(pool))])
			b.WriteString("\n\n")
			// 30% 的段落带 wikilink
			if rng.Intn(100) < 30 {
				t := fmt.Sprintf("doc-%05d", rng.Intn(10000))
				if rng.Intn(100) < 20 {
					fmt.Fprintf(&b, "相关阅读 [[%s|别名文本]]\n\n", t)
				} else {
					fmt.Fprintf(&b, "相关阅读 [[%s]]\n\n", t)
				}
			}
		}
	}
	// 偶尔一段代码围栏
	if rng.Intn(100) < 15 {
		b.WriteString("```go\n// sample code with tokens like getUserName and v4.1\n```\n\n")
	}
	return path, b.String()
}
