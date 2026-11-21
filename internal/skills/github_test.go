package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const (
	headSHA = "0123456789abcdef0123456789abcdef01234567"
	pathSHA = "89abcdef0123456789abcdef0123456789abcdef"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*GithubClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client := NewGithubClientForTest()
	client.APIBase = server.URL
	client.CodeloadBase = server.URL
	client.SetHTTPForTest(server.Client().Transport)
	return client, server
}

func TestGithubResolve(t *testing.T) {
	var sawUserAgent, sawSHAAlt string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		sawUserAgent = r.Header.Get("User-Agent")
		switch r.URL.Path {
		case "/repos/acme/skills":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"default_branch": "main"})
		case "/repos/acme/skills/commits/main":
			sawSHAAlt = r.Header.Get("Accept")
			_, _ = w.Write([]byte(headSHA + "\n"))
		case "/repos/acme/skills/commits":
			if r.URL.Query().Get("path") != "skills/pdf" || r.URL.Query().Get("sha") != "main" {
				t.Errorf("path commit query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": pathSHA}})
		default:
			http.NotFound(w, r)
		}
	})

	resolved, f := client.Resolve(context.Background(), GithubRef{Owner: "acme", Repo: "skills", Path: "skills/pdf"})
	if f != nil {
		t.Fatalf("resolve: %+v", f)
	}
	if resolved.Ref != "main" || resolved.Commit != headSHA || resolved.PathCommit != pathSHA {
		t.Fatalf("resolved = %+v", resolved)
	}
	if !strings.HasPrefix(sawUserAgent, "Mile-Knowledge-Base/") {
		t.Fatalf("user agent = %q", sawUserAgent)
	}
	if sawSHAAlt != "application/vnd.github.sha" {
		t.Fatalf("sha accept = %q", sawSHAAlt)
	}
}

func TestGithubResolvePathFallback(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/skills/commits/main":
			_, _ = w.Write([]byte(headSHA))
		case r.URL.Path == "/repos/acme/skills/commits":
			_, _ = w.Write([]byte("[]"))
		default:
			http.NotFound(w, r)
		}
	})
	resolved, f := client.Resolve(context.Background(), GithubRef{Owner: "acme", Repo: "skills", Ref: "main", Path: "gone"})
	if f != nil {
		t.Fatalf("resolve: %+v", f)
	}
	if resolved.PathCommit != headSHA {
		t.Fatalf("path commit = %q, want head", resolved.PathCommit)
	}
}

func TestGithubResolveErrors(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/missing/"):
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/repos/limited/"):
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-reset", "1700000000")
			w.WriteHeader(http.StatusForbidden)
		case strings.HasPrefix(r.URL.Path, "/repos/badsha/"):
			_, _ = w.Write([]byte("not-a-sha"))
		default:
			http.NotFound(w, r)
		}
	})
	cases := []struct {
		name string
		ref  GithubRef
		code string
	}{
		{"missing", GithubRef{Owner: "missing", Repo: "repo"}, contract.CodeNotFound},
		{"limited", GithubRef{Owner: "limited", Repo: "repo"}, contract.CodeRateLimited},
		{"badsha", GithubRef{Owner: "badsha", Repo: "repo"}, contract.CodeNetworkFailure},
	}
	for _, item := range cases {
		_, f := client.Resolve(context.Background(), item.ref)
		if f == nil || f.Code != item.code {
			t.Errorf("%s: got %+v, want %s", item.name, f, item.code)
		}
		if item.code == contract.CodeRateLimited && !strings.Contains(f.Detail, "额度重置") {
			t.Errorf("rate limit detail = %q", f.Detail)
		}
	}
}

func TestGithubResolveTimeout(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(headSHA))
	})
	client.APITimeout = 50 * time.Millisecond
	started := time.Now()
	_, f := client.Resolve(context.Background(), GithubRef{Owner: "acme", Repo: "skills", Ref: "main"})
	if f == nil || f.Code != contract.CodeNetworkFailure {
		t.Fatalf("timeout = %+v", f)
	}
	if elapsed := time.Since(started); elapsed > 250*time.Millisecond {
		t.Fatalf("timeout took %v, want the context to cancel quickly", elapsed)
	}
}

