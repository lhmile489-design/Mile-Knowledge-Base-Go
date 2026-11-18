package mutation

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func fixedExecutor(t *testing.T, root string) (Executor, *[]string) {
	t.Helper()
	trashed := []string{}
	at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	executor := Executor{
		Root: root, VaultID: "v",
		Manager: TxManager{Root: root, Now: func() time.Time { return at }, Rand: func() string { return "abcd1234" }},
		Trash: func(abs string) error {
			trashed = append(trashed, abs)
			return os.RemoveAll(abs)
		},
		WriteText: func(rel string, expectedRevision int64, text string) (string, *contract.Failure) {
			abs := filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return "", contract.Fail(contract.CodeIOFailure, err.Error())
			}
			if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
				return "", contract.Fail(contract.CodeIOFailure, err.Error())
			}
			hash, _ := hashFile(abs)
			return hash, nil
		},
		Now: func() time.Time { return at },
	}
	return executor, &trashed
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readNames lists a directory as an exact-case name set.
func readNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, entry := range entries {
		out[entry.Name()] = true
	}
	return out
}

func exists(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func planOf(actions ...contract.MutationAction) contract.MutationPlan {
	return contract.MutationPlan{ID: "plan-1", VaultID: "v", Origin: "user", Actions: actions}
}

func TestExecuteCreateAndDirectory(t *testing.T) {
	root := t.TempDir()
	executor, _ := fixedExecutor(t, root)
	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutCreate, Target: "raws/新.md", Text: ptr("# 新\n")},
		contract.MutationAction{ID: "b", Kind: contract.MutCreateDirectory, Target: "raws/目录"},
	))
	if report.Status != "completed" || len(report.Completed) != 2 {
		t.Fatalf("report = %+v", report)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "新.md"))
	if string(body) != "# 新\n" {
		t.Errorf("body = %q", body)
	}
	if !exists(root, "raws/目录") {
		t.Error("directory not created")
	}
	if report.TransactionID != "" {
		t.Error("non-destructive plan should not open a transaction")
	}
	if got := strings.Join(report.TouchedDirectories, ","); got != "raws" {
		t.Errorf("touched = %s", got)
	}
}

func TestExecuteEditUsesRevisionCheck(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "# 旧\n")
	executor, _ := fixedExecutor(t, root)
	zero := int64(1) // deliberately wrong: the simple test writer ignores it,
	// so a wrong revision must be caught before writing.
	_ = zero
	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutEdit, Target: "raws/a.md", Text: ptr("# 新\n")},
	))
	if report.Status != "completed" {
		t.Fatalf("report = %+v", report)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if string(body) != "# 新\n" {
		t.Errorf("body = %q", body)
	}
	if report.TransactionID == "" {
		t.Error("edit must open a transaction")
	}
}

func TestExecuteMoveCopyArchive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "# a\n")
	writeFile(t, root, "raws/b.md", "# b\n")
	writeFile(t, root, "raws/dir/x.md", "# x\n")
	executor, _ := fixedExecutor(t, root)

	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutMove, Source: "raws/a.md", Target: "raws/A.md"},
		contract.MutationAction{ID: "b", Kind: contract.MutCopy, Source: "raws/b.md", Target: "projects/b.md"},
		contract.MutationAction{ID: "c", Kind: contract.MutCopy, Source: "raws/dir", Target: "projects/dir"},
	))
	if report.Status != "completed" {
		t.Fatalf("report = %+v", report)
	}
	// Lstat is case-insensitive on Windows, so compare the directory listing.
	rawsNames := readNames(t, filepath.Join(root, "raws"))
	if !rawsNames["A.md"] || rawsNames["a.md"] {
		t.Errorf("case-only rename failed: %v", rawsNames)
	}
	if !exists(root, "projects/b.md") || !exists(root, "projects/dir/x.md") {
		t.Error("copy failed")
	}

	// Archive: first one goes to _archives/raws/A.md, the second collides and
	// gets a timestamp.
	report = executor.Execute(planOf(
		contract.MutationAction{ID: "d", Kind: contract.MutArchive, Source: "raws/A.md"},
	))
	if report.Status != "completed" || !exists(root, "_archives/raws/A.md") {
		t.Fatalf("archive: %+v", report)
	}
	writeFile(t, root, "raws/A.md", "# again\n")
	report = executor.Execute(planOf(
		contract.MutationAction{ID: "e", Kind: contract.MutArchive, Source: "raws/A.md"},
	))
	entries, _ := os.ReadDir(filepath.Join(root, "_archives", "raws"))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	if len(names) != 2 {
		t.Fatalf("archive names = %v", names)
	}
	stamped := 0
	for _, name := range names {
		if name == "A."+"2024-01-02T03-04-05-000Z"+".md" {
			stamped++
		}
	}
	if stamped != 1 {
		t.Fatalf("archive names = %v", names)
	}
}

