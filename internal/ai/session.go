package ai

import (
	"context"
	"strings"
	"sync"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Session is one agent's conversation with one vault. History is the only
// message store; permissions and usage live beside it.
type Session struct {
	Agent     string
	VaultID   string
	SessionID string

	Model         string
	Effort        string
	Approval      string
	ContextWindow int
	Busy          bool

	mu          sync.Mutex
	messages    []ChatMessage
	cumulative  contract.Usage
	calibration float64
	cancel      context.CancelFunc
}

// Messages returns a copy of the history.
func (s *Session) Messages() []ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ChatMessage(nil), s.messages...)
}

// Cumulative returns the session total usage.
func (s *Session) Cumulative() contract.Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cumulative
}

// Calibration returns the current token calibration factor.
func (s *Session) Calibration() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.calibration <= 0 {
		return 1
	}
	return s.calibration
}

// Registry keeps one session per (agent, vault), created lazily and idempotently.
type Registry struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry { return &Registry{sessions: map[string]*Session{}} }

func sessionKey(agent, vaultID string) string { return agent + "\x00" + vaultID }

// Get returns the existing session or creates one. The caller sets the model
// and approval from the resolved provider and config.
func (r *Registry) Get(agent, vaultID string) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := sessionKey(agent, vaultID)
	if session, ok := r.sessions[key]; ok {
		return session
	}
	session := &Session{
		Agent: agent, VaultID: vaultID, SessionID: "sess_" + randomHex(8),
		Effort: "auto", Approval: "ask", ContextWindow: DefaultContextWindow,
	}
	r.sessions[key] = session
	return session
}

// Find returns the session without creating one.
func (r *Registry) Find(agent, vaultID string) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sessions[sessionKey(agent, vaultID)]
}

// Reset drops the four agents' sessions for a vault (M43).
func (r *Registry) Reset(vaultID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, agent := range AgentIDs {
		delete(r.sessions, sessionKey(agent, vaultID))
	}
}

// CancelAgent aborts the running turn of every session of one agent (M40).
func (r *Registry) CancelAgent(agent string) {
	r.mu.Lock()
	targets := make([]*Session, 0, 4)
	for _, session := range r.sessions {
		if session.Agent == agent {
			targets = append(targets, session)
		}
	}
	r.mu.Unlock()
	for _, session := range targets {
		session.Cancel()
	}
}

// CancelVault aborts every running turn of one vault (M43).
func (r *Registry) CancelVault(vaultID string) {
	r.mu.Lock()
	targets := make([]*Session, 0, 4)
	for _, session := range r.sessions {
		if session.VaultID == vaultID {
			targets = append(targets, session)
		}
	}
	r.mu.Unlock()
	for _, session := range targets {
		session.Cancel()
	}
}

// Cancel aborts the running turn, if any.
func (s *Session) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ResolveProvider picks the provider and model in the legacy order: the
// session's choice, then the agent's saved model, then the first usable
// provider's default, then that provider's first model. Only kind=openai is
// usable by the engine (nativeProvider.ts:24-51).
func ResolveProvider(providers []Provider, requested, saved string) (Provider, string, *EngineError) {
	var usable []Provider
	for _, provider := range providers {
		if provider.Kind == "openai" {
			usable = append(usable, provider)
		}
	}
	if len(usable) == 0 {
		reason := "还没有配置模型服务：在设置的「模型服务」里添加一个"
		if len(providers) > 0 {
			reason = "自研引擎只支持 OpenAI 兼容格式的 provider，当前配置里没有"
		}
		return Provider{}, "", &EngineError{Code: contract.CodeEngineUnavailable, Message: reason}
	}
	owner := func(name string) (Provider, bool) {
		if name == "" {
			return Provider{}, false
		}
		for _, provider := range usable {
			if contains(provider.Models, name) {
				return provider, true
			}
		}
		return Provider{}, false
	}
	for _, candidate := range []string{requested, saved} {
		if provider, ok := owner(candidate); ok {
			return provider, candidate, nil
		}
	}
	first := usable[0]
	model := first.DefaultModel
	if !contains(first.Models, model) {
		model = first.Models[0]
	}
	return first, model, nil
}

// ModelChoices lists every distinct model across usable providers, for the
// settings dropdown.
func ModelChoices(providers []Provider) []string {
	seen := map[string]bool{}
	var out []string
	for _, provider := range providers {
		if provider.Kind != "openai" {
			continue
		}
		for _, model := range provider.Models {
			if !seen[model] {
				seen[model] = true
				out = append(out, model)
			}
		}
	}
	return out
}

// SessionInfo is what M38 returns.
type SessionInfo struct {
	Agent            string
	SessionID        string
	VaultID          string
	Model            string
	Effort           string
	Approval         string
	AvailableModels  []string
	AvailableEfforts []string // always empty: the engine declares no levels (D-002)
	Busy             bool
}

// Info describes a session for the UI.
func (s *Session) Info(models []string) SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return SessionInfo{
		Agent: s.Agent, SessionID: s.SessionID, VaultID: s.VaultID,
		Model: s.Model, Effort: s.Effort, Approval: s.Approval,
		AvailableModels: models, AvailableEfforts: []string{}, Busy: s.Busy,
	}
}

// HasKey reports whether the provider's key is configured. An empty apiKeyEnv
// means the provider needs no auth.
func HasKey(keys *KeyStore, provider Provider) bool {
	if strings.TrimSpace(provider.APIKeyEnv) == "" {
		return true
	}
	return keys != nil && keys.Has(provider.APIKeyEnv)
}
