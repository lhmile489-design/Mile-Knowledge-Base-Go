// Package ai holds the M2 engine: providers, keys, streaming, tools and the
// shared context. This file ports the provider store (API M67/M68,
// 数据库设计 3.6).
package ai

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Limits come from the legacy validators (appConfig.ts:135-139,
// ProviderStore.ts:36, validate.ts:618).
const (
	MaxProvidersRead     = 50
	MaxProvidersSave     = 32
	MaxProviderNameLen   = 64
	MaxProviderURLLen    = 512
	MaxEnvNameLen        = 96
	MaxProviderModels    = 32
	MaxModelNameLen      = 128
	providersVersion     = 1
	legacyConfigFileName = "reasonix.toml"
	legacyBeginMarker    = "# >>> mile-knowledge-base managed providers"
	legacyEndMarker      = "# <<< mile-knowledge-base managed providers"
)

var (
	providerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	envNamePattern      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	legacyKeyValue      = regexp.MustCompile(`^\s*([a-z_]+)\s*=\s*(.+?)\s*$`)
	legacyBlockSplit    = regexp.MustCompile(`(?m)^\s*\[\[providers\]\]\s*$`)
)

// Provider is one entry of providers.json. Field order matches the legacy
// writer; Extra keeps unknown keys so a newer reader's fields survive a
// round-trip (数据库设计 3.0.1).
type Provider struct {
	Name          string
	Kind          string
	BaseURL       string
	Models        []string
	DefaultModel  string
	APIKeyEnv     string
	ContextWindow *int
	Extra         map[string]json.RawMessage
}

var providerKnownFields = map[string]bool{
	"name": true, "kind": true, "baseUrl": true, "models": true,
	"defaultModel": true, "apiKeyEnv": true, "contextWindow": true,
}

// MarshalJSON writes the fixed key order first, then unknown fields sorted by
// key. The outer encoder disables HTML escaping.
func (p Provider) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	field := func(key string, value any) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		k, err := encodeValue(key)
		if err != nil {
			return err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := encodeValue(value)
		if err != nil {
			return err
		}
		buf.Write(v)
		return nil
	}
	if err := field("name", p.Name); err != nil {
		return nil, err
	}
	if err := field("kind", p.Kind); err != nil {
		return nil, err
	}
	if err := field("baseUrl", p.BaseURL); err != nil {
		return nil, err
	}
	if err := field("models", p.Models); err != nil {
		return nil, err
	}
	if err := field("defaultModel", p.DefaultModel); err != nil {
		return nil, err
	}
	if err := field("apiKeyEnv", p.APIKeyEnv); err != nil {
		return nil, err
	}
	if p.ContextWindow != nil {
		if err := field("contextWindow", *p.ContextWindow); err != nil {
			return nil, err
		}
	}
	keys := make([]string, 0, len(p.Extra))
	for key := range p.Extra {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := field(key, p.Extra[key]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func encodeValue(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// ProviderPath is where providers.json lives inside a vault.
func ProviderPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".mile-kb", "settings", "providers.json")
}

// NormalizeResult carries either a normalized provider or a failure.
type NormalizeResult struct {
	OK       bool
	Provider Provider
	Failure  *contract.Failure
}

func invalid(message string) *contract.Failure {
	return contract.Fail(contract.CodeInvalidInput, message)
}

func asMap(raw any) (map[string]any, bool) {
	switch value := raw.(type) {
	case map[string]any:
		return value, true
	case json.RawMessage:
		var out map[string]any
		if json.Unmarshal(value, &out) != nil {
			return nil, false
		}
		return out, true
	case []byte:
		var out map[string]any
		if json.Unmarshal(value, &out) != nil {
			return nil, false
		}
		return out, true
	default:
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, false
		}
		var out map[string]any
		if json.Unmarshal(b, &out) != nil {
			return nil, false
		}
		return out, true
	}
}

// NormalizeProvider trims, dedupes, fills defaults and validates one draft
// (legacy normalizeProviderDraft, appConfig.ts:208-286).
func NormalizeProvider(raw any) NormalizeResult {
	source, ok := asMap(raw)
	if !ok {
		return NormalizeResult{Failure: invalid("provider 必须是对象")}
	}

	name, _ := source["name"].(string)
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > MaxProviderNameLen || !providerNamePattern.MatchString(name) {
		return NormalizeResult{Failure: invalid("provider 名只能用字母、数字、下划线、点与短横线，且以字母或数字开头")}
	}

	kind, _ := source["kind"].(string)
	if kind != "openai" && kind != "anthropic" {
		return NormalizeResult{Failure: invalid("kind 只接受 openai / anthropic")}
	}

	baseURL, _ := source["baseUrl"].(string)
	baseURL = strings.TrimSpace(baseURL)
	if len(baseURL) == 0 || len(baseURL) > MaxProviderURLLen {
		return NormalizeResult{Failure: invalid("baseUrl 必须是 http/https 地址")}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return NormalizeResult{Failure: invalid("baseUrl 必须是 http/https 地址")}
	}

	apiKeyEnv, _ := source["apiKeyEnv"].(string)
	apiKeyEnv = strings.TrimSpace(apiKeyEnv)
	if apiKeyEnv != "" && (len(apiKeyEnv) > MaxEnvNameLen || !envNamePattern.MatchString(apiKeyEnv)) {
		return NormalizeResult{Failure: invalid("apiKeyEnv 是环境变量名（只含字母数字下划线），不是模型名也不是 key 值")}
	}

	models := make([]string, 0, 4)
	if rawList, ok := source["models"].([]any); ok {
		for _, item := range rawList {
			model, ok := item.(string)
			if !ok {
				return NormalizeResult{Failure: invalid("模型名必须是字符串")}
			}
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if len(model) > MaxModelNameLen {
				return NormalizeResult{Failure: invalid("模型名过长")}
			}
			if !contains(models, model) {
				models = append(models, model)
			}
			if len(models) > MaxProviderModels {
				return NormalizeResult{Failure: invalid("模型数量不能超过 32")}
			}
		}
	}
	if len(models) == 0 {
		return NormalizeResult{Failure: invalid("至少要有一个模型")}
	}

	requested, _ := source["defaultModel"].(string)
	requested = strings.TrimSpace(requested)
	defaultModel := models[0]
	if contains(models, requested) {
		defaultModel = requested
	}

	var contextWindow *int
	if raw, ok := source["contextWindow"].(float64); ok && raw > 0 && raw == float64(int(raw)) {
		v := int(raw)
		contextWindow = &v
	}

	extra := map[string]json.RawMessage{}
	for key, value := range source {
		if providerKnownFields[key] {
			continue
		}
		encoded, err := encodeValue(value)
		if err != nil {
			continue
		}
		extra[key] = json.RawMessage(encoded)
	}
	if len(extra) == 0 {
		extra = nil
	}

	return NormalizeResult{OK: true, Provider: Provider{
		Name: name, Kind: kind, BaseURL: baseURL, Models: models,
		DefaultModel: defaultModel, APIKeyEnv: apiKeyEnv, ContextWindow: contextWindow, Extra: extra,
	}}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// CoerceProviders reads the top-level object the way the legacy store does:
// bad JSON, a non-object or a missing list all yield an empty result; each
// entry is normalized and skipped when invalid; names are deduped
// case-sensitively on read (ProviderStore.ts:62-76); extras are cut at 50.
func CoerceProviders(raw any) []Provider {
	top, ok := raw.(map[string]any)
	if !ok {
		if asMapValue, ok2 := asMap(raw); ok2 {
			top = asMapValue
		} else {
			return nil
		}
	}
	list, ok := top["providers"].([]any)
	if !ok {
		return nil
	}
	if len(list) > MaxProvidersRead {
		list = list[:MaxProvidersRead]
	}
	out := make([]Provider, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		normalized := NormalizeProvider(item)
		if !normalized.OK {
			continue
		}
		if seen[normalized.Provider.Name] {
			continue
		}
		seen[normalized.Provider.Name] = true
		out = append(out, normalized.Provider)
	}
	return out
}

// ReadProviders reads providers.json, or migrates the legacy managed block
// from reasonix.toml when the file is absent or unreadable. An empty migration
// writes nothing so the next read retries (ProviderStore.ts:38-60).
func ReadProviders(vaultRoot string) ([]Provider, *contract.Failure) {
	raw, err := os.ReadFile(ProviderPath(vaultRoot))
	if err == nil {
		var top any
		if json.Unmarshal(raw, &top) != nil {
			return nil, nil
		}
		return CoerceProviders(top), nil
	}
	legacy, err := os.ReadFile(filepath.Join(vaultRoot, legacyConfigFileName))
	if err != nil {
		return nil, nil
	}
	migrated := ParseLegacyProviders(string(legacy))
	if len(migrated) > 0 {
		if f := WriteProviders(vaultRoot, migrated); f != nil {
			return nil, f
		}
	}
	return migrated, nil
}

// SanitizeProviders validates a save request: at most 32 entries, every entry
// normalized, duplicate names rejected case-insensitively, and one bad entry
// fails the whole save (validate.ts:625-643).
func SanitizeProviders(list []Provider) ([]Provider, *contract.Failure) {
	if len(list) > MaxProvidersSave {
		return nil, invalid("provider 数量过多")
	}
	out := make([]Provider, 0, len(list))
	names := map[string]bool{}
	for _, item := range list {
		normalized := NormalizeProvider(providerToMap(item))
		if !normalized.OK {
			return nil, normalized.Failure
		}
		key := strings.ToLower(normalized.Provider.Name)
		if names[key] {
			return nil, invalid("provider 名重复：" + normalized.Provider.Name)
		}
		names[key] = true
		out = append(out, normalized.Provider)
	}
	return out, nil
}

func providerToMap(p Provider) map[string]any {
	out := map[string]any{
		"name": p.Name, "kind": p.Kind, "baseUrl": p.BaseURL, "models": toAnyList(p.Models),
		"defaultModel": p.DefaultModel, "apiKeyEnv": p.APIKeyEnv,
	}
	if p.ContextWindow != nil {
		out["contextWindow"] = *p.ContextWindow
	}
	for key, value := range p.Extra {
		out[key] = value
	}
	return out
}

func toAnyList(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

// WriteProviders writes the whole file atomically with 2-space indentation,
// no HTML escaping and the fixed key order (数据库设计 3.6).
func WriteProviders(vaultRoot string, list []Provider) *contract.Failure {
	target := ProviderPath(vaultRoot)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建设置目录")
	}
	payload := struct {
		Version   int        `json:"version"`
		Providers []Provider `json:"providers"`
	}{Version: providersVersion, Providers: list}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法序列化供应商配置")
	}
	tmp := target + "." + randomHex(4) + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入供应商配置")
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return contract.Fail(contract.CodeIOFailure, "无法写入供应商配置")
	}
	return nil
}

