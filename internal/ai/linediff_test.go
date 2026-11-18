package ai

import (
	"strings"
	"testing"
)

func TestLineDiff(t *testing.T) {
	if diff := LineDiff("same", "same"); len(diff) != 1 || diff[0].Kind != "same" {
		t.Fatalf("identical: %+v", diff)
	}
	// First argument is the old text, second the new one: removed lines come
	// from the old text, added lines from the new.
	diff := LineDiff("a\nb\nc", "a\nB\nc")
	kinds := []string{diff[0].Kind, diff[1].Kind, diff[2].Kind, diff[3].Kind}
	want := []string{"same", "removed", "added", "same"}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds: %v", kinds)
		}
	}
	if diff[1].Text != "b" || diff[2].Text != "B" {
		t.Fatalf("lines: %+v", diff)
	}

	added := LineDiff("new", "")
	if len(added) != 2 || added[0].Kind != "removed" || added[1].Kind != "added" {
		t.Fatalf("empty disk: %+v", added)
	}
}

func TestBuildWritePreviewCreated(t *testing.T) {
	preview := BuildWritePreview("one\ntwo", nil, 7)
	if preview.Kind != "created" || len(preview.Lines) != 2 || preview.NewLineCount != 2 || preview.NewBytes != 7 {
		t.Fatalf("created: %+v", preview)
	}
	for _, line := range preview.Lines {
		if line.Kind != "added" {
			t.Fatalf("created lines must be added: %+v", line)
		}
	}

	big := strings.Repeat("line\n", 100)
	if summary := BuildWritePreview(big, nil, 500); summary.Kind != "summary" || summary.Reason != "too-large" {
		t.Fatalf("too large create: %+v", summary)
	}
}

func TestBuildWritePreviewModified(t *testing.T) {
	old := "a\nb\nc"
	preview := BuildWritePreview("a\nB\nc", &old, 5)
	if preview.Kind != "modified" || preview.OldLineCount != 3 || preview.NewLineCount != 3 {
		t.Fatalf("modified: %+v", preview)
	}
	if preview.OldBytes != int64(len(old)) || preview.NewBytes != 5 {
		t.Fatalf("bytes: %+v", preview)
	}

	unchanged := BuildWritePreview(old, &old, 5)
	if unchanged.Kind != "summary" || unchanged.Reason != "unchanged" {
		t.Fatalf("unchanged: %+v", unchanged)
	}

	bigOld := strings.Repeat("x\n", 80)
	bigNew := strings.Repeat("y\n", 80)
	summary := BuildWritePreview(bigNew, &bigOld, 160)
	if summary.Kind != "summary" || summary.Reason != "too-large" {
		t.Fatalf("too large diff: %+v", summary)
	}
}

func TestBuildWritePreviewClipsLongLines(t *testing.T) {
	long := strings.Repeat("字", 300)
	preview := BuildWritePreview(long, nil, 900)
	if preview.Kind != "created" {
		t.Fatalf("kind: %s", preview.Kind)
	}
	line := preview.Lines[0].Text
	if len([]rune(line)) != PreviewMaxLineChars+1 || !strings.HasSuffix(line, "…") {
		t.Fatalf("clip: %d", len([]rune(line)))
	}
}
