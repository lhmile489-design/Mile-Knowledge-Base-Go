package skills

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// FR-SKM-DIST (PRD §15.5; 技术设计 15.5): distribute one knowledge-base skill
// to tool directories, junction first with a copy fallback. Every operation
// re-validates the target: a preview may be stale (API.md §12.3).

// DistributeTarget is one resolved destination. InstallDir is the tool's
// registered skill directory; the skill lands in InstallDir/<name>.
type DistributeTarget struct {
	TargetID       string
	ToolID         string
	Name           string
	InstallDir     string
	ReadsShared    bool // the tool reads ~/.agents/skills (duplicate hint)
	AllowOverwrite bool
	Conflict       string // cross-category name conflict detail from the caller
}

// DistributeOptions carries the preview/execute inputs.
type DistributeOptions struct {
	VaultID        string
	SkillPath      string // vault-relative, e.g. skills/前端/vue
	SourceDir      string
	Mode           string // link | copy
	Targets        []DistributeTarget
	Backups        *BackupStore
	Now            func() time.Time
	SharedTargeted bool // this request also targets the shared directory
	Distributions  []Distribution
	CreateJunction func(link, target string) error // injectable for tests
}

// PreviewDistribute answers M78 without writing anything.
func PreviewDistribute(opts DistributeOptions) []contract.SkillPreviewItem {
	items := make([]contract.SkillPreviewItem, 0, len(opts.Targets))
	for _, target := range opts.Targets {
		item := contract.SkillPreviewItem{TargetID: target.TargetID, ToolID: target.ToolID, Name: target.Name}
		if !IsValidSegment(target.Name) {
			item.Action = "conflict"
			item.Detail = "技能名不合法"
			items = append(items, item)
			continue
		}
		if target.Conflict != "" {
			item.Action = "conflict"
			item.Detail = target.Conflict
			items = append(items, item)
			continue
		}
		targetDir := filepath.Join(target.InstallDir, target.Name)
		item.TargetPath = targetDir
		if duplicateRisk(opts, target) {
			item.Detail = "可能重复加载"
		}
		kind, linkedTo, err := Classify(targetDir)
		switch {
		case err == nil && kind == KindMissing:
			item.Action = "create"
		case err == nil && (kind == KindJunction || kind == KindSymlink):
			if strings.EqualFold(filepath.Clean(linkedTo), filepath.Clean(opts.SourceDir)) {
				item.Action = "exists"
				item.Detail = "已分发（链接）"
			} else if target.AllowOverwrite {
				item.Action = "overwrite"
				item.BackupPath = filepath.Join(backupRoot(opts), target.ToolID, target.Name)
			} else {
				item.Action = "exists"
				item.Detail = "目标已存在"
			}
		case err == nil:
			if target.AllowOverwrite {
				item.Action = "overwrite"
				item.BackupPath = filepath.Join(backupRoot(opts), target.ToolID, target.Name)
			} else {
				item.Action = "exists"
				item.Detail = "目标已存在"
			}
		default:
			item.Action = "conflict"
			item.Detail = err.Error()
		}
		items = append(items, item)
	}
	return items
}

func backupRoot(opts DistributeOptions) string {
	if opts.Backups != nil {
		return opts.Backups.Dir
	}
	return ""
}

// duplicateRisk reports the "may load twice" hint: the skill goes (or already
// is) into the shared directory while a reader tool of that directory gets it
// as well (PRD FR-SKM-DIST).
func duplicateRisk(opts DistributeOptions, target DistributeTarget) bool {
	if target.ToolID == "agents" || !target.ReadsShared {
		return false
	}
	if opts.SharedTargeted {
		return true
	}
	for _, dist := range opts.Distributions {
		if dist.ToolID == "agents" && dist.Name == target.Name {
			return true
		}
	}
	// The skill may already sit in the shared directory without a record
	// (installed before R2); the caller passes its expanded path as a hint.
	return false
}

// ExecuteDistribute performs M79 and returns the records to persist.
func ExecuteDistribute(opts DistributeOptions) ([]contract.SkillOpItem, []Distribution) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	createLink := opts.CreateJunction
	if createLink == nil {
		createLink = CreateJunction
	}
	baseHash, _ := TreeHash(opts.SourceDir)
	var items []contract.SkillOpItem
	var records []Distribution
	for _, target := range opts.Targets {
		item := contract.SkillOpItem{TargetID: target.TargetID, ToolID: target.ToolID}
		targetDir := filepath.Join(target.InstallDir, target.Name)
		if !IsValidSegment(target.Name) || target.Conflict != "" {
			item.Result = "failed"
			item.Detail = "名称冲突"
			items = append(items, item)
			continue
		}
		kind, linkedTo, err := Classify(targetDir)
		switch {
		case err != nil:
			item.Result = "failed"
			item.Detail = err.Error()
			items = append(items, item)
			continue
		case kind != KindMissing:
			if kind == KindJunction || kind == KindSymlink {
				if strings.EqualFold(filepath.Clean(linkedTo), filepath.Clean(opts.SourceDir)) {
					item.Result = "skipped"
					item.Detail = "已分发（链接）"
					items = append(items, item)
					continue
				}
			}
			if !target.AllowOverwrite {
				item.Result = "skipped"
				item.Detail = "目标已存在"
				items = append(items, item)
				continue
			}
			if opts.Backups == nil {
				item.Result = "failed"
				item.Detail = "备份不可用"
				items = append(items, item)
				continue
			}
			meta, _, err := opts.Backups.Create(target.ToolID, targetDir, "overwrite", now())
			if err != nil {
				item.Result = "failed"
				item.Detail = "备份失败：" + err.Error()
				items = append(items, item)
				continue
			}
			item.BackupID = meta.ID
		}
		mode := opts.Mode
		detail := ""
		if mode == "link" {
			if err := os.MkdirAll(target.InstallDir, 0o755); err != nil {
				item.Result = "failed"
				item.Detail = err.Error()
				items = append(items, item)
				continue
			}
			if err := createLink(targetDir, opts.SourceDir); err != nil {
				mode = "copy"
				detail = "联接创建失败，已改用拷贝"
			}
		}
		if mode != "link" {
			if err := copyTree(opts.SourceDir, targetDir); err != nil {
				item.Result = "failed"
				item.Detail = detail + "拷贝失败：" + err.Error()
				items = append(items, item)
				continue
			}
		}
		item.Result = "linked"
		if mode != "link" {
			item.Result = "copied"
		}
		item.Detail = detail
		items = append(items, item)
		records = append(records, Distribution{
			ID:         distributionID(),
			VaultID:    opts.VaultID,
			SkillPath:  opts.SkillPath,
			ToolID:     target.ToolID,
			TargetPath: targetDir,
			Mode:       mode,
			Name:       target.Name,
			BaseHash:   baseHash,
			CreatedAt:  now().UTC().Format(time.RFC3339),
		})
	}
	return items, records
}

func distributionID() string {
	return backupID()
}

// copyTree copies regular files (never links) from source to target.
func copyTree(source, target string) error {
	files, _, err := SafeWalk(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, rel := range files {
		src := filepath.Join(source, rel)
		dst := filepath.Join(target, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		out, err := os.Create(dst)
		if err != nil {
			_ = in.Close()
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			return err
		}
		_ = in.Close()
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}
