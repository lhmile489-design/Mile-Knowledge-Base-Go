package policy

import "testing"

func TestNormalize(t *testing.T) {
	good := map[string]string{
		"raws/a.md":  "raws/a.md",
		`raws\a.md`:  "raws/a.md",
		"raws//a.md": "raws/a.md",
		"a b/c.md":   "a b/c.md",
	}
	for input, want := range good {
		if got, ok := Normalize(input); !ok || got != want {
			t.Errorf("%q: %q %v", input, got, ok)
		}
	}
	bad := []string{"", "/abs.md", `\abs.md`, `\\server\share`, "C:/x.md", "c:x.md", "../up.md", "a/../b.md", "./a.md", "a/./b.md"}
	for _, input := range bad {
		if _, ok := Normalize(input); ok {
			t.Errorf("%q should be refused", input)
		}
	}
}

func TestEveryAgentCannotWriteSystemPaths(t *testing.T) {
	paths := []string{
		".mile-kb/config.json",
		".mile-kb/transactions/t1/manifest.json",
		".mile-kb/index/x.db",
		".mile-kb/provenance.json",
		".mile-kb/settings/agents.json",
		".mile-kb/drafts/x.md",
		".git/config",
	}
	for _, agent := range AgentIDs {
		for _, path := range paths {
			decision := CanWrite(agent, path)
			if decision.Allowed {
				t.Errorf("%s wrote %s", agent, path)
			}
			if decision.Code != "system-path-protected" && decision.Code != "area-not-writable" {
				t.Errorf("%s %s code=%s", agent, path, decision.Code)
			}
		}
	}
}

func TestWriteContentAreas(t *testing.T) {
	for _, path := range []string{"raws/a.md", "skills/x/SKILL.md", "projects/p.md", "other/x.md", "_archives/a.md", "tools/t.md"} {
		for _, agent := range AgentIDs {
			if decision := CanWrite(agent, path); !decision.Allowed {
				t.Errorf("%s should write %s: %+v", agent, path, decision)
			}
		}
	}
}

func TestDotSegmentsCannotBypassPrefixes(t *testing.T) {
	// `.mile-kb/drafts/../config.json` starts with the drafts prefix; the dot
	// segment must be refused before any prefix logic runs.
	if decision := CanWrite("curator", ".mile-kb/drafts/../config.json"); decision.Allowed {
		t.Fatalf("dot segment bypassed the deny list: %+v", decision)
	}
}

func TestUnknownAgent(t *testing.T) {
	if decision := CanWrite("nobody", "raws/a.md"); decision.Allowed || decision.Code != "invalid-path" {
		t.Fatalf("unknown agent: %+v", decision)
	}
	if decision := CanMutate("nobody", KindMove, "raws/a.md", "raws"); decision.Allowed {
		t.Fatalf("unknown agent mutate: %+v", decision)
	}
}

func TestCanMutate(t *testing.T) {
	if decision := CanMutate("curator", KindMove, "raws/a.md", "raws/b.md"); !decision.Allowed {
		t.Fatalf("move: %+v", decision)
	}
	if decision := CanMutate("curator", KindMove, "raws/a.md", ".mile-kb/settings/x.json"); decision.Allowed {
		t.Fatalf("move into settings: %+v", decision)
	}
	if decision := CanMutate("curator", KindMove, ".mile-kb/config.json", "raws/x"); decision.Allowed {
		t.Fatalf("move out of settings: %+v", decision)
	}
	if decision := CanMutate("curator", KindCreate, "", "raws/new.md"); !decision.Allowed {
		t.Fatalf("create at root target: %+v", decision)
	}
	if decision := CanMutate("curator", KindCreate, "raws/new.md", ""); !decision.Allowed {
		t.Fatalf("create via source: %+v", decision)
	}
	if decision := CanMutate("curator", "rename", "raws/a.md", "raws/b.md"); decision.Allowed {
		t.Fatalf("unsupported kind: %+v", decision)
	}
	deleteDecision := CanMutate("curator", KindPermanentDelete, "raws/a.md", "")
	if !deleteDecision.Allowed || !deleteDecision.RequiresFreshApproval {
		t.Fatalf("permanent delete must require fresh approval: %+v", deleteDecision)
	}
	if decision := CanMutate("curator", KindTrash, "raws/a.md", ""); !decision.Allowed || decision.RequiresFreshApproval {
		t.Fatalf("trash: %+v", decision)
	}
}

func TestGlobalPrefixesAreLowercaseMatched(t *testing.T) {
	if decision := CanWrite("scout", ".MILE-KB/Settings/x.json"); decision.Allowed {
		t.Fatalf("case variant bypassed: %+v", decision)
	}
}
