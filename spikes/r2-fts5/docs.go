// docs.go — 移植旧版 parseDocument（frontmatter / 标题行 / tags）与 extractLinks / resolveLink。
// 对齐 main/services/tokenize.ts:125-156、shared/links.ts:79-225。
package main

import (
	"regexp"
	"strings"
)

type parsedDocument struct {
	title      string
	headings   []string
	tags       []string
	body       string
	pathTokens []string
	links      []linkRef
}

type linkRef struct {
	kind   string // wiki | internal | external | other
	target string
}

var (
	headingRe  = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	fenceRe    = regexp.MustCompile(`^\s*(` + "```" + `|~~~)`)
	keyValRe   = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*:\s*(.*)$`)
	listItemRe = regexp.MustCompile(`^\s*-\s+(.+)$`)
	wikiRe     = regexp.MustCompile(`\[\[([^[\]]*?)\]\]`)
	schemeRe   = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
	httpRe     = regexp.MustCompile(`^https?://`)
)

func stripQuotes(v string) string {
	if len(v) >= 2 {
		f, l := v[0], v[len(v)-1]
		if (f == '"' && l == '"') || (f == '\'' && l == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// parseFrontmatter 只支持扁平 key: value 与 tags 的三种写法；未闭合 fence 容错。
func parseFrontmatter(lines []string) (fields map[string]string, tags []string, bodyStart int) {
	fields = map[string]string{}
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fields, tags, 0
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	limit := len(lines)
	if end != -1 {
		limit = end
	}
	collectingTags := false
	for i := 1; i < limit; i++ {
		line := lines[i]
		if m := listItemRe.FindStringSubmatch(line); m != nil && collectingTags {
			tags = append(tags, stripQuotes(strings.TrimSpace(m[1])))
			continue
		}
		m := keyValRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := strings.ToLower(m[1])
		value := strings.TrimSpace(m[2])
		collectingTags = key == "tags" && value == ""
		if key == "tags" && value != "" {
			inner := value
			if strings.HasPrefix(inner, "[") && strings.HasSuffix(inner, "]") {
				inner = inner[1 : len(inner)-1]
			}
			for _, part := range strings.Split(inner, ",") {
				if t := stripQuotes(strings.TrimSpace(part)); t != "" {
					tags = append(tags, t)
				}
			}
			continue
		}
		if value != "" {
			fields[key] = stripQuotes(value)
		}
	}
	if end == -1 {
		return fields, tags, len(lines)
	}
	return fields, tags, end + 1
}

func parseDocument(path, content string) parsedDocument {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	fields, tags, bodyStart := parseFrontmatter(lines)

	var headings []string
	inFence := false
	for i := bodyStart; i < len(lines); i++ {
		line := lines[i]
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			headings = append(headings, strings.TrimSpace(m[2]))
		}
	}

	fileName := path[strings.LastIndex(path, "/")+1:]
	title := fields["title"]
	if title == "" && len(headings) > 0 {
		title = headings[0]
	}
	if title == "" {
		title = fileName
	}

	body := strings.Join(lines[min(bodyStart, len(lines)):], "\n")

	lowerTags := make([]string, 0, len(tags))
	for _, t := range tags {
		lowerTags = append(lowerTags, strings.ToLower(t))
	}

	return parsedDocument{
		title:      title,
		headings:   headings,
		tags:       lowerTags,
		body:       body,
		pathTokens: tokenizePath(path),
		links:      extractLinks(body),
	}
}

// blankOutInlineCode 把行内 code 区间替换成等长空格（保列号）。
func blankOutInlineCode(line string) string {
	var b strings.Builder
	b.Grow(len(line))
	inCode := false
	for _, r := range line {
		if r == '`' {
			inCode = !inCode
			b.WriteByte(' ')
			continue
		}
		if inCode {
			b.WriteByte(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func classifyURL(target string) string {
	if httpRe.MatchString(strings.ToLower(target)) {
		return "external"
	}
	if schemeRe.MatchString(strings.ToLower(target)) {
		return "other"
	}
	return "internal"
}

// extractLinks 跳过代码围栏与行内 code；`[[目标|别名]]` 取竖线左侧。
func extractLinks(content string) []linkRef {
	var links []linkRef
	inFence := false
	for _, raw := range strings.Split(content, "\n") {
		if fenceRe.MatchString(raw) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		line := blankOutInlineCode(raw)

		for _, m := range wikiRe.FindAllStringSubmatch(line, -1) {
			inner := m[1]
			target := inner
			if p := strings.Index(inner, "|"); p != -1 {
				target = inner[:p]
			}
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			links = append(links, linkRef{kind: "wiki", target: target})
		}
		// Markdown 链接：Go 的 regexp 不支持 (?!) 前瞻，手写扫描。
		// 找 ![ 或 [ 之后紧跟 (target) 的形态；!( 前缀是图片不算链接。
		for i := 0; i < len(line); i++ {
			if line[i] != '[' {
				continue
			}
			isImage := i > 0 && line[i-1] == '!'
			// 找匹配的 ]
			j := i + 1
			for j < len(line) && line[j] != ']' && line[j] != '[' {
				j++
			}
			if j >= len(line) || line[j] != ']' {
				continue
			}
			if j+1 >= len(line) || line[j+1] != '(' {
				continue
			}
			// 在 ( ) 里取 target，允许成对括号（javascript:alert(1)、Foo_(bar)）
			depth := 0
			k := j + 2
			var tb strings.Builder
			for k < len(line) {
				c := line[k]
				if c == '(' {
					depth++
				} else if c == ')' {
					if depth == 0 {
						break
					}
					depth--
				}
				tb.WriteByte(c)
				k++
			}
			target := strings.TrimSpace(tb.String())
			if isImage || target == "" {
				i = j + 1
				continue
			}
			links = append(links, linkRef{kind: classifyURL(target), target: target})
			i = k
		}
	}
	return links
}

// resolveLink 三层顺序：精确相对路径（可省扩展名）→ 文件名 → 标题；每层须唯一命中。
func resolveLink(target string, candidates []linkTarget) (resolved bool, path string, candCount int) {
	needle := strings.ToLower(strings.TrimSpace(target))
	if needle == "" {
		return false, "", 0
	}

	var byPath []linkTarget
	for _, c := range candidates {
		p := strings.ToLower(c.path)
		if p == needle || stripExtension(p) == stripExtension(needle) {
			byPath = append(byPath, c)
		}
	}
	if len(byPath) == 1 {
		return true, byPath[0].path, 1
	}
	if len(byPath) > 1 {
		return false, "", len(byPath)
	}

	var byName []linkTarget
	for _, c := range candidates {
		base := baseName(strings.ToLower(c.path))
		if stripExtension(base) == stripExtension(needle) {
			byName = append(byName, c)
		}
	}
	if len(byName) == 1 {
		return true, byName[0].path, 1
	}
	if len(byName) > 1 {
		return false, "", len(byName)
	}

	var byTitle []linkTarget
	for _, c := range candidates {
		if strings.ToLower(strings.TrimSpace(c.title)) == needle {
			byTitle = append(byTitle, c)
		}
	}
	if len(byTitle) == 1 {
		return true, byTitle[0].path, 1
	}
	if len(byTitle) > 1 {
		return false, "", len(byTitle)
	}

	return false, "", 0
}

type linkTarget struct {
	path  string
	title string
}

func baseName(p string) string {
	if i := strings.LastIndex(p, "/"); i != -1 {
		return p[i+1:]
	}
	return p
}

func stripExtension(name string) string {
	dot := strings.LastIndex(name, ".")
	slash := strings.LastIndex(name, "/")
	if dot > slash && dot > 0 {
		return name[:dot]
	}
	return name
}
