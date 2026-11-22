package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func manifestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".mile-kb"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeManifestFile(t *testing.T, root, body string) string {
	t.Helper()
	path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func kbManifestItem() ManifestItem {
	return ManifestItem{
		Source:    "kb",
		Target:    ManifestTarget{Kind: "builtin", ToolID: "claude-code"},
		Name:      "vue",
		SkillPath: "skills/前端/vue",
		Mode:      "link",
		UpdatedAt: "2026-11-22T09:00:00Z",
	}
}

func githubManifestItem() ManifestItem {
	return ManifestItem{
		Source:     "github",
		Target:     ManifestTarget{Kind: "project", Name: "Mile KB Go", Subdir: ".claude/skills"},
		Name:       "pdf",
		Owner:      "anthropics",
		Repo:       "skills",
		Ref:        "main",
		Path:       "skills/pdf",
		Commit:     strings.Repeat("a", 40),
		PathCommit: strings.Repeat("b", 40),
		UpdatedAt:  "2026-11-22T09:05:00Z",
	}
}

func TestManifestReadStates(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		got := LoadManifest(t.TempDir())
		if got.Status != ManifestMissing {
			t.Fatalf("status = %q, want %q", got.Status, ManifestMissing)
		}
		if len(got.Items) != 0 {
			t.Fatalf("items = %d, want 0", len(got.Items))
		}
	})

	t.Run("ok", func(t *testing.T) {
		root := manifestRoot(t)
		// The writer produces the canonical file; reading it back must be ok.
		manifest := Manifest{Items: []ManifestItem{kbManifestItem(), githubManifestItem()}}
		if f := SaveManifest(root, &manifest); f != nil {
			t.Fatalf("save: %v", f.Message)
		}
		got := LoadManifest(root)
		if got.Status != ManifestOK {
			t.Fatalf("status = %q, want %q (warnings %v)", got.Status, ManifestOK, got.Warnings)
		}
		if len(got.Items) != 2 {
			t.Fatalf("items = %d, want 2", len(got.Items))
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		root := manifestRoot(t)
		writeManifestFile(t, root, "<<<<<<< HEAD\n{\"version\":1}\n=======\n{\"version\":1}\n>>>>>>> branch\n")
		got := LoadManifest(root)
		if got.Status != ManifestUnreadable {
			t.Fatalf("status = %q, want %q", got.Status, ManifestUnreadable)
		}
		if len(got.Warnings) == 0 {
			t.Fatal("want a warning")
		}
	})

	t.Run("too-large", func(t *testing.T) {
		root := manifestRoot(t)
		path := filepath.Join(root, ".mile-kb", "skill-distributions.json")
		big := strings.Repeat(" ", manifestMaxBytes+1)
		if err := os.WriteFile(path, []byte(big), 0o644); err != nil {
			t.Fatal(err)
		}
		got := LoadManifest(root)
		if got.Status != ManifestTooLarge {
			t.Fatalf("status = %q, want %q", got.Status, ManifestTooLarge)
		}
	})

	t.Run("missing items array", func(t *testing.T) {
		root := manifestRoot(t)
		writeManifestFile(t, root, `{"version":1,"updatedAt":"2026-11-23T09:00:00Z"}`)
		if got := LoadManifest(root); got.Status != ManifestUnreadable {
			t.Fatalf("missing items = %q, want %q", got.Status, ManifestUnreadable)
		}
	})

	t.Run("null items array", func(t *testing.T) {
		root := manifestRoot(t)
		writeManifestFile(t, root, `{"version":1,"items":null}`)
		if got := LoadManifest(root); got.Status != ManifestUnreadable {
			t.Fatalf("null items = %q, want %q", got.Status, ManifestUnreadable)
		}
	})

	t.Run("version-unknown", func(t *testing.T) {
		root := manifestRoot(t)
		writeManifestFile(t, root, `{"version":2,"items":[]}`)
		got := LoadManifest(root)
		if got.Status != ManifestVersionUnknown {
			t.Fatalf("status = %q, want %q", got.Status, ManifestVersionUnknown)
		}
	})
}

