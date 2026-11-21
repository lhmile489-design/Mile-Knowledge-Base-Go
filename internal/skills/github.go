package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/netproxy"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const (
	defaultGithubAPI  = "https://api.github.com"
	defaultCodeload   = "https://codeload.github.com"
	defaultUserAgent  = "Mile-Knowledge-Base/dev"
	apiTimeout        = 15 * time.Second
	downloadTimeout   = 120 * time.Second
	maxDownloadBytes  = int64(200 << 20) // Q-SKM-5: the whole archive
	maxRedirects      = 3
	shaLength         = 40
	rateLimitExceeded = 403
)

// githubHosts are the only hosts the client may talk to (PRD 15.4).
var githubHosts = map[string]bool{
	"github.com":          true,
	"api.github.com":      true,
	"codeload.github.com": true,
}

// GithubClient is the anonymous GitHub client (技术设计 16.2). Every request
// goes through the system proxy like the AI providers do. Tests inject the
// base URLs; e2e builds read SKILLS_FAKE_GITHUB.
type GithubClient struct {
	APIBase      string
	CodeloadBase string
	UserAgent    string
	HTTP         *http.Client
	MaxDownload  int64
	APITimeout   time.Duration

	rateLimitReset time.Time
}

// ResolvedRef is what the client learned about a ref before downloading.
type ResolvedRef struct {
	Ref        string
	Commit     string // head commit of Ref
	PathCommit string // latest commit touching Path (same as Commit when Path is "")
}

// NewGithubClient builds the production client.
func NewGithubClient(version string) *GithubClient {
	client := NewGithubClientForTest()
	if version != "" {
		client.UserAgent = "Mile-Knowledge-Base/" + version
	}
	return client
}

// NewGithubClientForTest builds a client with the public defaults; tests set
// the base URLs and HTTP client afterwards. The e2e build swaps the bases for
// the local fake server here.
func NewGithubClientForTest() *GithubClient {
	client := &GithubClient{
		APIBase:      defaultGithubAPI,
		CodeloadBase: defaultCodeload,
		UserAgent:    defaultUserAgent,
		MaxDownload:  maxDownloadBytes,
		APITimeout:   apiTimeout,
	}
	if fake := fakeGithubBase(); fake != "" {
		base := strings.TrimRight(fake, "/")
		client.APIBase = base
		client.CodeloadBase = base
	}
	client.HTTP = &http.Client{
		Transport:     netproxy.Transport(),
		CheckRedirect: client.checkRedirect,
	}
	return client
}

// SetHTTPForTest swaps the transport while keeping the redirect policy.
func (c *GithubClient) SetHTTPForTest(transport http.RoundTripper) {
	c.HTTP = &http.Client{Transport: transport, CheckRedirect: c.checkRedirect}
}

func (c *GithubClient) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if !githubHosts[strings.ToLower(req.URL.Hostname())] && !c.isBaseHost(req.URL.Hostname()) {
		return fmt.Errorf("redirect to %s is not allowed", req.URL.Hostname())
	}
	return nil
}

// isBaseHost lets the e2e fake server redirect within itself (16.2 keeps the
// production whitelist to the three GitHub hosts).
func (c *GithubClient) isBaseHost(host string) bool {
	host = strings.ToLower(host)
	for _, base := range []string{c.APIBase, c.CodeloadBase} {
		parsed, err := url.Parse(base)
		if err == nil && strings.ToLower(parsed.Hostname()) == host {
			return true
		}
	}
	return false
}

