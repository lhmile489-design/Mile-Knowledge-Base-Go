package mutation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func testVault(t *testing.T) string {
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
	write("raws/首页.md", "# 首页\n")
	write("raws/旧名.md", "# 旧名\n")
	write("raws/冲突.md", "# 冲突\n")
	write("raws/conflict.md", "# conflict\n")
	write("raws/目录/子.md", "# 子\n")
	write("_archives/raws/首页.md", "# 已归档\n")
	return root
}

func builderFor(t *testing.T) (Builder, string) {
	t.Helper()
	root := testVault(t)
	return Builder{Root: root, NewID: func() string { return "plan-test" }}, root
}

func plan(t *testing.T, b Builder, actions ...contract.MutationAction) (contract.MutationPlan, *contract.Failure) {
	t.Helper()
	return b.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: actions})
}

func TestCheckEntryName(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		siblings  []string
		wantOK    bool
		wantCode  string
		wantValue string
	}{
		{"plain", "  笔记  ", nil, true, "", "笔记"},
		{"empty", "   ", nil, false, contract.CodeInvalidInput, ""},
		{"illegal char", "a<b.md", nil, false, contract.CodeInvalidInput, ""},
		{"control char", "a\x01b", nil, false, contract.CodeInvalidInput, ""},
		{"dot", ".", nil, false, contract.CodeInvalidInput, ""},
		{"dotdot", "..", nil, false, contract.CodeInvalidInput, ""},
		{"trailing dot", "笔记.", nil, false, contract.CodeInvalidInput, ""},
		// A trailing space never reaches the check: the name is trimmed first
		// (legacy entryName.ts trims too).
		{"trailing space trimmed", "笔记 ", nil, true, "", "笔记"},
		{"reserved", "nul.md", nil, false, contract.CodeInvalidInput, ""},
		{"reserved case", "COM1.txt", nil, false, contract.CodeInvalidInput, ""},
		{"too long", strings.Repeat("a", 201), nil, false, contract.CodeInvalidInput, ""},
		{"collision", "README.md", []string{"readme.md"}, false, contract.CodeNameConflict, ""},
	}
	for _, c := range cases {
		got, f := CheckEntryName(c.input, c.siblings)
		if c.wantOK {
			if f != nil || got != c.wantValue {
				t.Errorf("%s: got %q %+v", c.name, got, f)
			}
			continue
		}
		if f == nil || f.Code != c.wantCode {
			t.Errorf("%s: expected %s, got %+v", c.name, c.wantCode, f)
		}
	}
}

func TestWithDefaultExtension(t *testing.T) {
	cases := map[string]string{
		"笔记":      "笔记.md",
		"笔记.md":   "笔记.md",
		"a.txt":   "a.txt",
		".hidden": ".hidden.md",
	}
	for in, want := range cases {
		if got := WithDefaultExtension(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestAssessRisk(t *testing.T) {
	create := contract.MutationAction{Kind: contract.MutCreate}
	danger := contract.MutationAction{Kind: contract.MutPermanentDelete}
	edit := contract.MutationAction{Kind: contract.MutEdit}
	if risk := AssessRisk([]contract.MutationAction{create}); risk != "low" {
		t.Errorf("create risk %s", risk)
	}
	if risk := AssessRisk([]contract.MutationAction{edit}); risk != "medium" {
		t.Errorf("edit risk %s", risk)
	}
	if risk := AssessRisk([]contract.MutationAction{danger}); risk != "high" {
		t.Errorf("permanent delete risk %s", risk)
	}
	many := make([]contract.MutationAction, 21)
	for i := range many {
		many[i] = create
	}
	if risk := AssessRisk(many); risk != "high" {
		t.Errorf("21 actions risk %s", risk)
	}
}

func TestPlanValidations(t *testing.T) {
	b, _ := builderFor(t)

	if _, f := b.Plan(contract.PlanRequest{VaultID: "v", Origin: "nobody",
		Actions: []contract.MutationAction{{ID: "a", Kind: contract.MutCreate, Target: "raws/新.md"}}}); f == nil {
		t.Error("unknown origin accepted")
	}
	if _, f := b.Plan(contract.PlanRequest{VaultID: "v", Origin: "user"}); f == nil {
		t.Error("empty plan accepted")
	}
	tooMany := make([]contract.MutationAction, MaxPlanActions+1)
	for i := range tooMany {
		tooMany[i] = contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "raws/x.md"}
	}
	if _, f := b.Plan(contract.PlanRequest{VaultID: "v", Origin: "user", Actions: tooMany}); f == nil {
		t.Error("501 actions accepted")
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "", Kind: contract.MutCreate, Target: "raws/x.md"}); f == nil {
		t.Error("empty id accepted")
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: "explode", Target: "raws/x.md"}); f == nil {
		t.Error("unknown kind accepted")
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove, Source: "../x", Target: "raws/y.md"}); f == nil {
		t.Error("escaping source accepted")
	}
	negative := int64(-1)
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutEdit, Target: "raws/首页.md",
		Text: ptr("x"), ExpectedSourceRevision: &negative}); f == nil {
		t.Error("negative revision accepted")
	}
	big := strings.Repeat("大", MaxWriteBytes/3+10)
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutEdit, Target: "raws/首页.md", Text: &big}); f == nil {
		t.Error("oversize edit accepted")
	}
}

