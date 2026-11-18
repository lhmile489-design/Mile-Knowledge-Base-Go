// tokenize.go — 移植旧版 src/shared/tokenize.ts 与 src/main/services/tokenize.ts 的分词规则。
// 规则对齐点（逐条对应 tokenize.ts:33-91、tokenizePath:110-123、buildExcerpt:162-178）：
//   1. 输入截断到 200000 字符（JS 按 UTF-16 code unit，这里按 rune 截断，见 RESULT.md 差异注记）。
//   2. 连续 CJK 段切成二元组；单字保留；不跨标点段。
//   3. 英文数字词内保留 . - + #（仅在后面紧跟词字符时）；转小写。
//   4. 去重，保持首次出现顺序。
//   5. 路径分词：每段产出 整段小写 / 分词结果 / 按 .-_ 再拆分 三种形态。
package main

import (
	"strings"
	"unicode/utf8"
)

const maxTokenizeLength = 200_000

// isCjk 对应旧版正则 /[぀-ヿ㐀-䶿一-鿿豈-﫿]/：
// 假名(0x3040-0x30FF) + CJK 扩展 A(0x3400-0x4DBF) + 统一表意(0x4E00-0x9FFF) + 兼容表意(0xF900-0xFAFF)。
func isCjk(r rune) bool {
	return (r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func isInnerWordChar(r rune) bool {
	return r == '.' || r == '-' || r == '+' || r == '#'
}

// tokenize 返回去重后的小写 token 列表，顺序为首次出现顺序。
func tokenize(input string) []string {
	if len(input) == 0 {
		return nil
	}
	text := truncateRunes(input, maxTokenizeLength)

	seen := map[string]struct{}{}
	var out []string
	add := func(tok string) {
		if _, ok := seen[tok]; !ok {
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}

	runes := []rune(text)
	i := 0
	for i < len(runes) {
		r := runes[i]

		if isCjk(r) {
			j := i
			for j < len(runes) && isCjk(runes[j]) {
				j++
			}
			run := runes[i:j]
			if len(run) == 1 {
				add(string(run))
			} else {
				for k := 0; k+1 < len(run); k++ {
					add(string(run[k : k+2]))
				}
			}
			i = j
			continue
		}

		if isWordChar(r) {
			j := i
			for j < len(runes) {
				c := runes[j]
				if isWordChar(c) {
					j++
					continue
				}
				// 内部符号只在后面紧跟词字符时并入词内（v4.1 vs 句末 end.）
				if isInnerWordChar(c) && j+1 < len(runes) && isWordChar(runes[j+1]) {
					j++
					continue
				}
				break
			}
			add(strings.ToLower(string(runes[i:j])))
			i = j
			continue
		}

		i++
	}
	return out
}

// tokenizePath 对应旧版 main/services/tokenize.ts:tokenizePath。
// 路径按 / 分段，每段产出三种 token：整段小写、分词结果、再按 .-_ 拆分的结果。
func tokenizePath(path string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(tok string) {
		if tok == "" {
			return
		}
		if _, ok := seen[tok]; !ok {
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}

	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			continue
		}
		add(strings.ToLower(seg))
		for _, tok := range tokenize(seg) {
			add(tok)
			for _, part := range strings.FieldsFunc(tok, func(r rune) bool {
				return r == '.' || r == '-' || r == '_'
			}) {
				add(part)
			}
		}
	}
	return out
}

// truncateRunes 按 rune 数截断（JS 是 UTF-16 code unit；BMP 内一致，见 RESULT.md 差异注记）。
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}
	return s
}

// buildExcerpt 对应旧版 main/services/tokenize.ts:buildExcerpt。
// 取首个命中 token 位置前后各 radius 字符；取不到返回空串，绝不伪造。
// 注意旧版按 UTF-16 下标切片，这里按 rune 下标（BMP 中文一致）。
func buildExcerpt(body string, queryTokens []string, radius int) string {
	if body == "" || len(queryTokens) == 0 {
		return ""
	}
	haystack := strings.ToLower(body)

	best := -1
	for _, tok := range queryTokens {
		if at := strings.Index(haystack, strings.ToLower(tok)); at != -1 && (best == -1 || at < best) {
			best = at
		}
	}
	if best == -1 {
		return ""
	}

	// best 是字节下标；转成 rune 上下文切片
	bodyRunes := []rune(body)
	bytePos := 0
	runePos := 0
	for bytePos < best && runePos < len(bodyRunes) {
		bytePos += utf8.RuneLen(bodyRunes[runePos])
		runePos++
	}
	start := runePos - radius
	if start < 0 {
		start = 0
	}
	end := runePos + radius
	if end > len(bodyRunes) {
		end = len(bodyRunes)
	}
	prefix := ""
	if start > 0 {
		prefix = "…"
	}
	suffix := ""
	if end < len(bodyRunes) {
		suffix = "…"
	}
	seg := string(bodyRunes[start:end])
	seg = strings.Join(strings.Fields(seg), " ")
	return prefix + seg + suffix
}

// highlightRange 半开区间 [start, end)，按原文 rune 下标。
type highlightRange struct {
	start int
	end   int
}

// highlightRanges 对应旧版 shared/search.ts:highlightRanges。
// 大小写不敏感字面匹配；区间排序去重叠合并。返回 rune 下标（JS 为 UTF-16 下标）。
func highlightRanges(text string, queryTokens []string) []highlightRange {
	if text == "" || len(queryTokens) == 0 {
		return nil
	}
	// 在 rune 层做大小写不敏感字面查找
	lowerRunes := []rune(strings.ToLower(text))
	var raw []highlightRange
	for _, tok := range queryTokens {
		needle := []rune(strings.ToLower(tok))
		if len(needle) == 0 {
			continue
		}
		for i := 0; i+len(needle) <= len(lowerRunes); i++ {
			match := true
			for k := 0; k < len(needle); k++ {
				if lowerRunes[i+k] != needle[k] {
					match = false
					break
				}
			}
			if match {
				raw = append(raw, highlightRange{i, i + len(needle)})
				i += len(needle) - 1
			}
		}
	}
	if len(raw) == 0 {
		return nil
	}
	// 排序（start 升序、end 升序）
	for i := 1; i < len(raw); i++ {
		for j := i; j > 0 && (raw[j].start < raw[j-1].start || (raw[j].start == raw[j-1].start && raw[j].end < raw[j-1].end)); j-- {
			raw[j], raw[j-1] = raw[j-1], raw[j]
		}
	}
	var merged []highlightRange
	for _, r := range raw {
		if len(merged) > 0 && r.start <= merged[len(merged)-1].end {
			if r.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = r.end
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}
