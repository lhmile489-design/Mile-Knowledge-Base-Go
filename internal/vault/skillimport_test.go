package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from the legacy tests/unit/skillImport.test.ts cases.

func writeSkillFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProbeFolderIgnoresOnlyOSJunk(t *testing.T) {
	empty := t.TempDir()
	writeSkillFile(t, empty, ".DS_Store", "x")
	writeSkillFile(t, empty, "Thumbs.db", "x")
	writeSkillFile(t, empty, "desktop.ini", "x")
	if count, isempty, err := ProbeFolder(empty); err != nil || !isempty || count != 0 {
		t.Fatalf("junk only: count=%d empty=%v err=%v", count, isempty, err)
	}

	repo := t.TempDir()
	writeSkillFile(t, repo, ".git/HEAD", "ref")
	if count, isempty, err := ProbeFolder(repo); err != nil || isempty || count != 1 {
		t.Fatalf("dot dir counts as content: count=%d empty=%v err=%v", count, isempty, err)
	}
}

func TestIsSingleSegmentName(t *testing.T) {
	accept := []string{"vue", "react-native", strings.Repeat("a", 80), "技能"}
	for _, name := range accept {
		if !IsSingleSegmentName(name) {
			t.Errorf("%q rejected", name)
		}
	}
	reject := []string{"", ".hidden", "a/b", `a\b`, "..", ".", strings.Repeat("a", 81), "a\x00b", "a:b"}
	for _, name := range reject {
		if IsSingleSegmentName(name) {
			t.Errorf("%q accepted", name)
		}
	}
}