func TestPlanCreateFile(t *testing.T) {
	b, _ := builderFor(t)
	p, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "raws/新笔记"})
	if f != nil {
		t.Fatal(f)
	}
	if p.Actions[0].Target != "raws/新笔记.md" {
		t.Errorf("extension not added: %s", p.Actions[0].Target)
	}
	if p.Risk != "low" || p.ID != "plan-test" || len(p.Actions) != 1 {
		t.Errorf("plan = %+v", p)
	}

	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "raws/首页.md"}); f == nil ||
		f.Code != contract.CodeNameConflict {
		t.Errorf("existing target accepted: %+v", f)
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "nowhere/x.md"}); f == nil {
		t.Error("missing parent accepted")
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "raws/con.md"}); f == nil {
		t.Error("reserved name accepted")
	}
}

func TestPlanCreateDirectory(t *testing.T) {
	b, _ := builderFor(t)
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreateDirectory, Target: "raws/新目录"}); f != nil {
		t.Fatal(f)
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCreateDirectory, Target: "raws/首页.md"}); f == nil {
		t.Error("directory over a file accepted")
	}
}

func TestPlanRenameRules(t *testing.T) {
	b, _ := builderFor(t)
	// Plain rename.
	p, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove,
		Source: "raws/旧名.md", Target: "raws/新名.md"})
	if f != nil || p.Actions[0].Target != "raws/新名.md" {
		t.Fatalf("rename failed: %+v %+v", p, f)
	}
	// Collision with a different name.
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove,
		Source: "raws/旧名.md", Target: "raws/冲突.md"}); f == nil || f.Code != contract.CodeNameConflict {
		t.Errorf("collision accepted: %+v", f)
	}
	// Case-only rename is the one allowed collision.
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove,
		Source: "raws/conflict.md", Target: "raws/Conflict.md"}); f != nil {
		t.Errorf("case-only rename rejected: %+v", f)
	}
	// A directory cannot move into its own subtree.
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove,
		Source: "raws/目录", Target: "raws/目录/里面"}); f == nil {
		t.Error("move into own subtree accepted")
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutMove,
		Source: "raws/不存在.md", Target: "raws/x.md"}); f == nil || f.Code != contract.CodeNotFound {
		t.Errorf("missing source accepted: %+v", f)
	}
}

func TestPlanCopyAndDestructiveKinds(t *testing.T) {
	b, _ := builderFor(t)
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCopy,
		Source: "raws/首页.md", Target: "projects/副本.md"}); f != nil {
		t.Fatalf("copy rejected: %+v", f)
	}
	if _, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutCopy,
		Source: "raws/首页.md", Target: "projects/首页.md"}); f != nil {
		t.Fatalf("copy to an existing parent rejected: %+v", f)
	}
	for _, kind := range []string{contract.MutArchive, contract.MutTrash, contract.MutPermanentDelete} {
		p, f := plan(t, b, contract.MutationAction{ID: "a", Kind: kind, Source: "raws/首页.md"})
		if f != nil {
			t.Errorf("%s rejected: %+v", kind, f)
			continue
		}
		if p.Risk == "low" {
			t.Errorf("%s should not be low risk", kind)
		}
	}
	// Archive has no target: it is derived at execution time.
	p, _ := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutArchive, Source: "raws/首页.md"})
	if p.Actions[0].Target != "" {
		t.Errorf("archive target should stay empty, got %q", p.Actions[0].Target)
	}
}

// TestPlanWritesNothing hashes the whole vault before and after a battery of
// plan calls, valid and invalid, and requires byte-identical state.
func TestPlanWritesNothing(t *testing.T) {
	b, root := builderFor(t)
	before := snapshotVault(t, root)

	calls := []contract.MutationAction{
		{ID: "a", Kind: contract.MutCreate, Target: "raws/新文件"},
		{ID: "b", Kind: contract.MutCreateDirectory, Target: "raws/新目录"},
		{ID: "c", Kind: contract.MutMove, Source: "raws/旧名.md", Target: "raws/改名.md"},
		{ID: "d", Kind: contract.MutMove, Source: "raws/conflict.md", Target: "raws/Conflict.md"},
		{ID: "e", Kind: contract.MutEdit, Source: "raws/首页.md", Text: ptr("# 新\n")},
		{ID: "f", Kind: contract.MutCopy, Source: "raws/首页.md", Target: "projects/副本.md"},
		{ID: "g", Kind: contract.MutArchive, Source: "raws/首页.md"},
		{ID: "h", Kind: contract.MutTrash, Source: "raws/首页.md"},
		{ID: "i", Kind: contract.MutPermanentDelete, Source: "raws/首页.md"},
		{ID: "j", Kind: contract.MutCreate, Target: "raws/首页.md"}, // invalid on purpose
	}
	for _, action := range calls {
		_, _ = plan(t, b, action)
	}
	if after := snapshotVault(t, root); after != before {
		t.Fatalf("planning changed the vault:\nbefore %s\nafter  %s", before, after)
	}
}

// snapshotVault hashes every relative path plus its bytes and mode.
func snapshotVault(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		line := rel + "|" + info.Mode().String()
		if !d.IsDir() {
			body, readErr := os.ReadFile(p)
			if readErr != nil {
				return readErr
			}
			sum := sha256.Sum256(body)
			line += "|" + hex.EncodeToString(sum[:])
		}
		lines = append(lines, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func ptr[T any](v T) *T { return &v }
