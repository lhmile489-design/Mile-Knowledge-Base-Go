package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/netproxy"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// DefaultIdleTimeout aborts a stream that delivered no bytes for this long
// (openaiClient.ts:22).
const DefaultIdleTimeout = 300 * time.Second

// maxStreamLine guards against a provider sending one endless line.
const maxStreamLine = 8 * 1024 * 1024

// EngineError carries one of the API error codes plus a sanitized message.
type EngineError struct {
	Code    string
	Message string
}

func (e *EngineError) Error() string { return e.Message }

// ToolFunction is the wire shape of a function call.
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatToolCallWire is one tool call in an assistant message.
type ChatToolCallWire struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ChatMessage is one message of the OpenAI chat protocol.
type ChatMessage struct {
	Role             string             `json:"role"`
	Content          *string            `json:"content"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
	ToolCalls        []ChatToolCallWire `json:"tool_calls,omitempty"`
	ReasoningContent *string            `json:"reasoning_content,omitempty"`
}

// ChatRequest is one streaming completion request.
type ChatRequest struct {
	BaseURL   string
	APIKey    string
	Model     string
	Messages  []ChatMessage
	Tools     []json.RawMessage
	MaxTokens int
}

// StreamOptions makes transport and timing injectable.
type StreamOptions struct {
	Client      *http.Client
	IdleTimeout time.Duration
	MaxRetries  *int
	OnRetry     func(RetryInfo)
	Delay       func(attempt int, retryAfter time.Duration) time.Duration
	Random      func() float64
}

func chatCompletionsURL(base string) (string, *EngineError) {
	parsed, f := CheckEndpoint(base)
	if f != nil {
		return "", &EngineError{Code: f.Code, Message: f.Message}
	}
	return strings.TrimRight(parsed.String(), "/") + "/chat/completions", nil
}

type chatPayload struct {
	Model         string            `json:"model"`
	Messages      []ChatMessage     `json:"messages"`
	Stream        bool              `json:"stream"`
	StreamOptions map[string]bool   `json:"stream_options"`
	Tools         []json.RawMessage `json:"tools,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
}

// StreamChat performs one request and emits chunks until the stream ends.
// Errors: EngineError for provider/protocol problems, ctx.Err() on cancel.
func StreamChat(ctx context.Context, request ChatRequest, options StreamOptions, emit func(StreamChunk) error) error {
	url, engineErr := chatCompletionsURL(request.BaseURL)
	if engineErr != nil {
		return engineErr
	}
	payload := chatPayload{
		Model: request.Model, Messages: request.Messages, Stream: true,
		StreamOptions: map[string]bool{"include_usage": true},
		Tools:         request.Tools, MaxTokens: request.MaxTokens,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return &EngineError{Code: contract.CodeInvalidInput, Message: "请求无法序列化"}
	}

	client := options.Client
	if client == nil {
		client = &http.Client{Transport: netproxy.Transport()}
	}
	idle := options.IdleTimeout
	if idle <= 0 {
		idle = DefaultIdleTimeout
	}

	child, cancel := context.WithCancel(ctx)
	defer cancel()
	var stalled int32
	idleTimer := time.AfterFunc(idle, func() {
		atomic.StoreInt32(&stalled, 1)
		cancel()
	})
	defer idleTimer.Stop()

	response, err := SendWithRetry(child, func() (*http.Response, error) {
		req, reqErr := http.NewRequestWithContext(child, http.MethodPost, url, bytes.NewReader(body))
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		if request.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+request.APIKey)
		}
		return client.Do(req)
	}, RetryOptions{MaxRetries: options.MaxRetries, OnRetry: options.OnRetry, Delay: options.Delay, Random: options.Random})
	if err != nil {
		if atomic.LoadInt32(&stalled) == 1 {
			return stalledError(idle)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return redactKey(err, request.APIKey)
	}
	defer response.Body.Close()

	assembler := &Assembler{}
	reader := bufio.NewReaderSize(response.Body, 64*1024)
	finished := false
	for !finished {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			if len(line) > maxStreamLine {
				return &EngineError{Code: contract.CodeProviderError, Message: "供应商返回了无法解析的流式数据：单行过长"}
			}
			idleTimer.Reset(idle)
			chunks, done, pushErr := assembler.Push(strings.TrimSpace(line))
			if pushErr != nil {
				return mapStreamError(pushErr)
			}
			for _, chunk := range chunks {
				if err := emit(chunk); err != nil {
					return err
				}
			}
			if done {
				finished = true
				break
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			if atomic.LoadInt32(&stalled) == 1 {
				return stalledError(idle)
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return &EngineError{Code: contract.CodeProviderError, Message: "读取供应商响应失败"}
		}
	}

	if atomic.LoadInt32(&stalled) == 1 {
		return stalledError(idle)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !assembler.Complete() {
		return &EngineError{Code: contract.CodeProviderError, Message: "供应商的响应在完成前就结束了"}
	}
	for _, chunk := range assembler.Finish() {
		if err := emit(chunk); err != nil {
			return err
		}
	}
	return nil
}

func stalledError(idle time.Duration) error {
	seconds := int(idle.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return &EngineError{
		Code:    contract.CodeProviderError,
		Message: fmt.Sprintf("供应商 %d 秒没有返回任何数据，连接可能已断开", seconds),
	}
}

func mapStreamError(err error) error {
	var decode *StreamDecodeError
	if errors.As(err, &decode) {
		return &EngineError{Code: contract.CodeProviderError, Message: decode.Error()}
	}
	var provider *ProviderStreamError
	if errors.As(err, &provider) {
		return &EngineError{Code: contract.CodeProviderError, Message: provider.Error()}
	}
	return &EngineError{Code: contract.CodeProviderError, Message: err.Error()}
}

// redactKey replaces the exact key in an engine error before it reaches the
// frames or the history. Providers sometimes echo the request back, and the
// generic Redact patterns do not know this turn's key (models.go does the
// same for fetchModels snippets).
func redactKey(err error, key string) error {
	if err == nil || key == "" {
		return err
	}
	var engineErr *EngineError
	if !errors.As(err, &engineErr) {
		return err
	}
	replaced := strings.ReplaceAll(engineErr.Message, key, "<redacted>")
	if replaced == engineErr.Message {
		return err
	}
	clone := *engineErr
	clone.Message = replaced
	return &clone
}
