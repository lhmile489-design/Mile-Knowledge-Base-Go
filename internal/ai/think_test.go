package ai

import "testing"

func TestThinkSplitterPassthrough(t *testing.T) {
	splitter := &ThinkSplitter{}
	piece := splitter.Push("普通回答")
	if piece.Text != "普通回答" || piece.Reasoning != "" {
		t.Fatalf("passthrough: %+v", piece)
	}
	if out := splitter.Flush(); out.Text != "" || out.Reasoning != "" {
		t.Fatalf("flush after passthrough: %+v", out)
	}
}

func TestThinkSplitterBare(t *testing.T) {
	splitter := &ThinkSplitter{}
	var reasoning, text string
	for _, chunk := range []string{"<thi", "nk>先想", "一想</thi", "nk>然后回答"} {
		piece := splitter.Push(chunk)
		reasoning += piece.Reasoning
		text += piece.Text
	}
	piece := splitter.Flush()
	reasoning += piece.Reasoning
	text += piece.Text
	if reasoning != "先想一想" {
		t.Fatalf("reasoning: %q", reasoning)
	}
	if text != "然后回答" {
		t.Fatalf("text: %q", text)
	}
}

func TestThinkSplitterUnclosedIsReasoning(t *testing.T) {
	splitter := &ThinkSplitter{}
	piece := splitter.Push("<think>没闭合")
	out := splitter.Flush()
	if piece.Reasoning+out.Reasoning != "没闭合" || piece.Text+out.Text != "" {
		t.Fatalf("unclosed: %+v %+v", piece, out)
	}
}

func TestThinkSplitterOnlyAtStart(t *testing.T) {
	splitter := &ThinkSplitter{}
	splitter.Push("正文里提到 ")
	piece := splitter.Push("<think>不算思考</think>")
	if piece.Reasoning != "" || piece.Text != "<think>不算思考</think>" {
		t.Fatalf("mid-answer tag must pass through: %+v", piece)
	}
}

func TestThinkSplitterLeadingWhitespace(t *testing.T) {
	splitter := &ThinkSplitter{}
	piece := splitter.Push("\n  <think>想</think>答")
	if piece.Reasoning != "想" || piece.Text != "答" {
		t.Fatalf("leading whitespace: %+v", piece)
	}
}
