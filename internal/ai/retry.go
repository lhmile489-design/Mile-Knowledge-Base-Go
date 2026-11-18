package ai

import (
	"context"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Retry limits (retry.ts:16-19).
const (
	MaxRetries        = 10
	MaxBackoff        = 15 * time.Second
	MaxRetryAfter     = 60 * time.Second
	maxErrorBodyBytes = 4096
)

// RetryInfo is what the UI shows as "第 n/m 次重试".
type RetryInfo struct {
	Attempt int
	Max     int
	Delay   time.Duration
	Err     error
}

// RetryOptions makes the timing injectable for tests.
type RetryOptions struct {
	MaxRetries *int
	OnRetry    func(RetryInfo)
	Delay      func(attempt int, retryAfter time.Duration) time.Duration
	Random     func() float64
}

// RetryableStatus: 408/429/5xx retry, everything else 4xx fails at once.
func RetryableStatus(status int) bool {
	return status == 408 || status == 429 || (status >= 500 && status <= 599)
}

// BackoffDelay is 500ms * 2^(n-1), capped at 15s, plus up to 250ms jitter. A
// server-provided Retry-After wins, capped at 60s.
func BackoffDelay(attempt int, retryAfter time.Duration, random func() float64) time.Duration {
	if retryAfter > 0 {
		if retryAfter > MaxRetryAfter {
			return MaxRetryAfter
		}
		return retryAfter
	}
	base := time.Duration(float64(500*time.Millisecond) * math.Pow(2, float64(attempt-1)))
	if base > MaxBackoff {
		base = MaxBackoff
	}
	if random == nil {
		random = func() float64 { return 0 }
	}
	return base + time.Duration(random()*250)*time.Millisecond
}

// ParseRetryAfter supports seconds and HTTP-date (RFC 9110). An unparseable
// value returns 0 so our own backoff applies.
func ParseRetryAfter(value string, now time.Time) time.Duration {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		if seconds < 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(trimmed)
	if err != nil {
		return 0
	}
	if when.Before(now) {
		return 0
	}
	return when.Sub(now)
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// SendWithRetry sends until it gets a 2xx response header. It never replays a
// stream that already started, so a half answer cannot appear twice.
func SendWithRetry(ctx context.Context, send func() (*http.Response, error), opts RetryOptions) (*http.Response, error) {
	max := MaxRetries
	if opts.MaxRetries != nil {
		max = *opts.MaxRetries
	}
	delayFor := opts.Delay
	if delayFor == nil {
		delayFor = func(attempt int, retryAfter time.Duration) time.Duration {
			return BackoffDelay(attempt, retryAfter, opts.Random)
		}
	}
	var lastErr error
	var retryAfter time.Duration

	for attempt := 0; attempt <= max; attempt++ {
		if attempt > 0 {
			delay := delayFor(attempt, retryAfter)
			if opts.OnRetry != nil {
				opts.OnRetry(RetryInfo{Attempt: attempt, Max: max, Delay: delay, Err: lastErr})
			}
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		retryAfter = 0

		response, err := send()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return response, nil
		}

		body := readErrorBody(response)
		retryAfter = ParseRetryAfter(response.Header.Get("Retry-After"), time.Now())
		if response.StatusCode == 401 || response.StatusCode == 403 {
			return nil, &EngineError{Code: contract.CodePermissionDenied, Message: providerStatusMessage(response.StatusCode, "")}
		}
		engineErr := &EngineError{Code: contract.CodeProviderError, Message: providerStatusMessage(response.StatusCode, body)}
		if !RetryableStatus(response.StatusCode) {
			return nil, engineErr
		}
		lastErr = engineErr
	}
	if lastErr == nil {
		lastErr = &EngineError{Code: contract.CodeProviderError, Message: "请求失败"}
	}
	return nil, lastErr
}

func readErrorBody(response *http.Response) string {
	if response.Body == nil {
		return ""
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxErrorBodyBytes))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

func providerStatusMessage(status int, body string) string {
	if status == 401 || status == 403 {
		return "供应商拒绝了 API key（" + strconv.Itoa(status) + "）"
	}
	if body == "" {
		return "供应商返回 " + strconv.Itoa(status)
	}
	return "供应商返回 " + strconv.Itoa(status) + "：" + body
}
