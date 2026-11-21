package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M5 T08: M93 update detection (技术设计 16.5).

const (
	updateTestBaseHash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	updateTestSHA      = "0123456789abcdef0123456789abcdef01234567"
	updateTestNewSHA   = "89abcdef0123456789abcdef0123456789abcdef"
)

func updateTestService(t *testing.T) (*SkillService, string) {
	t.Helper()
	svc, vaultID := skillServiceWithHome(t, t.TempDir())
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	return svc, vaultID
}

func injectUpdateClient(svc *SkillService, server *httptest.Server) {
	svc.githubClient = func(string) *skills.GithubClient {
		client := skills.NewGithubClientForTest()
		client.APIBase = server.URL
		client.CodeloadBase = server.URL
		client.SetHTTPForTest(server.Client().Transport)
		return client
	}
}

func writeKbSourceRecord(t *testing.T, svc *SkillService, vaultID, path, skillPath string) {
	t.Helper()
	root, f := svc.app.Vaults.Root(vaultID)
	if f != nil {
		t.Fatal(f)
	}
	record := skills.KbSource{
		SkillPath: skillPath, Owner: "anthropics", Repo: "skills", Ref: "main", Path: path,
		Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: updateTestBaseHash,
		InstalledAt: "2026-11-21T09:00:00Z",
	}
	if f := skills.SaveKbSource(root, record); f != nil {
		t.Fatal(f)
	}
}

func waitUpdateRound(t *testing.T, svc *SkillService) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		svc.mu.Lock()
		checking := svc.checking
		svc.mu.Unlock()
		if !checking {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("update round did not finish")
}

func updateServer(t *testing.T, commit string, requests *int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(requests, 1)
		if r.URL.Path != "/repos/anthropics/skills/commits" && r.URL.Path != "/repos/acme/skills/commits" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": commit}})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestSkillServiceCheckUpdatesRoundsAndCaches(t *testing.T) {
	svc, vaultID := updateTestService(t)
	var requests int64
	injectUpdateClient(svc, updateServer(t, updateTestSHA, &requests))
	writeKbSourceRecord(t, svc, vaultID, "skills/pdf", "skills/文档/pdf")

	first := svc.CheckUpdates(context.Background(), vaultID, false)
	if !first.Checking || first.Groups != 1 || first.CheckedAt != "" {
		t.Fatalf("first = %+v", first)
	}
	waitUpdateRound(t, svc)
	if got := atomic.LoadInt64(&requests); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
	cache := skills.LoadUpdateCache(filepath.Join(svc.app.DataDir, "skills"))
	if cache == nil || len(cache.Results) != 1 || cache.Results[0].LatestPathCommit != updateTestSHA || cache.CheckedAt == "" {
		t.Fatalf("cache = %+v", cache)
	}

	second := svc.CheckUpdates(context.Background(), vaultID, false)
	if second.Checking || second.CheckedAt != cache.CheckedAt || second.Unchecked != 0 {
		t.Fatalf("cached = %+v", second)
	}
	if got := atomic.LoadInt64(&requests); got != 1 {
		t.Fatalf("fresh cache made a request: %d", got)
	}

	forced := svc.CheckUpdates(context.Background(), vaultID, true)
	if !forced.Checking {
		t.Fatalf("force = %+v", forced)
	}
	waitUpdateRound(t, svc)
	if got := atomic.LoadInt64(&requests); got != 2 {
		t.Fatalf("force requests = %d, want 2", got)
	}
}

