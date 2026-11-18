package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestModelListRequest(t *testing.T) {
	cases := []struct {
		kind    string
		base    string
		wantURL string
	}{
		{"openai", "https://api.test/v1", "https://api.test/v1/models"},
		{"openai", "https://api.test/v1/", "https://api.test/v1/models"},
		{"openai", "https://api.test/models", "https://api.test/models"},
		{"anthropic", "https://api.test/v1", "https://api.test/v1/models"},
		{"anthropic", "https://api.test", "https://api.test/v1/models"},
	}
	for _, c := range cases {
		got, f := ModelListRequest(c.kind, c.base, "sk-test")
		if f != nil {
			t.Fatalf("%s %s: %+v", c.kind, c.base, f)
		}
		if got.URL != c.wantURL {
			t.Errorf("%s %s: url=%s want %s", c.kind, c.base, got.URL, c.wantURL)
		}
		if c.kind == "openai" && got.Headers["Authorization"] != "Bearer sk-test" {
			t.Errorf("openai auth header: %v", got.Headers)
		}
		if c.kind == "anthropic" && (got.Headers["x-api-key"] != "sk-test" || got.Headers["anthropic-version"] != "2023-06-01") {
			t.Errorf("anthropic headers: %v", got.Headers)
		}
	}

	anon, _ := ModelListRequest("openai", "https://api.test", "")
	if _, ok := anon.Headers["Authorization"]; ok {
		t.Errorf("empty key should send no auth header: %v", anon.Headers)
	}
}

func TestParseModelList(t *testing.T) {
	raw := `{"data":[{"id":"models/z"},{"id":"a"},{"id":"a"},{"id":""},{"id":"bad name"},{"id":5},{"id":"` + strings.Repeat("m", 129) + `"},{"id":"models/b"}]}`
	got, f := ParseModelList([]byte(raw))
	if f != nil {
		t.Fatal(f)
	}
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "z" {
		t.Fatalf("parsed: %v", got)
	}

	if _, f := ParseModelList([]byte(`{"data":{}}`)); f == nil || f.Code != contract.CodeIOFailure {
		t.Fatalf("bad shape: %+v", f)
	}

	many := `{"data":[` + strings.Join(modelIDs(600), ",") + `]}`
	got, _ = ParseModelList([]byte(many))
	if len(got) != 500 {
		t.Fatalf("cap: %d", len(got))
	}
}

func modelIDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = `{"id":"m` + strings.Repeat("0", 3-len(itoaSmall(i))) + itoaSmall(i) + `"}`
	}
	return out
}

func itoaSmall(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

func fetchServer(t *testing.T, status int, body string, delay time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchModels(t *testing.T) {
	ok := fetchServer(t, 200, `{"data":[{"id":"m2"},{"id":"m1"}]}`, 0)
	defer ok.Close()
	got, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: ok.URL, APIKey: "sk-x"})
	if f != nil || len(got) != 2 || got[0] != "m1" {
		t.Fatalf("ok case: %v %+v", got, f)
	}

	auth := fetchServer(t, 401, `bad key sk-secret-value`, 0)
	defer auth.Close()
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: auth.URL, APIKey: "sk-secret-value"}); f == nil || f.Code != contract.CodePermissionDenied {
		t.Fatalf("401: %+v", f)
	}

	missing := fetchServer(t, 404, ``, 0)
	defer missing.Close()
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: missing.URL}); f == nil || f.Code != contract.CodeUnsupported {
		t.Fatalf("404: %+v", f)
	}

	echo := fetchServer(t, 500, `invalid key sk-secret-value at C:\Users\someone\key.txt`, 0)
	defer echo.Close()
	_, f = FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: echo.URL, APIKey: "sk-secret-value"})
	if f == nil || f.Code != contract.CodeIOFailure {
		t.Fatalf("500: %+v", f)
	}
	if strings.Contains(f.Message, "sk-secret-value") || strings.Contains(f.Message, "Users") {
		t.Fatalf("error leaks secret or path: %s", f.Message)
	}

	slow := fetchServer(t, 200, `{"data":[]}`, 200*time.Millisecond)
	defer slow.Close()
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: slow.URL, Timeout: 30 * time.Millisecond}); f == nil || f.Code != contract.CodeIOFailure {
		t.Fatalf("timeout: %+v", f)
	}

	big := fetchServer(t, 200, strings.Repeat("x", 2*1024*1024+1), 0)
	defer big.Close()
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: big.URL}); f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("too large: %+v", f)
	}

	notJSON := fetchServer(t, 200, `<html></html>`, 0)
	defer notJSON.Close()
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: notJSON.URL}); f == nil || f.Code != contract.CodeIOFailure {
		t.Fatalf("non-json: %+v", f)
	}

	// http to a non-loopback host must be refused before any key is sent.
	if _, f := FetchModels(context.Background(), FetchModelsInput{Kind: "openai", BaseURL: "http://example.com/v1", APIKey: "sk-x"}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("plain http to external host: %+v", f)
	}
}