func TestGithubRetriesOnce(t *testing.T) {
	var calls int32
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/commits/main") {
			_, _ = w.Write([]byte(headSHA))
			return
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"default_branch":"main"}`))
	})
	if _, f := client.Resolve(context.Background(), GithubRef{Owner: "acme", Repo: "skills"}); f != nil {
		t.Fatalf("resolve after retry: %+v", f)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestGithubDownload(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/acme/skills/tar.gz/" + headSHA:
			http.Redirect(w, r, "/archive", http.StatusFound)
		case "/archive":
			_, _ = w.Write([]byte("tarball-bytes"))
		case "/evil/skills/tar.gz/" + headSHA:
			http.Redirect(w, r, "http://evil.example/archive", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})
	var buffer bytes.Buffer
	written, f := client.DownloadTarball(context.Background(), GithubRef{Owner: "acme", Repo: "skills"}, headSHA, &buffer)
	if f != nil {
		t.Fatalf("download: %+v", f)
	}
	if written != int64(len("tarball-bytes")) || buffer.String() != "tarball-bytes" {
		t.Fatalf("written = %d, body = %q", written, buffer.String())
	}
}

func TestGithubDownloadRejectsForeignRedirect(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://evil.example/archive", http.StatusFound)
	})
	var buffer bytes.Buffer
	_, f := client.DownloadTarball(context.Background(), GithubRef{Owner: "evil", Repo: "skills"}, headSHA, &buffer)
	if f == nil || f.Code != contract.CodeNetworkFailure {
		t.Fatalf("foreign redirect = %+v", f)
	}
	if buffer.Len() != 0 {
		t.Fatalf("foreign redirect wrote %d bytes", buffer.Len())
	}
}

func TestGithubDownloadSizeCap(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 100))
	})
	client.MaxDownload = 10
	var buffer bytes.Buffer
	_, f := client.DownloadTarball(context.Background(), GithubRef{Owner: "acme", Repo: "skills"}, headSHA, &buffer)
	if f == nil || f.Code != contract.CodeTooLarge {
		t.Fatalf("size cap = %+v", f)
	}
	if f.Limit != 10 || f.Size <= 10 {
		t.Fatalf("limit = %d size = %d", f.Limit, f.Size)
	}
	if buffer.Len() > 11 {
		t.Fatalf("cap did not stop the stream: %d bytes", buffer.Len())
	}
}

func TestRateLimitFailure(t *testing.T) {
	client := NewGithubClientForTest()
	response := &http.Response{StatusCode: http.StatusForbidden, Header: http.Header{}}
	if f := client.rateLimitFailure(response); f != nil {
		t.Fatalf("no remaining header should not be a rate limit: %+v", f)
	}
	reset := time.Now().Add(time.Hour).Truncate(time.Second)
	response.Header.Set("x-ratelimit-remaining", "0")
	response.Header.Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))
	f := client.rateLimitFailure(response)
	if f == nil || f.Code != contract.CodeRateLimited || !strings.Contains(f.Detail, "额度重置") {
		t.Fatalf("rate limit = %+v", f)
	}
	if !client.RateLimitResetAt().Equal(reset) {
		t.Fatalf("reset = %v, want %v", client.RateLimitResetAt(), reset)
	}
}

func TestGithubLatestPathCommit(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/skills/commits/main":
			_, _ = w.Write([]byte(headSHA))
		case "/repos/acme/skills/commits":
			if r.URL.Query().Get("path") != "skills/pdf" || r.URL.Query().Get("sha") != "main" {
				t.Errorf("path commit query = %s", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": pathSHA}})
		case "/repos/acme/gone/commits":
			_ = json.NewEncoder(w).Encode([]map[string]string{})
		default:
			http.NotFound(w, r)
		}
	})

	commit, f := client.LatestPathCommit(context.Background(), GithubRef{Owner: "acme", Repo: "skills", Ref: "main", Path: "skills/pdf"})
	if f != nil || commit != pathSHA {
		t.Fatalf("path commit = %q %+v", commit, f)
	}
	commit, f = client.LatestPathCommit(context.Background(), GithubRef{Owner: "acme", Repo: "skills", Ref: "main"})
	if f != nil || commit != headSHA {
		t.Fatalf("root commit = %q %+v", commit, f)
	}
	if _, f := client.LatestPathCommit(context.Background(), GithubRef{Owner: "acme", Repo: "gone", Ref: "main", Path: "skills/pdf"}); f == nil || f.Code != contract.CodeNotFound {
		t.Fatalf("empty history = %+v", f)
	}
	if _, f := client.LatestPathCommit(context.Background(), GithubRef{Owner: "acme", Repo: "skills"}); f == nil || f.Code != contract.CodeInvalidInput {
		t.Fatalf("missing ref = %+v", f)
	}
}
