package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// tokenFixtureCase is one row of the cross-language parity fixture. The Go
// index is the authority: this test owns the file, and the renderer test
// (src/lib/tokenParity.test.ts) reads the same rows and asserts its own
// tokenizer produces identical output.
type tokenFixtureCase struct {
	Input  string   `json:"input"`
	Tokens []string `json:"tokens"`
}

// tokenFixtureInputs are the strings the two implementations must agree on.
// They cover every branch of the tokenizer plus the boundary shapes the legacy
// renderer can meet in a search box.
var tokenFixtureInputs = []string{
	"Hello World",
	"deepseek v4.1 release",
	"getUserName",
	"知识库管理",
	"中",
	"React 状态管理 best practice",
	"alpha, beta; gamma\n\tdelta",
	"知识库。管理",
	"",
	"!!! ---",
	"test test test",
	"knowledge-base",
	"x-ray",
	"v4.1-notes.md",
	"C#",
	"c++",
	"a_b_c",
	"a..b",
	"中文English混排",
	"emoji😀测试",
	"全角ＡＢＣ１２３",
	"  leading and trailing  ",
	"tab\tseparated\twords",
	"ＳＫＩＬＬ．ｍｄ",
}

func TestTokenFixtures(t *testing.T) {
	path := filepath.Join("testdata", "tokens.json")

	if os.Getenv("UPDATE_TOKEN_FIXTURES") == "1" {
		cases := make([]tokenFixtureCase, 0, len(tokenFixtureInputs))
		for _, input := range tokenFixtureInputs {
			tokens := Tokenize(input)
			if tokens == nil {
				tokens = []string{}
			}
			cases = append(cases, tokenFixtureCase{Input: input, Tokens: tokens})
		}
		body, err := json.MarshalIndent(cases, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d fixture rows", len(cases))
		return
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture missing (run with UPDATE_TOKEN_FIXTURES=1): %v", err)
	}
	var cases []tokenFixtureCase
	if err := json.Unmarshal(body, &cases); err != nil {
		t.Fatalf("fixture is not valid JSON: %v", err)
	}
	if len(cases) != len(tokenFixtureInputs) {
		t.Fatalf("fixture has %d rows, inputs list has %d (regenerate with UPDATE_TOKEN_FIXTURES=1)",
			len(cases), len(tokenFixtureInputs))
	}
	for i, c := range cases {
		if c.Input != tokenFixtureInputs[i] {
			t.Fatalf("row %d input drift: fixture %q, list %q", i, c.Input, tokenFixtureInputs[i])
		}
		want := Tokenize(c.Input)
		if want == nil {
			want = []string{}
		}
		if !reflect.DeepEqual(want, c.Tokens) {
			t.Errorf("row %d input %q: got %v, fixture %v", i, c.Input, want, c.Tokens)
		}
	}
}
