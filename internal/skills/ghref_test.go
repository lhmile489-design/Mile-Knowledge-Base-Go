package skills

import (
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func TestParseGithubInput(t *testing.T) {
	valid := []struct {
		input string
		want  GithubRef
	}{
		{"anthropics/skills", GithubRef{Owner: "anthropics", Repo: "skills"}},
		{"https://github.com/anthropics/skills", GithubRef{Owner: "anthropics", Repo: "skills"}},
		{"https://github.com/anthropics/skills.git", GithubRef{Owner: "anthropics", Repo: "skills"}},
		{"https://github.com/anthropics/skills/tree/main/skills/pdf", GithubRef{Owner: "anthropics", Repo: "skills", Ref: "main", Path: "skills/pdf"}},
		{"https://github.com/anthropics/skills/tree/v1.0.0", GithubRef{Owner: "anthropics", Repo: "skills", Ref: "v1.0.0"}},
		{"  https://github.com/o/r/tree/main/a/b/c  ", GithubRef{Owner: "o", Repo: "r", Ref: "main", Path: "a/b/c"}},
		{"https://github.com/o/r/", GithubRef{Owner: "o", Repo: "r"}},
	}
	for _, item := range valid {
		got, f := ParseGithubInput(item.input)
		if f != nil {
			t.Errorf("%q: unexpected failure %+v", item.input, f)
			continue
		}
		if got != item.want {
			t.Errorf("%q = %+v, want %+v", item.input, got, item.want)
		}
	}

	invalid := []string{
		"",
		"   ",
		"http://github.com/o/r",
		"https://gitlab.com/o/r",
		"https://www.github.com/o/r",
		"https://github.com/o",
		"https://github.com/o/r?x=1",
		"https://github.com/o/r#readme",
		"https://oauth2@github.com/o/r",
		"https://github.com/o/r/blob/main/x",
		"https://github.com/o/r/tree/main/../evil",
		"https://github.com/o/r/tree/main/.hidden",
		"https://github.com/o/r/tree/main/" + longPath(),
		"o/r/extra",
		"o",
		"o//r",
		"../r",
		string(make([]byte, MaxGithubInputLength+1)),
	}
	for _, input := range invalid {
		if _, f := ParseGithubInput(input); f == nil || f.Code != contract.CodeInvalidInput {
			t.Errorf("%q: expected invalid-input, got %+v", input, f)
		}
	}
}

func longPath() string {
	path := ""
	for i := 0; i < maxGithubPathSegments+1; i++ {
		if path != "" {
			path += "/"
		}
		path += "d"
	}
	return path
}