// Resolve finds the head commit and the latest commit touching ref.Path
// (16.2). A missing ref means the repository's default branch.
func (c *GithubClient) Resolve(ctx context.Context, ref GithubRef) (ResolvedRef, *contract.Failure) {
	resolved := ResolvedRef{Ref: ref.Ref}
	if resolved.Ref == "" {
		body, f := c.getJSON(ctx, c.APIBase+"/repos/"+url.PathEscape(ref.Owner)+"/"+url.PathEscape(ref.Repo))
		if f != nil {
			return resolved, f
		}
		var repo struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := json.Unmarshal(body, &repo); err != nil || repo.DefaultBranch == "" {
			return resolved, contract.Fail(contract.CodeNetworkFailure, "GitHub 返回的内容无法解析")
		}
		resolved.Ref = repo.DefaultBranch
	}

	head, f := c.getSHA(ctx, c.APIBase+"/repos/"+url.PathEscape(ref.Owner)+"/"+url.PathEscape(ref.Repo)+"/commits/"+url.PathEscape(resolved.Ref))
	if f != nil {
		return resolved, f
	}
	resolved.Commit = head
	resolved.PathCommit = head
	if ref.Path == "" {
		return resolved, nil
	}

	query := url.Values{}
	query.Set("sha", resolved.Ref)
	query.Set("path", ref.Path)
	query.Set("per_page", "1")
	body, f := c.getJSON(ctx, c.APIBase+"/repos/"+url.PathEscape(ref.Owner)+"/"+url.PathEscape(ref.Repo)+"/commits?"+query.Encode())
	if f != nil {
		return resolved, f
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commits); err != nil {
		return resolved, contract.Fail(contract.CodeNetworkFailure, "GitHub 返回的内容无法解析")
	}
	if len(commits) > 0 && isHexSHA(commits[0].SHA) {
		resolved.PathCommit = commits[0].SHA
	}
	return resolved, nil
}

// DownloadTarball streams the archive pinned to commit into dst. The size cap
// comes from Q-SKM-5; the caller turns dst into the staging area.
func (c *GithubClient) DownloadTarball(ctx context.Context, ref GithubRef, commit string, dst io.Writer) (int64, *contract.Failure) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	target := c.CodeloadBase + "/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repo) + "/tar.gz/" + url.PathEscape(commit)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, contract.Fail(contract.CodeInvalidInput, "下载地址无法构造")
	}
	request.Header.Set("User-Agent", c.UserAgent)
	response, f := c.do(request)
	if f != nil {
		return 0, f
	}
	defer response.Body.Close()
	limit := c.MaxDownload
	if limit <= 0 {
		limit = maxDownloadBytes
	}
	written, err := io.Copy(dst, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return written, contract.Fail(contract.CodeNetworkFailure, "下载中断")
	}
	if written > limit {
		return written, contract.Fail(contract.CodeTooLarge, "下载包超过 200 MiB").WithLimit(written, limit)
	}
	return written, nil
}

func (c *GithubClient) getJSON(ctx context.Context, target string) ([]byte, *contract.Failure) {
	ctx, cancel := context.WithTimeout(ctx, c.APITimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, contract.Fail(contract.CodeInvalidInput, "地址无法构造")
	}
	request.Header.Set("User-Agent", c.UserAgent)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, f := c.do(request)
	if f != nil {
		return nil, f
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, contract.Fail(contract.CodeNetworkFailure, "读取 GitHub 响应失败")
	}
	return body, nil
}

func (c *GithubClient) getSHA(ctx context.Context, target string) (string, *contract.Failure) {
	ctx, cancel := context.WithTimeout(ctx, c.APITimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", contract.Fail(contract.CodeInvalidInput, "地址无法构造")
	}
	request.Header.Set("User-Agent", c.UserAgent)
	request.Header.Set("Accept", "application/vnd.github.sha")
	response, f := c.do(request)
	if f != nil {
		return "", f
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return "", contract.Fail(contract.CodeNetworkFailure, "读取 GitHub 响应失败")
	}
	sha := strings.Trim(strings.TrimSpace(string(body)), `"`)
	if !isHexSHA(sha) {
		return "", contract.Fail(contract.CodeNetworkFailure, "GitHub 返回的提交号无效")
	}
	return sha, nil
}

