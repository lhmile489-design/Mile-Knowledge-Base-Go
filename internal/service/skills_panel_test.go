package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M4 T04: M76/M77 wire-up, redaction and cached-first scanning (API.md §12).

func skillServiceWithHome(t *testing.T, home string) (*SkillService, string) {
	t.Helper()
	app, vaultID, _ := mutationTestApp(t)
	svc := NewSkillService(app)
	svc.detector = func() *skills.Detector {
		return &skills.Detector{
			Home:             home,
			LocalAppData:     filepath.Join(home, "AppData", "Local"),
			UserProgramFiles: filepath.Join(home, "Programs"),
			CLIDirs:          []string{filepath.Join(home, ".local", "bin")},
		}
	}
	return svc, vaultID
}

func TestSkillServiceRegistryRedactsPaths(t *testing.T) {
	home := t.TempDir()
	svc, _ := skillServiceWithHome(t, home)
	result := svc.Registry(context.Background())
	if len(result.Tools) != 25 {
		t.Fatalf("tools = %d, want 25", len(result.Tools))
	}
	var claude contract.SkillTool
	for _, tool := range result.Tools {
		if tool.ID == "claude-code" {
			claude = tool
		}
	}
	if !strings.HasPrefix(claude.InstallDir, "<home>") || !strings.HasSuffix(claude.InstallDir, "skills") {
		t.Fatalf("installDir = %q, want redacted", claude.InstallDir)
	}
}

func TestSkillServiceScanIsCachedFirst(t *testing.T) {
	home := t.TempDir()
	writeSkill := func(rel string) {
		t.Helper()
		full := filepath.Join(home, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("# vue\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSkill(".local/bin/claude.cmd")
	writeSkill(".claude/skills/vue/SKILL.md")
	svc, vaultID := skillServiceWithHome(t, home)

	first := svc.Scan(context.Background(), vaultID)
	if first.Failure != nil {
		t.Fatalf("scan: %+v", first.Failure)
	}
	if first.Scan.Scanning {
		t.Fatal("first scan should be synchronous")
	}
	var found bool
	for _, group := range first.Scan.Tools {
		if group.ToolID != "claude-code" {
			continue
		}
		for _, skill := range group.Skills {
			if skill.Name == "vue" && skill.Origin == "independent" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("claude-code vu not found: %+v", first.Scan.Tools)
	}

	second := svc.Scan(context.Background(), vaultID)
	if second.Failure != nil {
		t.Fatalf("second scan: %+v", second.Failure)
	}
	if !second.Scan.Scanning {
		t.Fatal("second scan should serve the cache with scanning=true")
	}
	if second.Scan.CachedAt == "" {
		t.Fatal("cached scan has no timestamp")
	}
}

func TestSkillServiceScanRejectsBadVault(t *testing.T) {
	svc, _ := skillServiceWithHome(t, t.TempDir())
	result := svc.Scan(context.Background(), "bad id with spaces")
	if result.Failure == nil || result.Failure.Code != contract.CodeInvalidInput {
		t.Fatalf("failure = %+v", result.Failure)
	}
	if result.Failure.Message == "" {
		t.Fatal("failure message is empty")
	}
}
