package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func writeAliasFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aliases.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAliasesEmptyFileBytes(t *testing.T) {
	file := ReadAliases(filepath.Join(t.TempDir(), "missing.json"))
	if got := string(file.Bytes()); got != "{}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestAliasesRoundTripKeepsOrderAndBytes(t *testing.T) {
	original := "{\n  \"raws\": \"暂存\",\n  \"skills\": \"技能\"\n}\n"
	file := ReadAliases(writeAliasFile(t, original))
	if got := string(file.Bytes()); got != original {
		t.Fatalf("bytes changed:\n%q\n%q", got, original)
	}
}

func TestAliasesSetAppendDelete(t *testing.T) {
	path := writeAliasFile(t, "{\n  \"raws\": \"暂存\"\n}\n")
	file := ReadAliases(path)
	alias := "项目区"
	if f := file.Set("projects", &alias); f != nil {
		t.Fatal(f)
	}
	got := string(file.Bytes())
	if !strings.Contains(got, "\"projects\": \"项目区\"") || !strings.Contains(got, "\"raws\": \"暂存\"") {
		t.Fatalf("bytes = %s", got)
	}
	if !strings.HasSuffix(got, "\n") {
		t.Error("missing trailing newline")
	}
	// An empty value deletes the entry.
	empty := ""
	if f := file.Set("raws", &empty); f != nil {
		t.Fatal(f)
	}
	if strings.Contains(string(file.Bytes()), "raws") {
		t.Errorf("alias not removed: %s", file.Bytes())
	}
	if f := file.Set("projects", nil); f != nil {
		t.Fatal(f)
	}
	if got := string(file.Bytes()); got != "{}\n" {
		t.Fatalf("empty after delete: %q", got)
	}
}

func TestAliasesValidation(t *testing.T) {
	file := ReadAliases(filepath.Join(t.TempDir(), "a.json"))
	ok := "x"
	badKeys := []string{"", "  ", "/abs", "../x", "a/../b", `a\b`, "C:/x", "a//b"}
	for _, key := range badKeys {
		if f := file.Set(key, &ok); f == nil || f.Code != contract.CodeInvalidInput {
			t.Errorf("key %q accepted: %+v", key, f)
		}
	}
	long := strings.Repeat("字", MaxAliasLength+1)
	if f := file.Set("raws", &long); f == nil {
		t.Error("overlong alias accepted")
	}
}

func TestAliasesCap(t *testing.T) {
	file := ReadAliases(filepath.Join(t.TempDir(), "a.json"))
	value := "v"
	for i := 0; i < MaxAliasEntries; i++ {
		key := "dir-" + itoaT(i)
		if f := file.Set(key, &value); f != nil {
			t.Fatalf("entry %d: %+v", i, f)
		}
	}
	extra := "dir-extra"
	if f := file.Set(extra, &value); f == nil {
		t.Error("cap not enforced")
	}
}

func itoaT(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
