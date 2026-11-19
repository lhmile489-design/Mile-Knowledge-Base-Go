// Spike R-9: directory junctions for R2 skill distribution (docs/技术设计.md 13.4).
// Everything happens under fresh temp directories; no real tool directory is
// touched. Run: go run . [-alt <dir on another volume>]
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var pass, fail int

func check(id, desc string, ok bool, detail string) {
	if ok {
		pass++
		fmt.Printf("PASS %-4s %s", id, desc)
	} else {
		fail++
		fmt.Printf("FAIL %-4s %s", id, desc)
	}
	if detail != "" {
		fmt.Printf(" :: %s", detail)
	}
	fmt.Println()
}

func info(id, msg string) { fmt.Printf("INFO %-4s %s\n", id, msg) }

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(path, body string) {
	must(os.MkdirAll(filepath.Dir(path), 0o755))
	must(os.WriteFile(path, []byte(body), 0o644))
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "<err " + err.Error() + ">"
	}
	return string(b)
}

// treeHash hashes a directory's regular files without following links.
func treeHash(dir string) string {
	files, _, err := safeWalk(dir)
	if err != nil {
		return "<err " + err.Error() + ">"
	}
	sort.Strings(files)
	h := sha256.New()
	for _, rel := range files {
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(rel))
		f, err := os.Open(filepath.Join(dir, rel))
		must(err)
		_, _ = io.Copy(h, f)
		f.Close()
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func copyTree(src, dst string) error {
	files, _, err := safeWalk(src)
	if err != nil {
		return err
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(src, rel))
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			return err
		}
	}
	return os.MkdirAll(dst, 0o755)
}

// distribute is the production shape: junction first, copy on failure.
func distribute(src, dst string, link func(string, string) error) (string, error) {
	if err := link(dst, src); err == nil {
		return "junction", nil
	} else if _, statErr := os.Lstat(dst); statErr == nil {
		return "", fmt.Errorf("link failed and left %s behind: %v", dst, err)
	}
	if err := copyTree(src, dst); err != nil {
		_ = os.RemoveAll(dst)
		return "", err
	}
	return "copy", nil
}

// copyState compares a copied distribution with its baseline, like FR-PROV.
func copyState(base, src, dst string) string {
	srcNow, dstNow := treeHash(src), treeHash(dst)
	switch {
	case srcNow == base && dstNow == base:
		return "same"
	case srcNow != base && dstNow == base:
		return "source-changed"
	case srcNow == base && dstNow != base:
		return "copy-changed"
	default:
		return "both-changed"
	}
}

func modeBits(path string) string {
	fi, err := os.Lstat(path)
	if err != nil {
		return "lstat err: " + err.Error()
	}
	m := fi.Mode()
	return fmt.Sprintf("mode=%v dir=%v symlink=%v irregular=%v", m, m.IsDir(), m&fs.ModeSymlink != 0, m&fs.ModeIrregular != 0)
}

func main() { os.Exit(run()) }

