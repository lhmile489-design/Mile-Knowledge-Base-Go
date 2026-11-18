package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const (
	// MaxFetchedModels caps what the settings page shows (modelCatalog.ts:27).
	MaxFetchedModels = 500
	// MaxModelIDLen drops absurd ids from a provider response.
	MaxModelIDLen = 128
	// AnthropicVersion is the header anthropic's model list requires.
	AnthropicVersion = "2023-06-01"

	defaultFetchTimeout = 10 * time.Second
	maxModelsBody       = 2 * 1024 * 1024
	snippetLimit        = 300
)

var (
	controlOrSpace = regexp.MustCompile(`[\x00-\x1f\s]`)
	secretPattern  = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_-]{8,}|ghp_[A-Za-z0-9]{8,}|xox[bp]-[A-Za-z0-9-]{8,})`)
	userPath       = regexp.MustCompile(`(?i)([A-Za-z]:\\Users\\[^\\\s]+|/home/[^/\s]+)`)
)

// ModelRequest is the pure result of building a model-list request.
type ModelRequest struct {
	URL     string
	Headers map[string]string
}

// CheckEndpoint enforces the outbound rule: https anywhere, http only to
// loopback, never credentials in the URL (endpoint.ts:1-42).
func CheckEndpoint(raw string) (*url.URL, *contract.Failure) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, invalid("地址必须是 http/https 且包含主机")
	}
	if parsed.User != nil {
		return nil, invalid("地址不能带用户名密码")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return nil, invalid("明文 http 只允许回环地址")
	}
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ModelListRequest builds the URL and headers for both provider kinds.
// openai: {base}/models unless base already ends with /models. anthropic:
// strip a trailing /v1, then /v1/models (modelCatalog.ts:44-68).
func ModelListRequest(kind, baseURL, apiKey string) (ModelRequest, *contract.Failure) {
	parsed, f := CheckEndpoint(baseURL)
	if f != nil {
		return ModelRequest{}, f
	}
	trimmed := strings.TrimRight(parsed.String(), "/")
	headers := map[string]string{"Accept": "application/json"}
	key := strings.TrimSpace(apiKey)
	if kind == "anthropic" {
		root := strings.TrimSuffix(trimmed, "/v1")
		headers["anthropic-version"] = AnthropicVersion
		if key != "" {
			headers["x-api-key"] = key
		}
		return ModelRequest{URL: root + "/v1/models", Headers: headers}, nil
	}
	if key != "" {
		headers["Authorization"] = "Bearer " + key
	}
	if strings.HasSuffix(trimmed, "/models") {
		return ModelRequest{URL: trimmed, Headers: headers}, nil
	}
	return ModelRequest{URL: trimmed + "/models", Headers: headers}, nil
}

// ParseModelList reads {data:[{id}]}, strips the models/ prefix, drops empty,
// control-character and over-long ids, dedupes, sorts and caps at 500
// (modelCatalog.ts:70-87).
func ParseModelList(body []byte) ([]string, *contract.Failure) {
	var payload struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, contract.Fail(contract.CodeIOFailure, "供应商返回的不是 JSON")
	}
	if payload.Data == nil {
		return nil, contract.Fail(contract.CodeIOFailure, "供应商返回的模型列表格式不对")
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		var entry struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(item, &entry) != nil {
			continue
		}
		clean := strings.TrimPrefix(strings.TrimSpace(entry.ID), "models/")
		if clean == "" || len(clean) > MaxModelIDLen || controlOrSpace.MatchString(clean) {
			continue
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	sort.Strings(out)
	if len(out) > MaxFetchedModels {
		out = out[:MaxFetchedModels]
	}
	return out, nil
}

// FetchModelsInput is injectable for tests (client and timeout).
type FetchModelsInput struct {
	Kind    string
	BaseURL string
	APIKey  string
	Timeout time.Duration
	Client  *http.Client
}

// FetchModels fetches a provider's model list. Errors mirror the legacy
// catalog: 401/403 permission-denied, 404/405 unsupported, everything else
// io-failure with a sanitized snippet.
func FetchModels(ctx context.Context, input FetchModelsInput) ([]string, *contract.Failure) {
	request, f := ModelListRequest(input.Kind, input.BaseURL, input.APIKey)
	if f != nil {
		return nil, f
	}
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = defaultFetchTimeout
	}
	client := input.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		return nil, contract.Fail(contract.CodeIOFailure, "构造请求失败")
	}
	for name, value := range request.Headers {
		req.Header.Set(name, value)
	}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, contract.Fail(contract.CodeIOFailure, "请求已取消")
		}
		if isTimeout(err) {
			return nil, contract.Fail(contract.CodeIOFailure, fmt.Sprintf("供应商 %d 秒内没有响应", int(timeout.Seconds())))
		}
		return nil, contract.Fail(contract.CodeIOFailure, "连不上供应商地址，请检查请求地址与网络")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxModelsBody+1))
	if err != nil {
		return nil, contract.Fail(contract.CodeIOFailure, "读取供应商响应失败")
	}
	if len(body) > maxModelsBody {
		return nil, contract.Fail(contract.CodeTooLarge, "模型列表过大")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		snippet := sanitizeSnippet(string(body), input.APIKey)
		switch {
		case response.StatusCode == 401 || response.StatusCode == 403:
			return nil, contract.Fail(contract.CodePermissionDenied, fmt.Sprintf("供应商拒绝了 API key（%d）", response.StatusCode))
		case response.StatusCode == 404 || response.StatusCode == 405:
			return nil, contract.Fail(contract.CodeUnsupported, fmt.Sprintf("这个地址没有提供模型列表接口（%d），请手动填写模型名", response.StatusCode))
		}
		message := fmt.Sprintf("供应商返回 %d", response.StatusCode)
		if snippet != "" {
			message += "：" + snippet
		}
		return nil, contract.Fail(contract.CodeIOFailure, message)
	}
	return ParseModelList(body)
}

func isTimeout(err error) bool {
	var netErr net.Error
	if ok := asNetError(err, &netErr); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "timeout")
}

func asNetError(err error, target *net.Error) bool {
	for err != nil {
		if netErr, ok := err.(net.Error); ok {
			*target = netErr
			return true
		}
		type unwrapper interface{ Unwrap() error }
		next, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = next.Unwrap()
	}
	return false
}

// sanitizeSnippet replaces the request's own key first (it may straddle the
// cut), then common secret shapes and user paths, then truncates.
func sanitizeSnippet(body, key string) string {
	trimmed := strings.TrimSpace(body)
	if key = strings.TrimSpace(key); key != "" {
		trimmed = strings.ReplaceAll(trimmed, key, "<redacted>")
	}
	trimmed = secretPattern.ReplaceAllString(trimmed, "<redacted>")
	trimmed = userPath.ReplaceAllString(trimmed, "<path>")
	if len(trimmed) > snippetLimit {
		trimmed = trimmed[:snippetLimit]
	}
	return strings.TrimSpace(trimmed)
}
