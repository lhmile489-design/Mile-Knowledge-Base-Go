package ai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestRetryableStatus(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 599} {
		if !RetryableStatus(status) {
			t.Errorf("%d should be retryable", status)
		}
	}
	for _, status := range []int{400, 401, 403, 404, 422} {
		if RetryableStatus(status) {
			t.Errorf("%d should not be retryable", status)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		attempt int
		jitter  float64
		want    time.Duration
	}{
		{1, 0, 500 * time.Millisecond},
		{2, 0, time.Second},
		{3, 0, 2 * time.Second},
		{10, 0, 15 * time.Second}, // capped
		{1, 0.999, 749 * time.Millisecond},
	}
	for _, c := range cases {
		got := BackoffDelay(c.attempt, 0, func() float64 { return c.jitter })
		if got != c.want {
			t.Errorf("attempt %d jitter %v: %v want %v", c.attempt, c.jitter, got, c.want)
		}
	}
	if got := BackoffDelay(5, 3*time.Second, func() float64 { return 0 }); got != 3*time.Second {
		t.Errorf("Retry-After should win: %v", got)
	}
	if got := BackoffDelay(5, 90*time.Second, func() float64 { return 0 }); got != 60*time.Second {
		t.Errorf("Retry-After cap: %v", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if got := ParseRetryAfter("3", now); got != 3*time.Second {
		t.Errorf("seconds: %v", got)
	}
	if got := ParseRetryAfter("", now); got != 0 {
		t.Errorf("empty: %v", got)
	}
	if got := ParseRetryAfter("Wed, 18 Nov 2026 12:00:05 GMT", now); got != 5*time.Second {
		t.Errorf("http date: %v", got)
	}
	if got := ParseRetryAfter("not a date", now); got != 0 {
		t.Errorf("garbage: %v", got)
	}
	if got := ParseRetryAfter("Wed, 18 Nov 2026 11:00:00 GMT", now); got != 0 {
		t.Errorf("past date must be zero: %v", got)
	}
}

func TestSendWithRetry(t *testing.T) {
	attempts := 0
	response, err := SendWithRetry(context.Background(), func() (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("busy")), Header: http.Header{}}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
	}, RetryOptions{MaxRetries: ptr(5), Delay: func(int, time.Duration) time.Duration { return 0 }})
	if err != nil || attempts != 3 || response.StatusCode != 200 {
		t.Fatalf("retry: attempts=%d err=%v", attempts, err)
	}

	attempts = 0
	_, err = SendWithRetry(context.Background(), func() (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("bad key")), Header: http.Header{}}, nil
	}, RetryOptions{MaxRetries: ptr(5), Delay: func(int, time.Duration) time.Duration { return 0 }})
	var engineErr *EngineError
	if !errors.As(err, &engineErr) || engineErr.Code != contract.CodePermissionDenied || attempts != 1 {
		t.Fatalf("401: attempts=%d err=%v", attempts, err)
	}

	attempts = 0
	_, err = SendWithRetry(context.Background(), func() (*http.Response, error) {
		attempts++
		return nil, errors.New("connection reset")
	}, RetryOptions{MaxRetries: ptr(2), Delay: func(int, time.Duration) time.Duration { return 0 }})
	if err == nil || attempts != 3 {
		t.Fatalf("network retries: attempts=%d err=%v", attempts, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SendWithRetry(ctx, func() (*http.Response, error) {
		t.Fatal("send must not run after cancel")
		return nil, nil
	}, RetryOptions{}); err == nil {
		t.Fatal("cancelled context succeeded")
	}
}
