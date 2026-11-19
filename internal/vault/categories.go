package vault

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// Categories live in <vault>/.mile-kb/settings/categories.json as a top-level
// array (docs/数据库设计.md §3.2). Invalid rows are dropped, never reported:
// the file is user-editable and a bad row must not brick the settings page.

const (
	// MaxCategories caps the table (legacy MAX_CATEGORIES).
	MaxCategories = 200
	// MaxCategoryAliasLength caps one display alias (legacy MAX_ALIAS_LENGTH).
	MaxCategoryAliasLength = 64
	// maxCategoryDirectoryLength caps the directory name (legacy MAX_NAME_LENGTH).
	maxCategoryDirectoryLength = 64
)

// Category is one categories.json entry.
type Category struct {
	Directory string
	Alias     string
	Area      *string
	Indexed   bool
	Order     int
}

// CategoriesPath is the file location for a vault root.
func CategoriesPath(root string) string {
	return filepath.Join(root, ".mile-kb", "settings", "categories.json")
}

// DefaultIndexedFor is the indexed value a directory has with no config: dot
// directories classify as system and stay out of search, everything else is in.
func DefaultIndexedFor(directory string) bool {
	return AreaFor(directory) != AreaSystem
}

// IsMeaningfulCategory reports whether an entry differs from the defaults. The
// writer drops the rest so a fresh directory never adds noise to the file.
func IsMeaningfulCategory(category Category) bool {
	return category.Alias != "" || category.Area != nil || category.Indexed != DefaultIndexedFor(category.Directory)
}

// ParseCategories decodes the stored array, tolerating wrong types per row.
func ParseCategories(raw []byte) []Category {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return []Category{}
	}
	out := make([]Category, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		var fields struct {
			Directory json.RawMessage `json:"directory"`
			Alias     json.RawMessage `json:"alias"`
			Area      json.RawMessage `json:"area"`
			Indexed   json.RawMessage `json:"indexed"`
			Order     json.RawMessage `json:"order"`
		}
		if json.Unmarshal(item, &fields) != nil {
			continue
		}
		var directory string
		if json.Unmarshal(fields.Directory, &directory) != nil {
			continue
		}
		directory = strings.TrimSpace(directory)
		key := strings.ToLower(directory)
		if !validCategoryDirectory(directory) || seen[key] {
			continue
		}
		seen[key] = true

		var alias string
		_ = json.Unmarshal(fields.Alias, &alias)
		indexed, hasIndexed := parseCategoryBool(fields.Indexed)
		if !hasIndexed {
			indexed = DefaultIndexedFor(directory)
		}
		out = append(out, Category{
			Directory: directory,
			Alias:     truncateAlias(alias),
			Area:      parseCategoryArea(fields.Area),
			Indexed:   indexed,
			Order:     parseCategoryOrder(fields.Order),
		})
		if len(out) >= MaxCategories {
			break
		}
	}
	return out
}

// NormalizeCategories applies the same rules to already typed input (M65).
func NormalizeCategories(list []Category) []Category {
	out := make([]Category, 0, len(list))
	seen := map[string]bool{}
	for _, item := range list {
		directory := strings.TrimSpace(item.Directory)
		key := strings.ToLower(directory)
		if !validCategoryDirectory(directory) || seen[key] {
			continue
		}
		seen[key] = true
		area := item.Area
		if area != nil && !IsAreaID(*area) {
			area = nil
		}
		out = append(out, Category{
			Directory: directory,
			Alias:     truncateAlias(item.Alias),
			Area:      area,
			Indexed:   item.Indexed,
			Order:     item.Order,
		})
		if len(out) >= MaxCategories {
			break
		}
	}
	return out
}

// ReadCategories loads the file; a missing or damaged file yields an empty
// table so the settings page still opens.
func ReadCategories(root string) []Category {
	raw, err := os.ReadFile(CategoriesPath(root))
	if err != nil {
		return []Category{}
	}
	return ParseCategories(raw)
}

// WriteCategories normalizes the list, drops entries equal to the defaults and
// writes the legacy bytes (two-space indent, no HTML escaping, trailing
// newline) atomically.
func WriteCategories(root string, list []Category) error {
	kept := make([]Category, 0, len(list))
	for _, item := range NormalizeCategories(list) {
		if IsMeaningfulCategory(item) {
			kept = append(kept, item)
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(toCategoryPayload(kept)); err != nil {
		return err
	}
	target := CategoriesPath(root)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// categoryJSON fixes the field order the legacy writer produced.
type categoryJSON struct {
	Directory string  `json:"directory"`
	Alias     string  `json:"alias"`
	Area      *string `json:"area"`
	Indexed   bool    `json:"indexed"`
	Order     int     `json:"order"`
}

func toCategoryPayload(list []Category) []categoryJSON {
	out := make([]categoryJSON, 0, len(list))
	for _, item := range list {
		out = append(out, categoryJSON{
			Directory: item.Directory,
			Alias:     item.Alias,
			Area:      item.Area,
			Indexed:   item.Indexed,
			Order:     item.Order,
		})
	}
	return out
}

// IsAreaID reports whether a value is one of the known area ids.
func IsAreaID(value string) bool {
	for _, area := range AreaOrder {
		if area == value {
			return true
		}
	}
	return false
}

func validCategoryDirectory(value string) bool {
	if value == "" || len([]rune(value)) > maxCategoryDirectoryLength {
		return false
	}
	if value == "." || value == ".." {
		return false
	}
	return !strings.ContainsAny(value, "\\/:*?\"<>|")
}

func truncateAlias(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > MaxCategoryAliasLength {
		runes = runes[:MaxCategoryAliasLength]
	}
	return string(runes)
}

func parseCategoryArea(raw json.RawMessage) *string {
	var value string
	if json.Unmarshal(raw, &value) != nil || !IsAreaID(value) {
		return nil
	}
	return &value
}

func parseCategoryBool(raw json.RawMessage) (bool, bool) {
	var value bool
	if json.Unmarshal(raw, &value) != nil {
		return false, false
	}
	return value, true
}

// parseCategoryOrder mirrors Math.round: non-finite or non-numeric values
// become 0, halves round up (toward +Infinity).
func parseCategoryOrder(raw json.RawMessage) int {
	var value float64
	if json.Unmarshal(raw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return int(math.Floor(value + 0.5))
}
