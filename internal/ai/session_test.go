package ai

import (
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestRegistryLazyAndIdempotent(t *testing.T) {
	registry := NewRegistry()
	first := registry.Get("scout", "v1")
	second := registry.Get("scout", "v1")
	if first != second {
		t.Fatal("same (agent, vault) must return the same session")
	}
	if first.SessionID == "" || first.Approval != "ask" || first.ContextWindow != DefaultContextWindow {
		t.Fatalf("session defaults: %+v", first)
	}
	if registry.Find("scout", "v2") != nil {
		t.Fatal("Find must not create")
	}
	registry.Get("scout", "v2")
	registry.Reset("v1")
	if registry.Find("scout", "v1") != nil {
		t.Fatal("Reset must drop the sessions")
	}
	if registry.Find("scout", "v2") == nil {
		t.Fatal("Reset must only touch one vault")
	}
}

func provider(name, kind, base string, models []string, def string) Provider {
	return Provider{Name: name, Kind: kind, BaseURL: base, Models: models, DefaultModel: def}
}

func TestResolveProviderOrder(t *testing.T) {
	providers := []Provider{
		provider("anthropic", "anthropic", "https://a.test", []string{"claude"}, "claude"),
		provider("first", "openai", "https://f.test", []string{"m1", "m2"}, "m2"),
		provider("second", "openai", "https://s.test", []string{"m3"}, "m3"),
	}

	chosen, model, err := ResolveProvider(providers, "m3", "m1")
	if err != nil || chosen.Name != "second" || model != "m3" {
		t.Fatalf("session choice must win: %+v %s %+v", chosen, model, err)
	}
	chosen, model, _ = ResolveProvider(providers, "", "m1")
	if chosen.Name != "first" || model != "m1" {
		t.Fatalf("saved model: %+v %s", chosen, model)
	}
	chosen, model, _ = ResolveProvider(providers, "unknown", "unknown")
	if chosen.Name != "first" || model != "m2" {
		t.Fatalf("fallback to the first usable default: %+v %s", chosen, model)
	}
	chosen, model, _ = ResolveProvider([]Provider{provider("p", "openai", "https://p.test", []string{"only"}, "missing")}, "", "")
	if chosen.Name != "p" || model != "only" {
		t.Fatalf("default not in list falls back to models[0]: %+v %s", chosen, model)
	}
}

func TestResolveProviderUnavailable(t *testing.T) {
	_, _, err := ResolveProvider(nil, "", "")
	if err == nil || err.Code != contract.CodeEngineUnavailable || !containsSubstring(err.Message, "还没有配置") {
		t.Fatalf("no providers: %+v", err)
	}
	_, _, err = ResolveProvider([]Provider{provider("a", "anthropic", "https://a.test", []string{"x"}, "x")}, "", "")
	if err == nil || !containsSubstring(err.Message, "OpenAI") {
		t.Fatalf("no openai provider: %+v", err)
	}
}

func TestModelChoices(t *testing.T) {
	got := ModelChoices([]Provider{
		provider("a", "openai", "https://a.test", []string{"m1", "m2"}, "m1"),
		provider("b", "openai", "https://b.test", []string{"m2", "m3"}, "m2"),
		provider("c", "anthropic", "https://c.test", []string{"claude"}, "claude"),
	})
	if len(got) != 3 || got[0] != "m1" || got[2] != "m3" {
		t.Fatalf("choices: %v", got)
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || (len(needle) > 0 && indexOf(haystack, needle) >= 0))
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