func TestManifestRefreshesUpdatedAtOnSave(t *testing.T) {
	root := manifestRoot(t)
	first := Manifest{Items: []ManifestItem{kbManifestItem()}}
	if f := SaveManifest(root, &first); f != nil {
		t.Fatalf("save: %v", f.Message)
	}
	firstStamp := first.UpdatedAt
	if firstStamp == "" {
		t.Fatal("first save did not stamp updatedAt")
	}
	time.Sleep(1100 * time.Millisecond)
	again := LoadManifest(root)
	if again.Status != ManifestOK {
		t.Fatalf("status = %q", again.Status)
	}
	item := githubManifestItem()
	again.Upsert(item)
	if f := SaveManifest(root, &again); f != nil {
		t.Fatalf("save 2: %v", f.Message)
	}
	if again.UpdatedAt == firstStamp {
		t.Fatal("updatedAt was not refreshed on the second save")
	}
	if _, err := time.Parse(time.RFC3339, again.UpdatedAt); err != nil {
		t.Fatalf("updatedAt not RFC3339: %q", again.UpdatedAt)
	}
}

func TestManifestDropsInvalidItems(t *testing.T) {
	cases := map[string]func(item *ManifestItem){
		"skillPath outside skills": func(item *ManifestItem) { item.SkillPath = ".mile-kb/settings.json" },
		"skillPath traversal":      func(item *ManifestItem) { item.SkillPath = "skills/../etc" },
		"mode wrong":               func(item *ManifestItem) { item.Mode = "move" },
		"name dot start":           func(item *ManifestItem) { item.Name = ".hidden" },
		"name with slash":          func(item *ManifestItem) { item.Name = "a/b" },
		"builtin tool unknown":     func(item *ManifestItem) { item.Target = ManifestTarget{Kind: "builtin", ToolID: "nope"} },
		"target kind unknown":      func(item *ManifestItem) { item.Target = ManifestTarget{Kind: "agent", Name: "x"} },
		"project subdir unknown": func(item *ManifestItem) {
			item.Target = ManifestTarget{Kind: "project", Name: "p", Subdir: ".nope/skills"}
		},
		"custom missing name":   func(item *ManifestItem) { item.Target = ManifestTarget{Kind: "custom"} },
		"updatedAt not RFC3339": func(item *ManifestItem) { item.UpdatedAt = "yesterday" },
	}
	for label, mutate := range cases {
		t.Run(label, func(t *testing.T) {
			root := manifestRoot(t)
			bad := kbManifestItem()
			bad.Name = "good"
			mutate(&bad)
			file := map[string]any{"version": 1, "updatedAt": "2026-11-22T09:00:00Z", "items": []ManifestItem{kbManifestItem(), bad}}
			body, err := json.Marshal(file)
			if err != nil {
				t.Fatal(err)
			}
			writeManifestFile(t, root, string(body))
			got := LoadManifest(root)
			if got.Status != ManifestOK {
				t.Fatalf("status = %q, want %q", got.Status, ManifestOK)
			}
			if len(got.Items) != 1 || got.Items[0].Name != "vue" {
				t.Fatalf("items = %+v, want only the valid one", got.Items)
			}
			if len(got.Warnings) == 0 {
				t.Fatal("want a drop warning")
			}
		})
	}
}

func TestManifestGitHubItemValidation(t *testing.T) {
	cases := map[string]func(item *ManifestItem){
		"owner invalid":  func(item *ManifestItem) { item.Owner = "bad owner" },
		"commit short":   func(item *ManifestItem) { item.Commit = "abc" },
		"pathCommit":     func(item *ManifestItem) { item.PathCommit = strings.Repeat("z", 40) },
		"skillPath kept": func(item *ManifestItem) { item.SkillPath = "skills/x/y" },
		"mode kept":      func(item *ManifestItem) { item.Mode = "link" },
	}
	for label, mutate := range cases {
		t.Run(label, func(t *testing.T) {
			item := githubManifestItem()
			mutate(&item)
			if validManifestItem(item) {
				t.Fatalf("item accepted: %+v", item)
			}
		})
	}
	if !validManifestItem(githubManifestItem()) {
		t.Fatal("valid github item rejected")
	}
	if !validManifestItem(kbManifestItem()) {
		t.Fatal("valid kb item rejected")
	}
}