// ParseLegacyProviders reads only the managed block of reasonix.toml. It is a
// deliberate regex parser, not a TOML library, to keep legacy behavior
// identical (ProviderStore.ts:92-137).
func ParseLegacyProviders(text string) []Provider {
	begin := strings.Index(text, legacyBeginMarker)
	if begin < 0 {
		return nil
	}
	end := strings.Index(text[begin+1:], legacyEndMarker)
	if end < 0 {
		return nil
	}
	end += begin + 1
	bodyStart := strings.IndexByte(text[begin:], '\n')
	if bodyStart < 0 {
		return nil
	}
	bodyStart += begin
	if bodyStart >= end {
		return nil
	}
	region := text[bodyStart+1 : end]
	chunks := legacyBlockSplit.Split(region, -1)
	drafts := make([]any, 0, len(chunks))
	for _, chunk := range chunks[1:] {
		fields := map[string]string{}
		for _, line := range strings.Split(chunk, "\n") {
			line = strings.TrimSuffix(line, "\r")
			if match := legacyKeyValue.FindStringSubmatch(line); match != nil {
				fields[match[1]] = match[2]
			}
		}
		draft := map[string]any{
			"name":         parseTomlString(fields["name"]),
			"kind":         parseTomlString(fields["kind"]),
			"baseUrl":      parseTomlString(fields["base_url"]),
			"models":       toAnyList(parseTomlStringArray(fields["models"])),
			"defaultModel": parseTomlString(fields["default"]),
			"apiKeyEnv":    parseTomlString(fields["api_key_env"]),
		}
		if window, err := strconv.Atoi(strings.TrimSpace(fields["context_window"])); err == nil && window > 0 {
			draft["contextWindow"] = float64(window)
		}
		drafts = append(drafts, draft)
	}
	return CoerceProviders(map[string]any{"version": providersVersion, "providers": drafts})
}