func TestSkillServiceCheckUpdatesExpiresAfterSixtyMinutes(t *testing.T) {
	svc, vaultID := updateTestService(t)
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	var requests int64
	injectUpdateClient(svc, updateServer(t, updateTestSHA, &requests))
	writeKbSourceRecord(t, svc, vaultID, "skills/pdf", "skills/文档/pdf")

	svc.CheckUpdates(context.Background(), vaultID, false)
	waitUpdateRound(t, svc)

	now = now.Add(59 * time.Minute)
	if result := svc.CheckUpdates(context.Background(), vaultID, false); result.Checking {
		t.Fatalf("59 minutes should use the cache")
	}
	now = now.Add(2 * time.Minute)
	if result := svc.CheckUpdates(context.Background(), vaultID, false); !result.Checking {
		t.Fatalf("61 minutes should start a round")
	}
	waitUpdateRound(t, svc)
	if got := atomic.LoadInt64(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestSkillServiceCheckUpdatesStopsAtRateLimit(t *testing.T) {
	svc, vaultID := updateTestService(t)
	reset := time.Now().Add(30 * time.Minute).Truncate(time.Second)
	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&requests, 1) >= 2 {
			w.Header().Set("x-ratelimit-remaining", "0")
			w.Header().Set("x-ratelimit-reset", strconv.FormatInt(reset.Unix(), 10))
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": updateTestSHA}})
	}))
	t.Cleanup(server.Close)
	injectUpdateClient(svc, server)
	writeKbSourceRecord(t, svc, vaultID, "skills/a", "skills/文档/a")
	writeKbSourceRecord(t, svc, vaultID, "skills/b", "skills/文档/b")
	writeKbSourceRecord(t, svc, vaultID, "skills/c", "skills/文档/c")

	svc.CheckUpdates(context.Background(), vaultID, true)
	waitUpdateRound(t, svc)
	if got := atomic.LoadInt64(&requests); got != 2 {
		t.Fatalf("requests = %d, want the round to stop after the rate limit", got)
	}
	cache := skills.LoadUpdateCache(filepath.Join(svc.app.DataDir, "skills"))
	if cache == nil || len(cache.Results) != 2 {
		t.Fatalf("cache = %+v", cache)
	}
	if cache.Results[0].Error != "" || cache.Results[1].Error == "" {
		t.Fatalf("results = %+v", cache.Results)
	}
	result := svc.CheckUpdates(context.Background(), vaultID, false)
	if result.Checking || result.Unchecked != 2 {
		t.Fatalf("after rate limit = %+v", result)
	}
	if result.RateLimitResetAt != reset.UTC().Format(time.RFC3339) {
		t.Fatalf("reset = %q, want %q", result.RateLimitResetAt, reset.UTC().Format(time.RFC3339))
	}
}

func TestSkillServiceCheckUpdatesCapsAtFortyGroups(t *testing.T) {
	svc, _ := updateTestService(t)
	var requests int64
	injectUpdateClient(svc, updateServer(t, updateTestSHA, &requests))
	skillsDir := filepath.Join(svc.app.DataDir, "skills")
	for i := 0; i < 41; i++ {
		record := skills.ToolSource{
			ToolID: "claude-code", TargetPath: fmt.Sprintf(`C:\tools\skills\s%02d`, i),
			Owner: "acme", Repo: "skills", Ref: "main", Path: fmt.Sprintf("skills/s%02d", i),
			Commit: updateTestSHA, PathCommit: updateTestSHA, BaseHash: updateTestBaseHash,
			InstalledAt: "2026-11-21T09:00:00Z",
		}
		if f := skills.SaveToolSource(skillsDir, record); f != nil {
			t.Fatal(f)
		}
	}

	svc.CheckUpdates(context.Background(), "", true)
	waitUpdateRound(t, svc)
	if got := atomic.LoadInt64(&requests); got != skills.MaxUpdateGroups {
		t.Fatalf("requests = %d, want %d", got, skills.MaxUpdateGroups)
	}
	cache := skills.LoadUpdateCache(skillsDir)
	if cache == nil || len(cache.Results) != skills.MaxUpdateGroups {
		t.Fatalf("cache results = %d", len(cache.Results))
	}
	result := svc.CheckUpdates(context.Background(), "", false)
	if result.Groups != 41 || result.Unchecked != 1 {
		t.Fatalf("capped = %+v", result)
	}
}

func TestSkillServiceCheckUpdatesRunsOneRound(t *testing.T) {
	svc, vaultID := updateTestService(t)
	gate := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(release)
	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		<-gate
		_ = json.NewEncoder(w).Encode([]map[string]string{{"sha": updateTestSHA}})
	}))
	t.Cleanup(server.Close)
	injectUpdateClient(svc, server)
	writeKbSourceRecord(t, svc, vaultID, "skills/pdf", "skills/文档/pdf")

	if first := svc.CheckUpdates(context.Background(), vaultID, true); !first.Checking {
		t.Fatalf("first = %+v", first)
	}
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt64(&requests) == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	second := svc.CheckUpdates(context.Background(), vaultID, true)
	if !second.Checking || atomic.LoadInt64(&requests) != 1 {
		t.Fatalf("second round started: %+v requests=%d", second, atomic.LoadInt64(&requests))
	}
	release()
	waitUpdateRound(t, svc)
}
