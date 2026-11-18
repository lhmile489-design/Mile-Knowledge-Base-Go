package index

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestVault builds a small vault that covers every branch of the build:
// text, frontmatter, system area, excluded directories, a binary file and an
// oversize file.
func newTestVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("raws/首页.md", "---\ntitle: 首页标题\ntags: [demo, 排版]\n---\n\n# 首页\n\n知识库管理正文。\n")
	write("skills/react/SKILL.md", "# React 技能\n\n状态管理。\n")
	write("raws/code.txt", "const answer = 42\n")
	write("raws/单字.md", "# 单字\n\n库\n")
	write(".claude/notes.md", "# 系统区笔记\n\n不应出现在默认检索。\n")
	write(".mile-kb/settings/aliases.json", `{"a":"b"}`)
	write("node_modules/pkg/index.js", "module.exports = 1\n")

	binary := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, 0xff, 0xfe, 0xfd)
	if err := os.WriteFile(filepath.Join(root, "raws", "pic.png"), binary, 0o644); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("大", (MaxIndexedBytes/3)+64)
	if err := os.WriteFile(filepath.Join(root, "raws", "huge.md"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenIndex(filepath.Join(t.TempDir(), "index", "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestBuildIndexesTextAndSkipsBodies(t *testing.T) {
	ctx := context.Background()
	root := newTestVault(t)
	store := openTestStore(t)

	var last Status
	if err := Build(ctx, store, root, func(st Status) { last = st }); err != nil {
		t.Fatal(err)
	}
	if last.Phase != "ready" {
		t.Fatalf("phase %q", last.Phase)
	}
	// Seven files outside the excluded directories. skipped counts only text
	// files whose body could not be read (the oversize one here); a binary
	// file is indexed by path without being "skipped" (legacy semantics).
	if last.DocumentCount != 7 {
		t.Errorf("documentCount = %d, want 7", last.DocumentCount)
	}
	if last.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (oversize markdown)", last.Skipped)
	}
	if last.TokenCount <= 0 {
		t.Errorf("tokenCount = %d", last.TokenCount)
	}

	for _, rel := range []string{".mile-kb/settings/aliases.json", "node_modules/pkg/index.js"} {
		ok, err := store.PathExists(ctx, rel)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Errorf("%s must not be indexed", rel)
		}
	}

	meta, ok, err := store.Meta(ctx, "raws/首页.md")
	if err != nil || !ok {
		t.Fatalf("meta: %v %v", ok, err)
	}
	if meta.Title != "首页标题" || meta.Area != "raws" || meta.Extension != "md" || meta.Ignored {
		t.Errorf("meta = %+v", meta)
	}
	if !meta.BodyExists {
		t.Error("body should be stored for a text file")
	}

	body, ok, err := store.BodyText(ctx, "raws/首页.md")
	if err != nil || !ok || !strings.Contains(body, "知识库管理正文") {
		t.Fatalf("body %q %v %v", body, ok, err)
	}

	system, _, err := store.Meta(ctx, ".claude/notes.md")
	if err != nil {
		t.Fatal(err)
	}
	if !system.Ignored {
		t.Error("dot directories belong to the system area and must be ignored")
	}

	binaryMeta, _, err := store.Meta(ctx, "raws/pic.png")
	if err != nil {
		t.Fatal(err)
	}
	if binaryMeta.BodyExists {
		t.Error("binary file must not store a body")
	}
}

func TestBuildFeedsFTSWithBigrams(t *testing.T) {
	ctx := context.Background()
	root := newTestVault(t)
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}

	hits, err := store.FTSQuery(ctx, "知识", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "raws/首页.md" {
		t.Fatalf("hits = %v", hits)
	}
	// A lone CJK character is its own token. "库" alone must not match the
	// document that only contains it inside the bigram run (legacy behaviour).
	if hits, err = store.FTSQuery(ctx, "库", 10); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "raws/单字.md" {
		t.Fatalf("single-char hits = %v", hits)
	}
	// English words still match.
	if hits, err = store.FTSQuery(ctx, "answer", 10); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "raws/code.txt" {
		t.Fatalf("english hits = %v", hits)
	}
}

func TestUpdateAppliesAddsChangesAndDeletes(t *testing.T) {
	ctx := context.Background()
	root := newTestVault(t)
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	before, err := store.Revision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Change one file, delete another, add a third.
	if err := os.WriteFile(filepath.Join(root, "raws", "code.txt"), []byte("const renamed = 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "skills", "react", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raws", "新增.md"), []byte("# 新增\n\n全新内容。\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Update(ctx, store, root, []string{"raws", "skills/react"}, nil); err != nil {
		t.Fatal(err)
	}

	if hits, err := store.FTSQuery(ctx, "renamed", 10); err != nil || len(hits) != 1 {
		t.Fatalf("changed file not reindexed: %v %v", hits, err)
	}
	if hits, _ := store.FTSQuery(ctx, "answer", 10); len(hits) != 0 {
		t.Fatalf("stale tokens survived: %v", hits)
	}
	if ok, _ := store.PathExists(ctx, "skills/react/SKILL.md"); ok {
		t.Fatal("deleted file still indexed")
	}
	if ok, _ := store.PathExists(ctx, "raws/新增.md"); !ok {
		t.Fatal("new file not indexed")
	}
	after, err := store.Revision(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Errorf("revision %d -> %d, want exactly one bump", before, after)
	}
}

func TestUpdateWithoutChangesKeepsRevision(t *testing.T) {
	ctx := context.Background()
	root := newTestVault(t)
	store := openTestStore(t)
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Revision(ctx)
	if err := Update(ctx, store, root, []string{"raws"}, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := store.Revision(ctx)
	if after != before {
		t.Errorf("revision changed without changes: %d -> %d", before, after)
	}
}

func TestReopenKeepsIndexAndRecoversFromSchemaMismatch(t *testing.T) {
	ctx := context.Background()
	root := newTestVault(t)
	path := filepath.Join(t.TempDir(), "vault.db")
	store, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Build(ctx, store, root, nil); err != nil {
		t.Fatal(err)
	}
	count, _ := store.DocumentCount(ctx)
	_ = store.Close()

	reopened, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := reopened.DocumentCount(ctx)
	if again != count {
		t.Fatalf("reopen lost rows: %d -> %d", count, again)
	}
	_ = reopened.Close()

	// A database from a future schema is dropped and rebuilt empty.
	future, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := future.setMeta("schema_version", "99"); err != nil {
		t.Fatal(err)
	}
	_ = future.Close()

	recovered, err := OpenIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	empty, _ := recovered.DocumentCount(ctx)
	if empty != 0 {
		t.Fatalf("schema mismatch should rebuild empty, got %d rows", empty)
	}
}

func TestNormaliseDirs(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"a/b", "a", "a/b/c"}, []string{"a"}},
		{[]string{"", "a"}, []string{""}},
		{[]string{"../x", "a"}, []string{"a"}},
		{[]string{`a\b`, "a/b"}, []string{"a/b"}},
	}
	for _, c := range cases {
		got := normaliseDirs(c.in)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("normaliseDirs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
