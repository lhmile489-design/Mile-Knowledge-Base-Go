package skills

import (
	"net/url"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// MaxGithubInputLength caps the address box (API §13.3).
const MaxGithubInputLength = 300

// maxGithubPathSegments keeps a repository sub-path shallow enough to stay a
// skill directory while covering skills/<name> and one grouping level.
const maxGithubPathSegments = 8

// GithubRef is a parsed install address (PRD FR-SKM-SRC). Ref is a single
// segment: branch names that contain a slash are not representable in the
// /tree/ form (技术设计 16.9) and fall back to the default branch.
type GithubRef struct {
	Owner string
	Repo  string
	Ref   string
	Path  string // repository sub-path; "" means the repository root
}

// ParseGithubInput accepts the three documented forms and nothing else:
//   - https://github.com/<owner>/<repo>[.git][/tree/<ref>[/<path>]]
//   - <owner>/<repo>
//
// Other hosts, http, credentials, query strings and fragments are refused
// (API §13.3).
func ParseGithubInput(input string) (GithubRef, *contract.Failure) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "请填 GitHub 地址或 owner/repo")
	}
	if len(trimmed) > MaxGithubInputLength {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "地址过长")
	}
	if strings.Contains(trimmed, "://") || strings.HasPrefix(strings.ToLower(trimmed), "github.com/") {
		return parseGithubURL(trimmed)
	}
	return parseShorthand(trimmed)
}

func parseShorthand(input string) (GithubRef, *contract.Failure) {
	parts := strings.Split(input, "/")
	if len(parts) != 2 {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "只接受 owner/repo 或完整的 GitHub 地址")
	}
	owner, f := validateOwner(parts[0])
	if f != nil {
		return GithubRef{}, f
	}
	repo, f := validateRepo(parts[1])
	if f != nil {
		return GithubRef{}, f
	}
	return GithubRef{Owner: owner, Repo: repo}, nil
}

func parseGithubURL(input string) (GithubRef, *contract.Failure) {
	parsed, err := url.Parse(input)
	if err != nil {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "地址格式不正确")
	}
	if parsed.Scheme != "https" {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "只支持 https://github.com 地址")
	}
	if !strings.EqualFold(parsed.Host, "github.com") {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "只支持 github.com 上的仓库")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "地址不能带账号、查询串或锚点")
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 2 {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "地址里缺少仓库名")
	}
	owner, f := validateOwner(segments[0])
	if f != nil {
		return GithubRef{}, f
	}
	repo, f := validateRepo(segments[1])
	if f != nil {
		return GithubRef{}, f
	}
	ref := GithubRef{Owner: owner, Repo: repo}
	if len(segments) == 2 {
		return ref, nil
	}
	if segments[2] != "tree" || len(segments) < 4 {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "只支持仓库地址或 /tree/<分支>/<子目录>")
	}
	if !IsValidSegment(segments[3]) {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "分支名不合法（分支名不能含斜杠）")
	}
	ref.Ref = segments[3]
	rest := segments[4:]
	if len(rest) == 0 {
		return ref, nil
	}
	if len(rest) > maxGithubPathSegments {
		return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "子目录层数过多")
	}
	for _, segment := range rest {
		if !IsValidSegment(segment) {
			return GithubRef{}, contract.Fail(contract.CodeInvalidInput, "子目录名不合法")
		}
	}
	ref.Path = strings.Join(rest, "/")
	return ref, nil
}

func validateOwner(value string) (string, *contract.Failure) {
	if value == "" || len(value) > 100 {
		return "", contract.Fail(contract.CodeInvalidInput, "owner 不合法")
	}
	for _, r := range value {
		if !isGithubNameRune(r) {
			return "", contract.Fail(contract.CodeInvalidInput, "owner 含不支持的字符")
		}
	}
	if value == "." || value == ".." {
		return "", contract.Fail(contract.CodeInvalidInput, "owner 不合法")
	}
	return value, nil
}

func validateRepo(value string) (string, *contract.Failure) {
	value = strings.TrimSuffix(value, ".git")
	if value == "" || len(value) > 100 {
		return "", contract.Fail(contract.CodeInvalidInput, "仓库名不合法")
	}
	for _, r := range value {
		if !isGithubNameRune(r) {
			return "", contract.Fail(contract.CodeInvalidInput, "仓库名含不支持的字符")
		}
	}
	if value == "." || value == ".." {
		return "", contract.Fail(contract.CodeInvalidInput, "仓库名不合法")
	}
	return value, nil
}

func isGithubNameRune(r rune) bool {
	return r == '-' || r == '_' || r == '.' ||
		(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