func TestManifestUpsertAndRemove(t *testing.T) {
	manifest := Manifest{}
	item := kbManifestItem()
	if !manifest.Upsert(item) {
		t.Fatal("first upsert should change")
	}
	if manifest.Upsert(item) {
		t.Fatal("identical upsert should not change")
	}
	renamed := item
	renamed.Target.ToolID = "Claude-Code" // registry ids are lower case; same key comparison
	if !manifest.Upsert(renamed) {
		t.Fatal("case-variant key should replace the entry")
	}
	if len(manifest.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(manifest.Items))
	}

	manifest.Upsert(githubManifestItem())
	if len(manifest.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(manifest.Items))
	}
	if !manifest.Remove(githubManifestItem().Target, "PDF") {
		t.Fatal("case-insensitive remove should hit")
	}
	if len(manifest.Items) != 1 {
		t.Fatalf("items = %d after remove, want 1", len(manifest.Items))
	}
	if manifest.Remove(githubManifestItem().Target, "pdf") {
		t.Fatal("second remove should report no change")
	}
}

func TestManifestSerialization(t *testing.T) {
	root := manifestRoot(t)
	manifest := Manifest{Items: []ManifestItem{githubManifestItem(), kbManifestItem()}}
	if f := SaveManifest(root, &manifest); f != nil {
		t.Fatalf("save: %v", f.Message)
	}
	body, err := os.ReadFile(filepath.Join(root, ".mile-kb", "skill-distributions.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, forbidden := range []string{root, `\`, "\\", os.Getenv("USERNAME"), "t-"} {
		if forbidden == "" {
			continue
		}
		if strings.Contains(text, forbidden) {
			t.Fatalf("manifest leaks %q:\n%s", forbidden, text)
		}
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatal("manifest should end with a newline")
	}
	if !strings.Contains(text, "\n  \"version\": 1,") {
		t.Fatalf("want two-space indent, got:\n%s", text)
	}
	// Sorted: builtin before project.
	firstKb := strings.Index(text, `"toolId": "claude-code"`)
	firstProject := strings.Index(text, `"subdir": ".claude/skills"`)
	if firstKb < 0 || firstProject < 0 || firstKb > firstProject {
		t.Fatalf("items not sorted by target kind:\n%s", text)
	}

	// Round trip keeps the same order.
	got := LoadManifest(root)
	if len(got.Items) != 2 || got.Items[0].Name != "vue" || got.Items[1].Name != "pdf" {
		t.Fatalf("round trip order = %+v", got.Items)
	}
}

func TestManifestItemCap(t *testing.T) {
	root := manifestRoot(t)
	items := make([]ManifestItem, 0, manifestMaxItems+5)
	for i := 0; i < manifestMaxItems+5; i++ {
		item := kbManifestItem()
		item.Name = fmt.Sprintf("skill-%05d", i)
		items = append(items, item)
	}
	file := map[string]any{"version": 1, "updatedAt": "2026-11-22T09:00:00Z", "items": items}
	body, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	writeManifestFile(t, root, string(body))
	got := LoadManifest(root)
	if got.Status != ManifestOK {
		t.Fatalf("status = %q", got.Status)
	}
	if len(got.Items) != manifestMaxItems {
		t.Fatalf("items = %d, want capped %d", len(got.Items), manifestMaxItems)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("want a cap warning")
	}
}

func TestManifestDedupKeepsLast(t *testing.T) {
	root := manifestRoot(t)
	first := kbManifestItem()
	second := kbManifestItem()
	second.Mode = "copy"
	file := map[string]any{"version": 1, "updatedAt": "2026-11-22T09:00:00Z", "items": []ManifestItem{first, second}}
	body, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	writeManifestFile(t, root, string(body))
	got := LoadManifest(root)
	if len(got.Items) != 1 || got.Items[0].Mode != "copy" {
		t.Fatalf("items = %+v, want the last duplicate", got.Items)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("want a duplicate warning")
	}
}

func TestManifestInvalidWriteIsRefused(t *testing.T) {
	root := manifestRoot(t)
	bad := kbManifestItem()
	bad.Name = "../escape"
	manifest := Manifest{Items: []ManifestItem{bad}}
	if f := SaveManifest(root, &manifest); f == nil {
		t.Fatal("save should refuse an invalid item")
	}
	if _, err := os.Stat(filepath.Join(root, ".mile-kb", "skill-distributions.json")); !os.IsNotExist(err) {
		t.Fatal("nothing should be written")
	}
}