// do sends the request and maps GitHub's answers to our error codes (API
// §13.3). Network failures are retried once; rate limits and 404 are not.
func (c *GithubClient) do(request *http.Request) (*http.Response, *contract.Failure) {
	var lastFailure *contract.Failure
	for attempt := 0; attempt < 2; attempt++ {
		clone := request.Clone(request.Context())
		if attempt > 0 {
			clone.Body = nil
		}
		response, err := c.HTTP.Do(clone)
		if err != nil {
			lastFailure = contract.Fail(contract.CodeNetworkFailure, "连不上 GitHub，检查网络或代理").WithDetail(err.Error())
			continue
		}
		if f := c.rateLimitFailure(response); f != nil {
			response.Body.Close()
			return nil, f
		}
		switch {
		case response.StatusCode == http.StatusNotFound:
			response.Body.Close()
			return nil, contract.Fail(contract.CodeNotFound, "仓库或路径不存在；私有仓库在未登录时也显示为不存在")
		case response.StatusCode >= 200 && response.StatusCode < 300:
			return response, nil
		default:
			response.Body.Close()
			lastFailure = contract.Fail(contract.CodeNetworkFailure, "GitHub 返回了错误").WithDetail("HTTP " + strconv.Itoa(response.StatusCode))
			if response.StatusCode < 500 {
				return nil, lastFailure // 4xx (other than 404) is not retried
			}
		}
	}
	if lastFailure == nil {
		lastFailure = contract.Fail(contract.CodeNetworkFailure, "连不上 GitHub")
	}
	return nil, lastFailure
}

// rateLimitFailure reports GitHub's exhausted anonymous quota (16.2) and
// remembers the reset time for the update check's bottom bar.
func (c *GithubClient) rateLimitFailure(response *http.Response) *contract.Failure {
	if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusTooManyRequests {
		return nil
	}
	remaining := response.Header.Get("x-ratelimit-remaining")
	if remaining != "0" {
		return nil
	}
	failure := contract.Fail(contract.CodeRateLimited, "GitHub 匿名额度已用完")
	if reset := response.Header.Get("x-ratelimit-reset"); reset != "" {
		if epoch, err := strconv.ParseInt(reset, 10, 64); err == nil {
			c.rateLimitReset = time.Unix(epoch, 0)
			failure.Detail = "额度重置时间：" + c.rateLimitReset.Local().Format("2006-01-02 15:04")
		}
	}
	return failure
}

// RateLimitResetAt is the reset time reported by the last rate-limited answer;
// zero when the quota has not run out (M93).
func (c *GithubClient) RateLimitResetAt() time.Time {
	return c.rateLimitReset
}

// LatestPathCommit is the one request an update check needs per group (16.2):
// the ref head for a repository root, or the latest commit touching the
// skill's sub-path.
func (c *GithubClient) LatestPathCommit(ctx context.Context, ref GithubRef) (string, *contract.Failure) {
	if ref.Ref == "" {
		return "", contract.Fail(contract.CodeInvalidInput, "来源记录缺少分支")
	}
	base := c.APIBase + "/repos/" + url.PathEscape(ref.Owner) + "/" + url.PathEscape(ref.Repo)
	if ref.Path == "" {
		return c.getSHA(ctx, base+"/commits/"+url.PathEscape(ref.Ref))
	}
	query := url.Values{}
	query.Set("sha", ref.Ref)
	query.Set("path", ref.Path)
	query.Set("per_page", "1")
	body, f := c.getJSON(ctx, base+"/commits?"+query.Encode())
	if f != nil {
		return "", f
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &commits); err != nil {
		return "", contract.Fail(contract.CodeNetworkFailure, "GitHub 返回的内容无法解析")
	}
	if len(commits) == 0 {
		return "", contract.Fail(contract.CodeNotFound, "路径在远端不存在")
	}
	if !isHexSHA(commits[0].SHA) {
		return "", contract.Fail(contract.CodeNetworkFailure, "GitHub 返回的提交号无效")
	}
	return commits[0].SHA, nil
}

func isHexSHA(value string) bool {
	if len(value) != shaLength {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
