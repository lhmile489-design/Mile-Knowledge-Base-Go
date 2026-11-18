package main

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// sample.go builds the "large" sample knowledge base under a directory the
// caller chooses:
//
//	<root>/                 vault root (the watched directory)
//	  notes/...             10000 .md files over ~300 dirs, depth up to 6,
//	                        some directory names in Chinese
//	  code-repo/            ~5000 files: src/ ~3000, node_modules/ ~1000,
//	                        .git/ ~1000  (these two subtrees must be ignored)
//	  .mile-kb/             app data dir inside the vault
//
// File contents are small; what matters for watching is entry count and
// directory shape, not bytes.

type sampleStats struct {
	MDFiles      int
	Dirs         int
	RepoFiles    int
	IgnoredFiles int // files under node_modules/.git
	TotalFiles   int
	TotalDirs    int
	Root         string
}

var cnNames = []string{"笔记", "项目", "会议", "阅读", "想法", "归档", "每日", "草稿", "索引", "主题"}

var enNames = []string{"dev", "work", "ref", "log", "inbox", "area", "res", "journal", "wiki", "misc"}

// buildSample creates the whole tree. rnd seeds make the layout
// reproducible across runs: same root arg -> same layout.
func buildSample(root string, rnd *rand.Rand) (sampleStats, error) {
	var st sampleStats
	st.Root = root

	// ~300 directories, depth <= 6, names mixed Chinese/English.
	var dirs []string
	dirs = append(dirs, filepath.Join(root, "notes"))
	target := 300
	for len(dirs) < target {
		parent := dirs[rnd.Intn(len(dirs))]
		rel, _ := filepath.Rel(root, parent)
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if depth >= 6 {
			continue
		}
		var name string
		if rnd.Intn(3) == 0 {
			name = cnNames[rnd.Intn(len(cnNames))]
		} else {
			name = enNames[rnd.Intn(len(enNames))]
		}
		name = fmt.Sprintf("%s-%03d", name, len(dirs))
		dirs = append(dirs, filepath.Join(parent, name))
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return st, err
		}
	}
	st.Dirs = len(dirs)

	// 10000 md files spread over the dirs.
	body := "# title\n\nsome prose 一些中文内容。\n\n[[link]]\n"
	for i := 0; i < 10000; i++ {
		d := dirs[rnd.Intn(len(dirs))]
		p := filepath.Join(d, fmt.Sprintf("doc-%05d.md", i))
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return st, err
		}
	}
	st.MDFiles = 10000

	// code-repo/: ~3000 src files + node_modules ~1000 + .git ~1000.
	repo := filepath.Join(root, "code-repo")
	sub := []string{"src", "src/lib", "src/lib/deep", "docs", "scripts", "vendor/x"}
	for _, s := range sub {
		if err := os.MkdirAll(filepath.Join(repo, s), 0o755); err != nil {
			return st, err
		}
	}
	repoFiles := 0
	for i := 0; i < 3000; i++ {
		s := sub[rnd.Intn(len(sub))]
		ext := []string{".ts", ".js", ".json", ".md", ".css"}[rnd.Intn(5)]
		p := filepath.Join(repo, s, fmt.Sprintf("f-%05d%s", i, ext))
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			return st, err
		}
		repoFiles++
	}
	ignored := 0
	nm := filepath.Join(repo, "node_modules")
	for i := 0; i < 20; i++ {
		d := filepath.Join(nm, fmt.Sprintf("pkg-%02d/lib", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			return st, err
		}
	}
	for i := 0; i < 1000; i++ {
		d := filepath.Join(nm, fmt.Sprintf("pkg-%02d/lib", i%20))
		p := filepath.Join(d, fmt.Sprintf("m-%05d.js", i))
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			return st, err
		}
		ignored++
	}
	git := filepath.Join(repo, ".git")
	for _, s := range []string{"objects/ab", "objects/cd", "refs/heads", "refs/tags", "hooks"} {
		if err := os.MkdirAll(filepath.Join(git, s), 0o755); err != nil {
			return st, err
		}
	}
	for i := 0; i < 1000; i++ {
		var p string
		if i%2 == 0 {
			p = filepath.Join(git, "objects/ab", fmt.Sprintf("o-%05d", i))
		} else {
			p = filepath.Join(git, "objects/cd", fmt.Sprintf("o-%05d", i))
		}
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			return st, err
		}
		ignored++
	}
	st.RepoFiles = repoFiles
	st.IgnoredFiles = ignored

	// .mile-kb/ app data inside the vault.
	mile := filepath.Join(root, ".mile-kb", "settings")
	if err := os.MkdirAll(mile, 0o755); err != nil {
		return st, err
	}
	if err := os.WriteFile(filepath.Join(mile, "categories.json"), []byte("{}"), 0o644); err != nil {
		return st, err
	}

	// Count totals.
	var files, td int
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			td++
		} else {
			files++
		}
		return nil
	})
	st.TotalFiles = files
	st.TotalDirs = td
	return st, nil
}