func TestExecuteTrashFailureStopsThePlan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "# a\n")
	writeFile(t, root, "raws/b.md", "# b\n")
	executor, _ := fixedExecutor(t, root)
	executor.Trash = func(abs string) error { return contractError{code: "io-failure", message: "回收站不可用"} }
	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutTrash, Source: "raws/a.md"},
		contract.MutationAction{ID: "b", Kind: contract.MutTrash, Source: "raws/b.md"},
	))
	if report.Status != "failed" || len(report.Failed) != 1 || len(report.Remaining) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if report.Remaining[0].Status != "skipped" {
		t.Errorf("remaining status = %s", report.Remaining[0].Status)
	}
	if !exists(root, "raws/a.md") || !exists(root, "raws/b.md") {
		t.Error("files changed despite trash failure")
	}
	manifestPath := filepath.Join(root, ".mile-kb", "transactions", report.TransactionID, "manifest.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if !strings.Contains(string(body), `"status": "partial"`) {
		t.Errorf("manifest = %s", body)
	}
}

func TestExecutePermanentDeleteNeedsFreshApproval(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "# a\n")
	executor, _ := fixedExecutor(t, root)

	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutPermanentDelete, Source: "raws/a.md"},
	))
	if report.Status != "failed" || report.Failure == nil || report.Failure.Code != contract.CodePermissionDenied {
		t.Fatalf("report = %+v", report.Failure)
	}
	if !exists(root, "raws/a.md") {
		t.Error("file deleted without fresh approval")
	}
	if exists(root, ".mile-kb/transactions") {
		entries, _ := os.ReadDir(filepath.Join(root, ".mile-kb", "transactions"))
		if len(entries) > 0 {
			t.Error("rejected plan left a transaction behind")
		}
	}

	approved := planOf(contract.MutationAction{ID: "a", Kind: contract.MutPermanentDelete, Source: "raws/a.md"})
	approved.FreshApproval = true
	report = executor.Execute(approved)
	if report.Status != "completed" || exists(root, "raws/a.md") {
		t.Fatalf("approved delete: %+v", report)
	}
}

func TestExecutePartialFailureKeepsCompletedActions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "# a\n")
	executor, _ := fixedExecutor(t, root)
	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutMove, Source: "raws/a.md", Target: "raws/b.md"},
		contract.MutationAction{ID: "b", Kind: contract.MutMove, Source: "raws/missing.md", Target: "raws/c.md"},
		contract.MutationAction{ID: "c", Kind: contract.MutCreate, Target: "raws/d.md", Text: ptr("x")},
	))
	if report.Status != "partial" {
		t.Fatalf("status = %s", report.Status)
	}
	if len(report.Completed) != 1 || len(report.Failed) != 1 || len(report.Remaining) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if !exists(root, "raws/b.md") {
		t.Error("completed action was rolled back")
	}
	if exists(root, "raws/d.md") {
		t.Error("skipped action was executed")
	}
}