func TestPreviewSkillSourceSelfIsSkill(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my-skill")
	writeSkillFile(t, root, "SKILL.md", "# 技能\n")
	writeSkillFile(t, root, "refs/a.md", "参考\n")
	preview, err := PreviewSkillSource(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || preview.Candidates[0].Name != "my-skill" || preview.Candidates[0].FileCount != 2 {
		t.Fatalf("candidates = %+v", preview.Candidates)
	}
	if rel := preview.Dirs["my-skill"]; rel != "" {
		t.Fatalf("dirs = %+v, want self as empty rel", preview.Dirs)
	}
}

func TestPreviewSkillSourceReadsOneLevelOnly(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "vue/SKILL.md", "# vue\n")
	writeSkillFile(t, root, "go/SKILL.md", "# go\n")
	writeSkillFile(t, root, "go/refs/x.md", "x\n")
	writeSkillFile(t, root, ".git/SKILL.md", "# 不该出现\n")
	writeSkillFile(t, root, "deep/nested/SKILL.md", "# 不该出现\n")
	writeSkillFile(t, root, "plain/readme.md", "没有 SKILL.md\n")

	preview, err := PreviewSkillSource(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 2 {
		t.Fatalf("candidates = %+v", preview.Candidates)
	}
	if preview.Candidates[0].Name != "go" || preview.Candidates[0].FileCount != 2 {
		t.Errorf("go = %+v", preview.Candidates[0])
	}
	if preview.Candidates[1].Name != "vue" || preview.Candidates[1].FileCount != 1 {
		t.Errorf("vue = %+v", preview.Candidates[1])
	}
}

func TestPreviewDropsOversizeCandidate(t *testing.T) {
	root := t.TempDir()
	writeSkillFile(t, root, "small/SKILL.md", "# small\n")
	writeSkillFile(t, root, "huge/SKILL.md", "# huge\n")
	dir := filepath.Join(root, "huge")
	for i := 0; i < MaxSkillFiles+1; i++ {
		name := filepath.Join(dir, fmt.Sprintf("f%04d.txt", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := PreviewSkillSource(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || preview.Candidates[0].Name != "small" {
		t.Fatalf("candidates = %+v, want only small", preview.Candidates)
	}
	if _, ok := preview.Dirs["huge"]; ok {
		t.Fatal("oversize candidate stayed in dirs")
	}
}

func TestImportSkillsCopiesAndSkips(t *testing.T) {
	source := t.TempDir()
	writeSkillFile(t, source, "vue/SKILL.md", "# vue\n")
	writeSkillFile(t, source, "vue/refs/style.md", "样式\n")
	preview, err := PreviewSkillSource(source)
	if err != nil {
		t.Fatal(err)
	}
	vaultRoot := t.TempDir()
	// 已存在的目标绝不覆盖。
	writeSkillFile(t, vaultRoot, "skills/前端/vue/SKILL.md", "手改过\n")

	outcome, f := ImportSkills(vaultRoot, source, preview, "前端", []string{"vue", "nope"})
	if f != nil {
		t.Fatalf("failure = %+v", f)
	}
	if len(outcome.Imported) != 0 {
		t.Fatalf("imported = %v, want none", outcome.Imported)
	}
	if len(outcome.Skipped) != 2 || outcome.Skipped[0].Reason != "exists" || outcome.Skipped[1].Reason != "unknown" {
		t.Fatalf("skipped = %+v", outcome.Skipped)
	}
	body, _ := os.ReadFile(filepath.Join(vaultRoot, "skills", "前端", "vue", "SKILL.md"))
	if string(body) != "手改过\n" {
		t.Fatalf("existing skill overwritten: %q", body)
	}

	// 换个分类再导一次（令牌不销毁的对应场景）。
	outcome, f = ImportSkills(vaultRoot, source, preview, "前端2", []string{"vue"})
	if f != nil || len(outcome.Imported) != 1 {
		t.Fatalf("second import: %+v %+v", outcome, f)
	}
	body, _ = os.ReadFile(filepath.Join(vaultRoot, "skills", "前端2", "vue", "refs", "style.md"))
	if string(body) != "样式\n" {
		t.Fatalf("nested file = %q", body)
	}
}

func TestImportSkillsRejectsBadCategory(t *testing.T) {
	source := t.TempDir()
	writeSkillFile(t, source, "vue/SKILL.md", "# vue\n")
	preview, _ := PreviewSkillSource(source)
	vaultRoot := t.TempDir()
	_, f := ImportSkills(vaultRoot, source, preview, "../escape", []string{"vue"})
	if f == nil || f.Code != "invalid-input" {
		t.Fatalf("failure = %+v", f)
	}
	if _, err := os.Stat(filepath.Join(vaultRoot, "skills")); err == nil {
		t.Fatal("bad category touched the vault")
	}
}

func TestImportSkillsRollsBackOnCopyFailure(t *testing.T) {
	source := t.TempDir()
	writeSkillFile(t, source, "vue/SKILL.md", "# vue\n")
	writeSkillFile(t, source, "vue/refs/a.md", "a\n")
	preview, _ := PreviewSkillSource(source)
	vaultRoot := t.TempDir()
	// 预置一个同名兄弟，用来证明回滚只删本次新建的目标目录。
	writeSkillFile(t, vaultRoot, "skills/前端/other/SKILL.md", "# other\n")

	original := copySkillFile
	copySkillFile = func(src, dst string) error { return fmt.Errorf("disk full") }
	t.Cleanup(func() { copySkillFile = original })

	outcome, f := ImportSkills(vaultRoot, source, preview, "前端", []string{"vue"})
	if f != nil {
		t.Fatalf("failure = %+v", f)
	}
	if len(outcome.Skipped) != 1 || outcome.Skipped[0].Reason != "failed" {
		t.Fatalf("skipped = %+v", outcome.Skipped)
	}
	if _, err := os.Stat(filepath.Join(vaultRoot, "skills", "前端", "vue")); !os.IsNotExist(err) {
		t.Fatalf("failed target not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(vaultRoot, "skills", "前端", "other", "SKILL.md")); err != nil {
		t.Fatalf("sibling damaged: %v", err)
	}
}

func TestImportSkillsSkipsSymlinks(t *testing.T) {
	source := t.TempDir()
	writeSkillFile(t, source, "vue/SKILL.md", "# vue\n")
	target := filepath.Join(t.TempDir(), "outside.md")
	writeSkillFile(t, filepath.Dir(target), filepath.Base(target), "外部\n")
	if err := os.Symlink(target, filepath.Join(source, "vue", "link.md")); err != nil {
		t.Skipf("symlinks unavailable in this environment: %v", err)
	}
	preview, _ := PreviewSkillSource(source)
	vaultRoot := t.TempDir()
	if _, f := ImportSkills(vaultRoot, source, preview, "前端", []string{"vue"}); f != nil {
		t.Fatal(f)
	}
	if _, err := os.Lstat(filepath.Join(vaultRoot, "skills", "前端", "vue", "link.md")); err == nil {
		t.Fatal("symlink was copied")
	}
}
