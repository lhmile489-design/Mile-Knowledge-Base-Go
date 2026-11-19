package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ported from the legacy shared/appConfig.ts normalizeCategoryConfigs cases
// (appConfig.test.ts). Invalid rows are dropped, never reported.

func TestParseCategoriesNormalizesLikeLegacy(t *testing.T) {
	raw := []byte(`[
		{"directory": " projects ", "alias": ""},
		{"directory": "PROJECTS", "alias": "重复"},
		{"directory": ".obsidian"},
		{"directory": "tools", "area": "bogus", "order": 2.5},
		{"directory": "raws", "alias": "  暂存  ", "area": "raws", "indexed": false, "order": 7},
		{"directory": "a/b"},
		{"directory": ".."},
		{"directory": ""},
		{"directory": 42},
		"not an object",
		null,
		{"directory": "wiki", "alias": "` + strings.Repeat("长", 100) + `", "order": "3"}
	]`)
	got := ParseCategories(raw)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5: %+v", len(got), got)
	}
	if got[0].Directory != "projects" || got[0].Alias != "" || got[0].Area != nil || !got[0].Indexed {
		t.Errorf("projects = %+v, want defaults (indexed true)", got[0])
	}
	if got[1].Directory != ".obsidian" || got[1].Indexed {
		t.Errorf(".obsidian = %+v, want indexed false by default", got[1])
	}
	if got[2].Directory != "tools" || got[2].Area != nil || got[2].Order != 3 {
		t.Errorf("tools = %+v, want area null and order rounded to 3", got[2])
	}
	if got[3].Directory != "raws" || got[3].Alias != "暂存" || got[3].Area == nil || *got[3].Area != "raws" || got[3].Indexed || got[3].Order != 7 {
		t.Errorf("raws = %+v", got[3])
	}
}

func TestParseCategoriesTrimsAndCapsAlias(t *testing.T) {
	got := ParseCategories([]byte(`[{"directory":"wiki","alias":"` + strings.Repeat("长", 100) + `"}]`))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if n := len([]rune(got[0].Alias)); n != MaxCategoryAliasLength {
		t.Fatalf("alias runes = %d, want %d", n, MaxCategoryAliasLength)
	}
}

func TestParseCategoriesCapsAt200(t *testing.T) {
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < 205; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"directory":"dir`)
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(string(rune('a' + i/26)))
		b.WriteString(`"}`)
	}
	b.WriteByte(']')
	got := ParseCategories([]byte(b.String()))
	if len(got) != MaxCategories {
		t.Fatalf("len = %d, want %d", len(got), MaxCategories)
	}
}

func TestParseCategoriesRejectsNonArray(t *testing.T) {
	for _, raw := range [][]byte{
		[]byte(`{"directory":"projects"}`),
		[]byte(`"projects"`),
		[]byte(`[`),
		{},
	} {
		if got := ParseCategories(raw); len(got) != 0 {
			t.Errorf("%q => %+v, want empty", raw, got)
		}
	}
}

func TestNormalizeCategoriesTypedInput(t *testing.T) {
	area := "bogus"
	got := NormalizeCategories([]Category{
		{Directory: "  projects  ", Alias: " 项目 "},
		{Directory: "projects", Alias: "dup"},
		{Directory: "a\\b"},
		{Directory: "tools", Area: &area, Order: -2},
		{Directory: ".git"},
	})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %+v", len(got), got)
	}
	// 类型化入参（M65）里的 indexed 就是调用方的真实值：前端总会带上完整行，
	// 零值 false 表示「不进检索」，不再按目录补默认。
	if got[0].Directory != "projects" || got[0].Alias != "项目" || got[0].Indexed {
		t.Errorf("projects = %+v", got[0])
	}
	if got[1].Directory != "tools" || got[1].Area != nil || got[1].Order != -2 {
		t.Errorf("tools = %+v, want area null and order kept", got[1])
	}
	if got[2].Directory != ".git" || got[2].Indexed {
		t.Errorf(".git = %+v", got[2])
	}
}

func TestDefaultIndexedForAndMeaningful(t *testing.T) {
	if !DefaultIndexedFor("projects") {
		t.Error("projects should be indexed by default")
	}
	if DefaultIndexedFor(".mile-kb") || DefaultIndexedFor(".obsidian") {
		t.Error("dot directories are excluded by default")
	}
	cases := []struct {
		category Category
		want     bool
	}{
		{Category{Directory: "projects", Indexed: true}, false},
		{Category{Directory: "projects", Indexed: false}, true},
		{Category{Directory: "projects", Alias: "项目", Indexed: true}, true},
		{Category{Directory: ".mile-kb", Indexed: false}, false},
		{Category{Directory: ".mile-kb", Indexed: true}, true},
	}
	for _, item := range cases {
		if got := IsMeaningfulCategory(item.category); got != item.want {
			t.Errorf("%+v => %v, want %v", item.category, got, item.want)
		}
	}
}

func TestReadCategoriesMissingOrCorruptIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if got := ReadCategories(dir); len(got) != 0 {
		t.Fatalf("missing => %+v", got)
	}
	path := CategoriesPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"broken":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ReadCategories(dir); len(got) != 0 {
		t.Fatalf("corrupt => %+v", got)
	}
}

func TestWriteCategoriesFiltersDefaultsAndBytes(t *testing.T) {
	dir := t.TempDir()
	area := "raws"
	list := []Category{
		{Directory: "wiki", Alias: "", Area: nil, Indexed: true},             // 与默认无异，不落盘
		{Directory: "projects", Alias: "项目", Area: nil, Indexed: true},       // 有别名，落盘
		{Directory: "raws", Alias: "", Area: &area, Indexed: true, Order: 1}, // 有 area 覆盖，落盘
	}
	if err := WriteCategories(dir, list); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(CategoriesPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	want := `[
  {
    "directory": "projects",
    "alias": "项目",
    "area": null,
    "indexed": true,
    "order": 0
  },
  {
    "directory": "raws",
    "alias": "",
    "area": "raws",
    "indexed": true,
    "order": 1
  }
]
`
	if string(body) != want {
		t.Fatalf("bytes = %q\nwant %q", body, want)
	}

	if err := WriteCategories(dir, nil); err != nil {
		t.Fatal(err)
	}
	body, _ = os.ReadFile(CategoriesPath(dir))
	if string(body) != "[]\n" {
		t.Fatalf("empty bytes = %q", body)
	}
}

func TestCategoriesByteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := CategoriesPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `[
  {
    "directory": "skills",
    "alias": "技能库",
    "area": "skills",
    "indexed": true,
    "order": 0
  },
  {
    "directory": "projects",
    "alias": "",
    "area": null,
    "indexed": false,
    "order": 2
  }
]
`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteCategories(dir, ReadCategories(dir)); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != seed {
		t.Fatalf("round trip changed bytes:\n%q\nwant\n%q", body, seed)
	}
}
