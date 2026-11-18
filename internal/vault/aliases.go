package vault

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Aliases live in <vault>/.mile-kb/settings/aliases.json as a top-level object
// of directory path -> alias (docs/数据库设计.md §3.3). The file is Git-tracked,
// so entries keep their original order and new keys are appended sorted.

// MaxAliasEntries caps the table (PRD FR-AREA).
const MaxAliasEntries = 2000

// MaxAliasLength caps one alias.
const MaxAliasLength = 64

// AliasFile is an ordered aliases table.
type AliasFile struct {
	path   string
	order  []string
	values map[string]string
}

// ReadAliases loads the file; a missing or damaged file yields an empty table
// (the tree keeps working, the user just loses display aliases).
func ReadAliases(path string) *AliasFile {
	file := &AliasFile{path: path, values: map[string]string{}}
	body, err := os.ReadFile(path)
	if err != nil {
		return file
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	token, err := decoder.Token()
	if err != nil {
		return file
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return file
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			break
		}
		key, ok := keyToken.(string)
		if !ok {
			break
		}
		var value any
		if err := decoder.Decode(&value); err != nil {
			break
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		if _, seen := file.values[key]; !seen {
			file.order = append(file.order, key)
		}
		file.values[key] = text
	}
	return file
}

// Map returns the current table.
func (f *AliasFile) Map() map[string]string {
	out := make(map[string]string, len(f.values))
	for key, value := range f.values {
		out[key] = value
	}
	return out
}

// Set applies one patch entry; a nil or empty value deletes the alias.
func (f *AliasFile) Set(key string, value *string) *contract.Failure {
	normKey, fail := normaliseAliasKey(key)
	if fail != nil {
		return fail
	}
	if value == nil || strings.TrimSpace(*value) == "" {
		if _, ok := f.values[normKey]; ok {
			delete(f.values, normKey)
			f.order = removeString(f.order, normKey)
		}
		return nil
	}
	text := strings.TrimSpace(*value)
	if len([]rune(text)) > MaxAliasLength {
		return contract.Fail(contract.CodeInvalidInput, "别名过长")
	}
	if _, ok := f.values[normKey]; !ok {
		if len(f.values) >= MaxAliasEntries {
			return contract.Fail(contract.CodeInvalidInput, "别名条目超过上限")
		}
		f.order = append(f.order, normKey)
	}
	f.values[normKey] = text
	return nil
}

// Bytes serializes with the legacy layout: two spaces, fixed entry order and a
// trailing newline, HTML characters left literal.
func (f *AliasFile) Bytes() []byte {
	var b strings.Builder
	if len(f.order) == 0 {
		return []byte("{}\n")
	}
	b.WriteString("{\n")
	for i, key := range f.order {
		value, ok := f.values[key]
		if !ok {
			continue
		}
		b.WriteString("  ")
		b.WriteString(quoteAlias(key))
		b.WriteString(": ")
		b.WriteString(quoteAlias(value))
		if i != len(f.order)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// Save writes the file atomically.
func (f *AliasFile) Save() error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, f.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}

// AliasPath is the file location for a vault root.
func AliasPath(root string) string {
	return filepath.Join(root, ".mile-kb", "settings", "aliases.json")
}

func normaliseAliasKey(key string) (string, *contract.Failure) {
	key = strings.TrimSpace(key)
	if key == "" || len([]rune(key)) > 1024 {
		return "", contract.Fail(contract.CodeInvalidInput, "别名目录不合法")
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return "", contract.Fail(contract.CodeInvalidInput, "别名目录必须是库内相对路径")
	}
	if len(key) >= 2 && key[1] == ':' {
		return "", contract.Fail(contract.CodeInvalidInput, "别名目录必须是库内相对路径")
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", contract.Fail(contract.CodeInvalidInput, "别名目录越界")
		}
	}
	return key, nil
}

func removeString(list []string, target string) []string {
	out := list[:0]
	for _, item := range list {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}

func quoteAlias(value string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return `""`
	}
	return strings.TrimSuffix(b.String(), "\n")
}