func parseTomlString(literal string) string {
	trimmed := strings.TrimSpace(literal)
	if len(trimmed) < 2 || !strings.HasPrefix(trimmed, `"`) || !strings.HasSuffix(trimmed, `"`) {
		return ""
	}
	return strings.ReplaceAll(strings.ReplaceAll(trimmed[1:len(trimmed)-1], `\"`, `"`), `\\`, `\`)
}

func parseTomlStringArray(literal string) []string {
	trimmed := strings.TrimSpace(literal)
	if len(trimmed) < 2 || !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return nil
	}
	inner := trimmed[1 : len(trimmed)-1]
	out := make([]string, 0, 4)
	for _, part := range strings.Split(inner, ",") {
		value := parseTomlString(part)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// DeepSeekPreset is the first-run suggestion (appConfig.ts:125-133).
func DeepSeekPreset() Provider {
	window := 1_000_000
	return Provider{
		Name: "deepseek", Kind: "openai", BaseURL: "https://api.deepseek.com",
		Models: []string{"deepseek-v4-flash", "deepseek-v4-pro"}, DefaultModel: "deepseek-v4-flash",
		APIKeyEnv: "DEEPSEEK_API_KEY", ContextWindow: &window,
	}
}

func randomHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "tmp"
	}
	return hex.EncodeToString(buf)
}

var _ = errors.ErrUnsupported
