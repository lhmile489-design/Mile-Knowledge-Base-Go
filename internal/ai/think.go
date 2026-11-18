package ai

import "strings"

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

type thinkState int

const (
	thinkProbe thinkState = iota
	thinkInside
	thinkPassthrough
)

// SplitPiece is the reasoning/text split of one push.
type SplitPiece struct {
	Reasoning string
	Text      string
}

// ThinkSplitter moves a leading <think>…</think> block out of the answer. Only
// the very start of the answer is examined, so quoting the tag later does not
// hijack the stream (thinkSplitter.ts:1-81).
type ThinkSplitter struct {
	state thinkState
	buf   []rune
}

// Push consumes one delta.
func (t *ThinkSplitter) Push(s string) SplitPiece {
	if t.state == thinkPassthrough {
		return SplitPiece{Text: s}
	}
	if t.state == thinkInside {
		return t.scanClose(s)
	}

	t.buf = append(t.buf, []rune(s)...)
	trimmed := strings.TrimLeft(string(t.buf), " \t\r\n")
	if len([]rune(trimmed)) < len([]rune(thinkOpen)) {
		if strings.HasPrefix(thinkOpen, trimmed) {
			return SplitPiece{}
		}
		return SplitPiece{Text: t.drainPassthrough()}
	}
	if strings.HasPrefix(trimmed, thinkOpen) {
		t.state = thinkInside
		t.buf = nil
		return t.scanClose(trimmed[len(thinkOpen):])
	}
	return SplitPiece{Text: t.drainPassthrough()}
}

// Flush emits whatever is held when the stream ends. An unclosed block counts
// as reasoning.
func (t *ThinkSplitter) Flush() SplitPiece {
	if len(t.buf) == 0 {
		return SplitPiece{}
	}
	out := string(t.buf)
	t.buf = nil
	if t.state == thinkInside {
		return SplitPiece{Reasoning: out}
	}
	return SplitPiece{Text: out}
}

func (t *ThinkSplitter) scanClose(s string) SplitPiece {
	t.buf = append(t.buf, []rune(s)...)
	text := string(t.buf)
	if index := strings.Index(text, thinkClose); index >= 0 {
		reasoning := text[:index]
		rest := strings.TrimLeft(text[index+len(thinkClose):], " \t\r\n")
		t.buf = nil
		t.state = thinkPassthrough
		return SplitPiece{Reasoning: reasoning, Text: rest}
	}
	keep := markerSuffixLen(text, thinkClose)
	if keep == 0 {
		t.buf = nil
		return SplitPiece{Reasoning: text}
	}
	runes := []rune(text)
	t.buf = append([]rune(nil), runes[len(runes)-keep:]...)
	return SplitPiece{Reasoning: string(runes[:len(runes)-keep])}
}

func (t *ThinkSplitter) drainPassthrough() string {
	t.state = thinkPassthrough
	out := string(t.buf)
	t.buf = nil
	return out
}

// markerSuffixLen is the longest real suffix of s that is a proper prefix of
// marker; those runes must be held until the next delta completes the marker.
func markerSuffixLen(s, marker string) int {
	max := len(marker) - 1
	if len(s) < max {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasPrefix(marker, s[len(s)-n:]) {
			return n
		}
	}
	return 0
}
