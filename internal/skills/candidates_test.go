package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCandidate(t *testing.T, root, rel, body string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindCandidatesNested(t *testing.T) {
	root := t.TempDir()
	writeCandidate(t, root, "skills/pdf", "---\nname: pdf-tools\ndescription: PDF\n---\n")
	writeCandidate(t, root, "skills/group/docx", "# docx\n")
	writeCandidate(t, root, "plugins/a/b/deep", "# deep\n")
	writeCandidate(t, root, "plugins/a/b/c/too-deep", "# too deep\n")
	writeCandidate(t, root, ".hidden/skip", "# skip\n")
	writeCandidate(t, root, "node_modules/skip", "# skip\n")

	result, f := FindCandidates(root, "repo", CandidateOptions{})
	if f != nil {
		t.Fatalf("candidates: %+v", f)
	}
	names := map[string]string{}
	for _, candidate := range result.Candidates {
		names[candidate.Name] = candidate.State
	}
	if names["pdf"] != "ok" || names["docx"] != "ok" {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	for _, absent := range []string{"deep", "too-deep", "skip"} {
		if _, ok := names[absent]; ok {
			t.Fatalf("%s should not be a candidate: %+v", absent, result.Candidates)
		}
	}
}

func TestFindCandidatesRootSkill(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\nname: pdf-kit\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeCandidate(t, root, "skills/nested", "# nested\n")
	result, f := FindCandidates(root, "acme-skills", CandidateOptions{})
	if f != nil {
		t.Fatalf("candidates: %+v", f)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Name != "pdf-kit" || result.Candidates[0].ID != "" {
		t.Fatalf("root candidate = %+v", result.Candidates)
	}
}

func TestFindCandidatesStates(t *testing.T) {
	root := t.TempDir()
	writeCandidate(t, root, "a/dup", "# one\n")
	writeCandidate(t, root, "b/dup", "# two\n")
	writeCandidate(t, root, strings.Repeat("a", 81), "# bad\n")
	writeCandidate(t, root, "big", "# big\n")
	for i := 0; i < 3; i++ {
		path := filepath.Join(root, "big", "f"+string(rune('a'+i)))
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeCandidate(t, root, "known", "# known\n")

	result, f := FindCandidates(root, "repo", CandidateOptions{
		MaxFiles:      2,
		MaxCandidates: 50,
		Existing: func(name string) (string, bool) {
			if name == "known" {
				return "<vault>/skills/通用/known", true
			}
			return "", false
		},
	})
	if f != nil {
		t.Fatalf("candidates: %+v", f)
	}
	states := map[string]string{}
	for _, candidate := range result.Candidates {
		key := candidate.ID
		if key == "" {
			key = candidate.Name
		}
		states[key] = candidate.State
	}
	if states["a/dup"] != "ok" || states["b/dup"] != "duplicate" {
		t.Fatalf("duplicate states = %+v", states)
	}
	if states[strings.Repeat("a", 81)] != "invalid-name" {
		t.Fatalf("invalid name state = %+v", states)
	}
	if states["big"] != "too-large" {
		t.Fatalf("too-large state = %+v", states)
	}
	if states["known"] != "exists" {
		t.Fatalf("exists state = %+v", states)
	}
}

func TestFindCandidatesTruncates(t *testing.T) {
	root := t.TempDir()
	writeCandidate(t, root, "one", "# one\n")
	writeCandidate(t, root, "two", "# two\n")
	result, f := FindCandidates(root, "repo", CandidateOptions{MaxCandidates: 1})
	if f != nil {
		t.Fatalf("candidates: %+v", f)
	}
	if !result.Truncated || len(result.Candidates) != 1 {
		t.Fatalf("truncate = %+v", result)
	}
}