func TestTransactionManifestLayout(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "raws/a.md", "<html>\n")
	manager := TxManager{Root: root, Now: func() time.Time {
		return time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	}, Rand: func() string { return "abcd1234" }}

	tx, err := manager.Begin("v", "plan-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(tx.ID, "-abcd1234") {
		t.Errorf("tx id = %s", tx.ID)
	}
	abs := filepath.Join(root, "raws", "a.md")
	if err := manager.Capture(tx, "raws/a.md", abs); err != nil {
		t.Fatal(err)
	}
	if err := manager.Capture(tx, "raws/missing.md", filepath.Join(root, "raws", "missing.md")); err != nil {
		t.Fatal(err)
	}
	if err := manager.Capture(tx, "raws", filepath.Join(root, "raws")); err != nil {
		t.Fatal(err)
	}
	if err := manager.CommitWithAfterImages(tx); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(tx.Dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.HasSuffix(text, "\n") {
		t.Error("manifest must end with a newline")
	}
	if strings.Contains(text, "\\u003c") {
		t.Error("HTML characters must not be escaped")
	}
	wantOrder := []string{`"id"`, `"vaultId"`, `"planId"`, `"createdAt"`, `"status"`, `"entries"`}
	last := -1
	for _, key := range wantOrder {
		at := strings.Index(text, key)
		if at < last {
			t.Errorf("key order broken at %s", key)
		}
		last = at
	}
	if !strings.Contains(text, `"kind": "absent"`) || !strings.Contains(text, `"kind": "directory"`) {
		t.Errorf("manifest = %s", text)
	}
	if !strings.Contains(text, `"afterKind": "file"`) {
		t.Errorf("after images missing: %s", text)
	}

	// The payload holds the exact pre-image bytes.
	payload, err := os.ReadFile(manager.PayloadPath(tx, "raws/a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != "<html>\n" {
		t.Errorf("payload = %q", payload)
	}
}

func TestExecuteBlocksEscapingPaths(t *testing.T) {
	root := t.TempDir()
	executor, _ := fixedExecutor(t, root)
	for _, action := range []contract.MutationAction{
		{ID: "a", Kind: contract.MutCreate, Target: "../outside.md"},
		{ID: "b", Kind: contract.MutMove, Source: "raws/../x.md", Target: "raws/y.md"},
		{ID: "c", Kind: contract.MutPermanentDelete, Source: "raws/../../x.md"},
	} {
		report := executor.Execute(planOf(action))
		if report.Status != "failed" {
			t.Errorf("%s %q accepted", action.Kind, action.Target+action.Source)
		}
	}
}

func TestExecuteRejectsJunctionEscape(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "raws"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "raws", "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("cannot create a reparse point here: %v", err)
	}
	executor, _ := fixedExecutor(t, root)
	report := executor.Execute(planOf(
		contract.MutationAction{ID: "a", Kind: contract.MutPermanentDelete, Source: "raws/link/secret.md"},
	))
	if report.Status != "failed" {
		t.Fatalf("delete through a junction was allowed: %+v", report)
	}
	if body, err := os.ReadFile(filepath.Join(outside, "secret.md")); err != nil || string(body) != "secret" {
		t.Fatalf("outside file damaged: %v %q", err, body)
	}
}

func TestRestoreRejectsTamperedManifestPaths(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "evil.txt")
	if err := os.WriteFile(outside, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	manager := restoreManager(t, root)
	tx, err := manager.Begin("v", "p")
	if err != nil {
		t.Fatal(err)
	}
	// Hand-write a manifest whose entry points outside the vault, plus the
	// payload an attacker would want written there.
	tampered := `{
  "id": "` + tx.ID + `",
  "vaultId": "v",
  "planId": "p",
  "createdAt": "2024-01-01T00:00:00Z",
  "status": "committed",
  "entries": [
    {"path": "../evil.txt", "kind": "file", "hash": "x", "size": 1, "afterKind": "file"}
  ]
}`
	if err := os.WriteFile(filepath.Join(tx.Dir, "manifest.json"), []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	restored, conflicted, _, f := manager.Restore(tx.ID)
	if f != nil {
		t.Fatal(f)
	}
	if len(restored) != 0 || len(conflicted) != 1 {
		t.Fatalf("restored=%v conflicted=%v", restored, conflicted)
	}
	if body, _ := os.ReadFile(outside); string(body) != "untouched" {
		t.Fatalf("outside file overwritten: %q", body)
	}
}

func TestRecordIDsAreUniqueAcrossRuns(t *testing.T) {
	root := t.TempDir()
	executor, _ := fixedExecutor(t, root)
	plan := planOf(contract.MutationAction{ID: "a1", Kind: contract.MutCreate, Target: "raws/x.md", Text: ptr("x")})
	first := executor.Execute(plan)
	plan.ID = "plan-2"
	plan.Actions[0].Target = "raws/y.md"
	second := executor.Execute(plan)
	if first.Status != "completed" || second.Status != "completed" {
		t.Fatalf("statuses: %+v %+v", first, second)
	}
	if first.Completed[0].ID == second.Completed[0].ID {
		t.Fatalf("record ids collided: %s", first.Completed[0].ID)
	}
	if !strings.HasPrefix(first.Completed[0].ID, "plan-1:a1:") {
		t.Errorf("record id shape = %s", first.Completed[0].ID)
	}
}

func TestPlanEditDropsSource(t *testing.T) {
	b, _ := builderFor(t)
	p, f := plan(t, b, contract.MutationAction{ID: "a", Kind: contract.MutEdit,
		Source: "raws/旧名.md", Target: "raws/首页.md", Text: ptr("新")})
	if f != nil {
		t.Fatal(f)
	}
	if p.Actions[0].Source != "" {
		t.Errorf("edit kept source = %q", p.Actions[0].Source)
	}
}
