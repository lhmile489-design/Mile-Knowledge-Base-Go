package ai

import "github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"

// Preview limits (lineDiff.ts:93-98).
const (
	MaxLCSHistoryCells  = 4_000_000
	PreviewMaxLines     = 60
	PreviewMaxLineChars = 200
)

// LineDiff returns the full aligned diff of mine against disk. mine is the
// new text: its lines show as removed against disk and added from mine
// (lineDiff.ts:33-91). A huge input degrades to "all removed, then all added"
// rather than guessing an alignment.
func LineDiff(mine, disk string) []contract.DiffLine {
	if mine == disk {
		if mine == "" {
			return nil
		}
		return []contract.DiffLine{{Kind: "same", Text: mine}}
	}
	a := splitLines(mine)
	b := splitLines(disk)
	if int64(len(a))*int64(len(b)) > MaxLCSHistoryCells {
		out := make([]contract.DiffLine, 0, len(a)+len(b))
		for _, text := range a {
			out = append(out, contract.DiffLine{Kind: "removed", Text: text})
		}
		for _, text := range b {
			out = append(out, contract.DiffLine{Kind: "added", Text: text})
		}
		return out
	}

	rows, cols := len(a)+1, len(b)+1
	dp := make([]uint32, rows*cols)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i*cols+j] = dp[(i+1)*cols+j+1] + 1
			} else if dp[(i+1)*cols+j] >= dp[i*cols+j+1] {
				dp[i*cols+j] = dp[(i+1)*cols+j]
			} else {
				dp[i*cols+j] = dp[i*cols+j+1]
			}
		}
	}

	out := make([]contract.DiffLine, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, contract.DiffLine{Kind: "same", Text: a[i]})
			i++
			j++
		case dp[(i+1)*cols+j] >= dp[i*cols+j+1]:
			out = append(out, contract.DiffLine{Kind: "removed", Text: a[i]})
			i++
		default:
			out = append(out, contract.DiffLine{Kind: "added", Text: b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		out = append(out, contract.DiffLine{Kind: "removed", Text: a[i]})
	}
	for ; j < len(b); j++ {
		out = append(out, contract.DiffLine{Kind: "added", Text: b[j]})
	}
	return out
}

func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	out := make([]string, 0, 16)
	start := 0
	for index := 0; index < len(text); index++ {
		if text[index] == '\n' {
			out = append(out, text[start:index])
			start = index + 1
		}
	}
	out = append(out, text[start:])
	return out
}

// BuildWritePreview produces the approval-card preview: created when the file
// is new, modified with a full aligned diff, or a summary when the content is
// too large or unchanged (lineDiff.ts:125-175).
func BuildWritePreview(newText string, oldText *string, newBytes int64) *contract.WritePreview {
	newLineCount := 0
	if newText != "" {
		newLineCount = len(splitLines(newText))
	}
	if oldText == nil {
		if newLineCount > PreviewMaxLines {
			return &contract.WritePreview{Kind: "summary", Reason: "too-large", NewBytes: newBytes}
		}
		lines := make([]contract.DiffLine, 0, newLineCount)
		for _, text := range splitLines(newText) {
			lines = append(lines, contract.DiffLine{Kind: "added", Text: clipLine(text)})
		}
		return &contract.WritePreview{Kind: "created", Lines: lines, NewLineCount: newLineCount, NewBytes: newBytes}
	}

	oldLineCount := 0
	if *oldText != "" {
		oldLineCount = len(splitLines(*oldText))
	}
	if newText == *oldText {
		return &contract.WritePreview{Kind: "summary", Reason: "unchanged", NewBytes: newBytes}
	}
	diff := LineDiff(*oldText, newText)
	if len(diff) > PreviewMaxLines {
		return &contract.WritePreview{
			Kind: "summary", Reason: "too-large",
			OldBytes: int64(len(*oldText)), NewBytes: newBytes,
		}
	}
	lines := make([]contract.DiffLine, 0, len(diff))
	for _, line := range diff {
		lines = append(lines, contract.DiffLine{Kind: line.Kind, Text: clipLine(line.Text)})
	}
	return &contract.WritePreview{
		Kind: "modified", Lines: lines,
		OldLineCount: oldLineCount, NewLineCount: newLineCount,
		OldBytes: int64(len(*oldText)), NewBytes: newBytes,
	}
}

func clipLine(text string) string {
	runes := []rune(text)
	if len(runes) <= PreviewMaxLineChars {
		return text
	}
	return string(runes[:PreviewMaxLineChars]) + "…"
}