func run() int {
	alt := flag.String("alt", "", "a writable directory on another volume for the cross-volume case")
	flag.Parse()

	root, err := os.MkdirTemp("", "r9-junction-")
	must(err)
	defer func() {
		// Every link in here points inside root (or nowhere), so cleanup cannot
		// reach outside it even if a removal followed a link.
		_ = os.RemoveAll(root)
	}()
	kb := filepath.Join(root, "知识库")
	tool := filepath.Join(root, "工具", ".claude", "skills")
	backup := filepath.Join(root, "备份")
	skill := filepath.Join(kb, "skills", "前端", "vue技能")
	write(filepath.Join(skill, "SKILL.md"), "---\nname: vue技能\ndescription: 测试\n---\n正文\n")
	write(filepath.Join(skill, "refs", "a.md"), "参考\n")
	must(os.MkdirAll(tool, 0o755))
	must(os.MkdirAll(backup, 0o755))
	skillHash := treeHash(skill)
	info("env", fmt.Sprintf("go=%s GODEBUG=%q", goVersion(), os.Getenv("GODEBUG")))

	// C1 create via DeviceIoControl, with Chinese names.
	link := filepath.Join(tool, "vue技能")
	err = createJunction(link, skill, false)
	kind, target, cerr := classify(link)
	check("C1", "create junction via FSCTL_SET_REPARSE_POINT (Chinese path)", err == nil && cerr == nil && kind == KindJunction && strings.EqualFold(target, skill),
		fmt.Sprintf("err=%v kind=%s target-ok=%v", err, kind, strings.EqualFold(target, skill)))

	// C2 read and write through the junction.
	readThrough := read(filepath.Join(link, "SKILL.md")) == read(filepath.Join(skill, "SKILL.md"))
	write(filepath.Join(skill, "refs", "new.md"), "知识库里新加\n")
	seesNew := read(filepath.Join(link, "refs", "new.md")) == "知识库里新加\n"
	must(os.Remove(filepath.Join(skill, "refs", "new.md")))
	check("C2", "tool path reads knowledge-base content live", readThrough && seesNew, "")

	// C3 mklink /J gives the same reparse data.
	link2 := filepath.Join(root, "mklink-j")
	out, merr := exec.Command("cmd", "/c", "mklink", "/J", link2, skill).CombinedOutput()
	k2, t2, _ := classify(link2)
	check("C3", "mklink /J produces the same classification", merr == nil && k2 == KindJunction && strings.EqualFold(t2, skill),
		fmt.Sprintf("err=%v kind=%s out=%q", merr, k2, strings.TrimSpace(string(out))))
	if merr == nil {
		must(removeLink(link2))
	}

	// C4 what the Go standard library reports (informational).
	info("C4", "os.Lstat on junction: "+modeBits(link))
	rl, rlErr := os.Readlink(link)
	info("C4", fmt.Sprintf("os.Readlink on junction: ok=%v equal=%v", rlErr == nil, strings.EqualFold(stripNTPrefix(rl), skill)))
	ev, evErr := filepath.EvalSymlinks(link)
	info("C4", fmt.Sprintf("filepath.EvalSymlinks on junction: ok=%v equal=%v", evErr == nil, strings.EqualFold(ev, skill)))
	if fi, err := os.Stat(link); err == nil {
		info("C4", fmt.Sprintf("os.Stat follows junction: isDir=%v", fi.IsDir()))
	}

	// C5 directory symlink availability (needs Developer Mode or elevation).
	slink := filepath.Join(root, "symlink-dir")
	serr := os.Symlink(skill, slink)
	if serr != nil {
		info("C5", "os.Symlink(dir) unavailable here: "+serr.Error())
	} else {
		ks, ts, _ := classify(slink)
		check("C5", "directory symlink classified as symlink with target", ks == KindSymlink && strings.EqualFold(ts, skill), fmt.Sprintf("kind=%s", ks))
		info("C5", "os.Lstat on symlink: "+modeBits(slink))
		rel := filepath.Join(root, "symlink-rel")
		if err := os.Symlink(filepath.Join("知识库", "skills", "前端", "vue技能"), rel); err == nil {
			kr, tr, _ := classify(rel)
			check("C5r", "relative symlink target resolved against its directory", kr == KindSymlink && strings.EqualFold(tr, skill), "")
		}
	}

	// C6 removeLink deletes only the link.
	l6 := filepath.Join(root, "c6")
	must(createJunction(l6, skill, false))
	err = removeLink(l6)
	k6, _, _ := classify(l6)
	check("C6", "removeLink deletes the junction, target untouched", err == nil && k6 == KindMissing && treeHash(skill) == skillHash, "")
	check("C6b", "removeLink refuses a real directory", removeLink(skill) != nil && treeHash(skill) == skillHash, "")

	// C7 os.RemoveAll on the junction itself.
	l7 := filepath.Join(root, "c7")
	must(createJunction(l7, skill, false))
	err = os.RemoveAll(l7)
	k7, _, _ := classify(l7)
	check("C7", "os.RemoveAll(junction) removes only the link", err == nil && k7 == KindMissing && treeHash(skill) == skillHash, fmt.Sprintf("err=%v", err))

	// C8 os.RemoveAll on a parent that contains a junction (and a symlink when available).
	p8 := filepath.Join(root, "c8-parent")
	write(filepath.Join(p8, "own", "SKILL.md"), "自己的\n")
	must(createJunction(filepath.Join(p8, "linked"), skill, false))
	if serr == nil {
		must(os.Symlink(skill, filepath.Join(p8, "symlinked")))
	}
	err = os.RemoveAll(p8)
	_, p8err := os.Lstat(p8)
	check("C8", "os.RemoveAll(parent with links) never deletes link targets", err == nil && os.IsNotExist(p8err) && treeHash(skill) == skillHash,
		fmt.Sprintf("err=%v target-hash-ok=%v", err, treeHash(skill) == skillHash))

	// C9 filepath.WalkDir over a parent with a junction (informational) vs safeWalk.
	p9 := filepath.Join(root, "c9-parent")
	write(filepath.Join(p9, "own", "SKILL.md"), "自己的\n")
	must(createJunction(filepath.Join(p9, "linked"), skill, false))
	seen := 0
	_ = filepath.WalkDir(p9, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			seen++
		}
		return nil
	})
	info("C9", fmt.Sprintf("filepath.WalkDir saw %d non-dir entries (1 file + 1 link if it does not descend; 3+ if it does)", seen))
	files, skipped, werr := safeWalk(p9)
	check("C9", "safeWalk lists own files and skips the link", werr == nil && len(files) == 1 && len(skipped) == 1, fmt.Sprintf("files=%v skipped=%v", files, skipped))

	// C10 move a junction into the backup area (uninstall of a link is a delete,
	// but backup-by-rename must keep links as links).
	l10 := filepath.Join(tool, "c10")
	must(createJunction(l10, skill, false))
	moved := filepath.Join(backup, "c10")
	err = os.Rename(l10, moved)
	k10, t10, _ := classify(moved)
	check("C10", "os.Rename moves the junction itself", err == nil && k10 == KindJunction && strings.EqualFold(t10, skill) && treeHash(skill) == skillHash, fmt.Sprintf("err=%v", err))
	must(removeLink(moved))

	// C11 backing up a real skill directory that contains a junction.
	indep := filepath.Join(tool, "独立技能")
	write(filepath.Join(indep, "SKILL.md"), "独立\n")
	must(createJunction(filepath.Join(indep, "inner-link"), skill, false))
	bk := filepath.Join(backup, "独立技能")
	err = copyTree(indep, bk)
	bFiles, bSkipped, _ := safeWalk(bk)
	_, innerErr := os.Lstat(filepath.Join(bk, "inner-link"))
	check("C11", "backup copy excludes link contents", err == nil && len(bFiles) == 1 && len(bSkipped) == 0 && os.IsNotExist(innerErr), fmt.Sprintf("files=%v", bFiles))
	must(removeLink(filepath.Join(indep, "inner-link")))
	must(os.RemoveAll(indep))
	check("C11b", "removing the backed-up original leaves the link target intact", treeHash(skill) == skillHash, "")

	// C12 dangling junctions.
	ghost := filepath.Join(root, "不存在的目标")
	l12 := filepath.Join(root, "c12")
	err = createJunction(l12, ghost, false)
	k12, t12, _ := classify(l12)
	_, statErr := os.Stat(l12)
	check("C12", "junction to a missing target: kernel accepts it, Stat reports broken", err == nil && k12 == KindJunction && strings.EqualFold(t12, ghost) && statErr != nil, fmt.Sprintf("err=%v stat=%v", err, statErr))
	if err == nil {
		must(removeLink(l12))
	}
	tmpTarget := filepath.Join(root, "会被删的目标")
	write(filepath.Join(tmpTarget, "SKILL.md"), "x\n")
	l12b := filepath.Join(root, "c12b")
	must(createJunction(l12b, tmpTarget, false))
	must(os.RemoveAll(tmpTarget))
	k12b, _, _ := classify(l12b)
	_, stat12b := os.Stat(l12b)
	check("C12b", "target deleted later: still a junction, detected as broken", k12b == KindJunction && stat12b != nil, "")
	check("C12c", "broken junction can be removed", removeLink(l12b) == nil, "")

	// C13 cross-volume.
	if *alt == "" {
		info("C13", "skipped: pass -alt <dir on another volume>")
	} else {
		altRoot, err := os.MkdirTemp(*alt, "r9-junction-")
		if err != nil {
			info("C13", "skipped: cannot create temp under -alt: "+err.Error())
		} else {
			l13 := filepath.Join(altRoot, "跨盘")
			err := createJunction(l13, skill, false)
			ok := err == nil && read(filepath.Join(l13, "SKILL.md")) == read(filepath.Join(skill, "SKILL.md"))
			check("C13", "junction on another volume pointing back works", ok, fmt.Sprintf("vol=%s err=%v", filepath.VolumeName(altRoot), err))
			if err == nil {
				_ = removeLink(l13)
			}
			_ = os.RemoveAll(altRoot)
			check("C13b", "cleanup on the other volume left the target intact", treeHash(skill) == skillHash, "")
		}
	}

	// C14 network target: refused by our rule; what Windows does when forced.
	unc := `\\127.0.0.1\r9-no-such-share\skill`
	err = createJunction(filepath.Join(root, "c14"), unc, false)
	check("C14", "production rule refuses a UNC target before touching disk", errors.Is(err, errUNCTarget), "")
	l14 := filepath.Join(root, "c14-forced")
	ferr := createJunction(l14, unc, true)
	if ferr == nil {
		_, st := os.Stat(l14)
		info("C14", fmt.Sprintf("forced UNC junction: kernel accepted it; Stat through it: %v", st))
		_ = removeLink(l14)
	} else {
		info("C14", "forced UNC junction rejected by Windows: "+ferr.Error())
	}

	// C15 fallback to copy when the link cannot be made.
	dst := filepath.Join(tool, "回退拷贝")
	method, err := distribute(skill, dst, func(l, t string) error { return createJunction(l, unc, false) })
	kd, _, _ := classify(dst)
	check("C15", "link failure falls back to a real copy", err == nil && method == "copy" && kd == KindDir && treeHash(dst) == skillHash, fmt.Sprintf("method=%s err=%v", method, err))
	method2, err := distribute(skill, filepath.Join(tool, "正常联接"), func(l, t string) error { return createJunction(l, t, false) })
	check("C15b", "normal path uses a junction", err == nil && method2 == "junction", "")
	_ = removeLink(filepath.Join(tool, "正常联接"))
	check("C15c", "existing destination is never clobbered", createJunction(dst, skill, false) != nil && treeHash(dst) == skillHash, "")

	// C16 copy sync states.
	base := treeHash(skill)
	s0 := copyState(base, skill, dst)
	write(filepath.Join(skill, "refs", "a.md"), "参考 v2\n")
	s1 := copyState(base, skill, dst)
	write(filepath.Join(dst, "refs", "a.md"), "工具里改的\n")
	s2 := copyState(base, skill, dst)
	write(filepath.Join(skill, "refs", "a.md"), "参考\n")
	s3 := copyState(base, skill, dst)
	check("C16", "copy states: same / source-changed / both-changed / copy-changed", s0 == "same" && s1 == "source-changed" && s2 == "both-changed" && s3 == "copy-changed", fmt.Sprintf("%s %s %s %s", s0, s1, s2, s3))
	skillHash = treeHash(skill)

	// C17 knowledge-base vs external classification.
	vault := filepath.Join(root, "vault")
	cases := []struct {
		target string
		want   bool
	}{
		{filepath.Join(vault, "skills", "a"), true},
		{strings.ToUpper(filepath.Join(vault, "skills", "a")), true},
		{`\\?\` + filepath.Join(vault, "skills", "a"), true},
		{vault, true},
		{filepath.Join(root, "vault2", "skills", "a"), false},
		{filepath.Join(root, "other"), false},
	}
	allOK := true
	for _, c := range cases {
		if underRoot(c.target, vault) != c.want {
			allOK = false
			info("C17", fmt.Sprintf("mismatch target=%q want=%v", c.target, c.want))
		}
	}
	check("C17", "underRoot: case, \\\\?\\ prefix, sibling-prefix (vault2) handled", allOK, "")

	// C18 scan cost at about the size seen on this machine (700 entries).
	bench := filepath.Join(root, "bench")
	kbs := filepath.Join(bench, "kb")
	linked := filepath.Join(bench, "tool-linked")
	plain := filepath.Join(bench, "tool-plain")
	for i := 0; i < 350; i++ {
		name := fmt.Sprintf("技能-%03d", i)
		body := fmt.Sprintf("---\nname: %s\ndescription: 基准\n---\n%s\n", name, strings.Repeat("正文", 200))
		write(filepath.Join(kbs, name, "SKILL.md"), body)
		write(filepath.Join(plain, name, "SKILL.md"), body)
	}
	must(os.MkdirAll(linked, 0o755))
	t0 := time.Now()
	for i := 0; i < 350; i++ {
		name := fmt.Sprintf("技能-%03d", i)
		must(createJunction(filepath.Join(linked, name), filepath.Join(kbs, name), false))
	}
	createDur := time.Since(t0)
	var runs []time.Duration
	for r := 0; r < 5; r++ {
		start := time.Now()
		n := 0
		for _, dir := range []string{linked, plain} {
			entries, err := os.ReadDir(dir)
			must(err)
			for _, e := range entries {
				p := filepath.Join(dir, e.Name())
				if _, _, err := classify(p); err != nil {
					panic(err)
				}
				f, err := os.Open(filepath.Join(p, "SKILL.md"))
				must(err)
				buf := make([]byte, 4096)
				_, _ = f.Read(buf)
				f.Close()
				n++
			}
		}
		runs = append(runs, time.Since(start))
		if n != 700 {
			panic(fmt.Sprintf("scanned %d", n))
		}
	}
	inOrder := make([]string, len(runs))
	for i, r := range runs {
		inOrder[i] = r.Round(time.Millisecond).String()
	}
	info("C18", "scan runs in order: "+strings.Join(inOrder, " "))
	sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
	info("C18", fmt.Sprintf("create 350 junctions: %v (%.2fms each)", createDur.Round(time.Millisecond), float64(createDur.Microseconds())/350/1000))
	info("C18", fmt.Sprintf("scan 700 entries (classify + read SKILL.md 4KiB), 5 runs: min %v median %v max %v",
		runs[0].Round(time.Millisecond), runs[2].Round(time.Millisecond), runs[4].Round(time.Millisecond)))
	check("C18", "bench scan finished", true, "")

	check("END", "knowledge-base skill intact after all cases", treeHash(skill) == skillHash, "")
	fmt.Printf("RESULT pass=%d fail=%d\n", pass, fail)
	if fail > 0 {
		return 1
	}
	return 0
}

func goVersion() string {
	out, err := exec.Command("go", "env", "GOVERSION").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
