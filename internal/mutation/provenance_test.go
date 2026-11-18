package mutation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvenanceEmptySerializationIsExact(t *testing.T) {
	root := t.TempDir()
	opened, err := OpenProvenance(root)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"version\": 1,\n  \"records\": []\n}\n"
	if got := opened.Serialize(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestProvenanceStableOrderAndEscaping(t *testing.T) {
	root := t.TempDir()
	store, err := OpenProvenance(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Record("projects/b.md", "raws/b.md", 2, "hash-b"); err != nil {
		t.Fatal(err)
	}
	if err := store.Record("projects/a<b>.md", "raws/a.md", 1, "hash-a"); err != nil {
		t.Fatal(err)
	}
	text := store.Serialize()
	if strings.Index(text, "projects/a<b>.md") > strings.Index(text, "projects/b.md") {
		t.Errorf("records not sorted by copyPath:\n%s", text)
	}
	if strings.Contains(text, "\\u003c") {
		t.Errorf("HTML characters escaped:\n%s", text)
	}
	keyOrder := []string{`"copyPath"`, `"sourcePath"`, `"baseRevision"`, `"baseHash"`, `"copiedAt"`}
	last := -1
	for _, key := range keyOrder {
		at := strings.Index(text, key)
		if at < last {
			t.Errorf("key order broken at %s:\n%s", key, text)
		}
		last = at
	}
	if !strings.HasSuffix(text, "\n") {
		t.Error("missing trailing newline")
	}
}

func TestProvenanceKeepsUnknownFields(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".mile-kb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{
  "version": 1,
  "futureTop": {"a": 1},
  "records": [
    {"copyPath": "projects/a.md", "sourcePath": "raws/a.md", "baseRevision": 1, "baseHash": "h", "copiedAt": "t", "futureField": "keep"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "provenance.json"), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenProvenance(root)
	if err != nil {
		t.Fatal(err)
	}
	text := store.Serialize()
	if !strings.Contains(text, `"futureTop": {"a": 1}`) {
		t.Errorf("top-level unknown dropped:\n%s", text)
	}
	if !strings.Contains(text, `"futureField": "keep"`) {
		t.Errorf("record unknown dropped:\n%s", text)
	}
}

func TestProvenanceRecordPersistAndForget(t *testing.T) {
	root := t.TempDir()
	store, _ := OpenProvenance(root)
	if err := store.Record("projects/a.md", "raws/a.md", 3, "hash"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := OpenProvenance(root)
	if err != nil {
		t.Fatal(err)
	}
	record, ok := reloaded.Get("projects/a.md")
	if !ok || record.BaseRevision != 3 || record.BaseHash != "hash" {
		t.Fatalf("record = %+v ok=%v", record, ok)
	}
	if err := reloaded.UpdateBaseline("projects/a.md", 4, "hash2"); err != nil {
		t.Fatal(err)
	}
	updated, _ := reloaded.Get("projects/a.md")
	if updated.BaseRevision != 4 || updated.BaseHash != "hash2" {
		t.Errorf("baseline not updated: %+v", updated)
	}
	if err := reloaded.Forget("projects/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Get("projects/a.md"); ok {
		t.Error("record not removed")
	}
}

func TestAssessAllStates(t *testing.T) {
	base := ProvenanceRecord{CopyPath: "projects/a.md", SourcePath: "raws/a.md", BaseHash: "h"}
	ptr := func(v string) *string { return &v }
	cases := []struct {
		name        string
		record      ProvenanceRecord
		source      *string
		copy        *string
		want        string
		wantCanSync bool
	}{
		{"unknown without baseline", ProvenanceRecord{}, ptr("h"), ptr("h"), "unknown", false},
		{"copy missing", base, ptr("h"), nil, "copy-missing", false},
		{"source missing", base, nil, ptr("h"), "source-missing", false},
		{"up to date", base, ptr("h"), ptr("h"), "up-to-date", false},
		{"source changed", base, ptr("h2"), ptr("h"), "source-changed", true},
		{"copy modified", base, ptr("h"), ptr("h2"), "copy-modified", false},
		{"both changed", base, ptr("h2"), ptr("h3"), "both-changed", false},
	}
	for _, c := range cases {
		state, canSync, reason := Assess(c.record, c.source, c.copy)
		if state != c.want || canSync != c.wantCanSync || reason == "" {
			t.Errorf("%s: %s %v %q", c.name, state, canSync, reason)
		}
	}
}
