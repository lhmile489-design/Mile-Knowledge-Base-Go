package skills

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

const (
	// StageTTL is how long a staged download stays usable (16.4).
	StageTTL = 30 * time.Minute
	// MaxStages bounds disk use; the oldest stage is dropped first.
	MaxStages = 3
	stageMeta = "meta.json"
	stageRoot = "root"
)

// Stage is one downloaded archive waiting for the user to pick skills.
type Stage struct {
	Dir  string // skills/tmp/<id>
	Meta StageMeta
}

// StageMeta is stage meta.json (数据库设计 4.6 M5 增补).
type StageMeta struct {
	Version    int                   `json:"version"`
	ID         string                `json:"id"`
	Kind       string                `json:"kind"` // github | zip
	Origin     *contract.SkillOrigin `json:"origin,omitempty"`
	Label      string                `json:"label"`
	CreatedAt  string                `json:"createdAt"`
	ExpiresAt  string                `json:"expiresAt"`
	TreeHashes map[string]string     `json:"treeHashes,omitempty"`
}

// Root is where the archive was unpacked.
func (s Stage) Root() string { return filepath.Join(s.Dir, stageRoot) }

// StageStore owns skills/tmp: creation, lookup, expiry and startup cleanup.
type StageStore struct {
	Dir string // <app data>/skills/tmp
	Now func() time.Time
}

// NewStageStore builds the store; call CleanAll once at startup.
func NewStageStore(dir string, now func() time.Time) *StageStore {
	if now == nil {
		now = time.Now
	}
	return &StageStore{Dir: dir, Now: now}
}

// CleanAll removes every stage; the meta files are derived state (4.6).
func (s *StageStore) CleanAll() error {
	if err := os.RemoveAll(s.Dir); err != nil {
		return err
	}
	return os.MkdirAll(s.Dir, 0o755)
}

// Create starts a new stage and enforces MaxStages by dropping the oldest.
func (s *StageStore) Create(kind, label string, origin *contract.SkillOrigin) (Stage, *contract.Failure) {
	s.Sweep()
	if list, err := s.list(); err == nil && len(list) >= MaxStages {
		sort.Slice(list, func(i, j int) bool {
			left, right := stageModTime(list[i]), stageModTime(list[j])
			if left.Equal(right) {
				return list[i].Meta.ID < list[j].Meta.ID
			}
			return left.Before(right)
		})
		for _, old := range list[:len(list)-MaxStages+1] {
			_ = os.RemoveAll(old.Dir)
		}
	}
	id, err := newStageID()
	if err != nil {
		return Stage{}, contract.Fail(contract.CodeIOFailure, "无法创建暂存区")
	}
	dir := filepath.Join(s.Dir, id)
	if err := os.MkdirAll(filepath.Join(dir, stageRoot), 0o755); err != nil {
		return Stage{}, contract.Fail(contract.CodeIOFailure, "无法创建暂存区")
	}
	now := s.Now()
	meta := StageMeta{
		Version:   1,
		ID:        id,
		Kind:      kind,
		Origin:    origin,
		Label:     label,
		CreatedAt: now.UTC().Format(time.RFC3339),
		ExpiresAt: now.Add(StageTTL).UTC().Format(time.RFC3339),
	}
	stage := Stage{Dir: dir, Meta: meta}
	if f := stage.save(); f != nil {
		_ = os.RemoveAll(dir)
		return Stage{}, f
	}
	return stage, nil
}

// Get loads a stage and treats an expired one as gone (API §13.3).
func (s *StageStore) Get(id string) (Stage, *contract.Failure) {
	if !IsStageID(id) {
		return Stage{}, contract.Fail(contract.CodeInvalidInput, "暂存标识不合法")
	}
	dir := filepath.Join(s.Dir, id)
	body, err := os.ReadFile(filepath.Join(dir, stageMeta))
	if err != nil {
		return Stage{}, contract.Fail(contract.CodeNotFound, "暂存已过期，请重新读取")
	}
	var meta StageMeta
	if err := json.Unmarshal(body, &meta); err != nil || meta.ID != id {
		_ = os.RemoveAll(dir)
		return Stage{}, contract.Fail(contract.CodeNotFound, "暂存已过期，请重新读取")
	}
	if s.expired(meta.ExpiresAt) {
		_ = os.RemoveAll(dir)
		return Stage{}, contract.Fail(contract.CodeNotFound, "暂存已过期，请重新读取")
	}
	return Stage{Dir: dir, Meta: meta}, nil
}

// Discard removes one stage; a missing one is not an error (M92).
func (s *StageStore) Discard(id string) *contract.Failure {
	if !IsStageID(id) {
		return contract.Fail(contract.CodeInvalidInput, "暂存标识不合法")
	}
	if err := os.RemoveAll(filepath.Join(s.Dir, id)); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法清理暂存区")
	}
	return nil
}

// Sweep removes expired stages; it runs on every Create and at startup.
func (s *StageStore) Sweep() {
	list, err := s.list()
	if err != nil {
		return
	}
	for _, stage := range list {
		if s.expired(stage.Meta.ExpiresAt) {
			_ = os.RemoveAll(stage.Dir)
		}
	}
}

// SetTreeHashes records candidate hashes for the later update preview.
func (stage Stage) SetTreeHashes(hashes map[string]string) *contract.Failure {
	stage.Meta.TreeHashes = hashes
	return stage.save()
}

func stageModTime(stage Stage) time.Time {
	if info, err := os.Stat(stage.Dir); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}

func (s StageStore) expired(value string) bool {
	expires, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return true
	}
	return s.Now().After(expires)
}

func (s *StageStore) list() ([]Stage, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	stages := make([]Stage, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !IsStageID(entry.Name()) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.Dir, entry.Name(), stageMeta))
		if err != nil {
			continue
		}
		var meta StageMeta
		if json.Unmarshal(body, &meta) == nil {
			stages = append(stages, Stage{Dir: filepath.Join(s.Dir, entry.Name()), Meta: meta})
		}
	}
	return stages, nil
}

func (stage Stage) save() *contract.Failure {
	body, err := json.MarshalIndent(stage.Meta, "", "  ")
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入暂存信息")
	}
	body = append(body, '\n')
	target := filepath.Join(stage.Dir, stageMeta)
	temporary := target + ".tmp"
	if err := os.WriteFile(temporary, body, 0o644); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入暂存信息")
	}
	if err := os.Rename(temporary, target); err != nil {
		_ = os.Remove(temporary)
		return contract.Fail(contract.CodeIOFailure, "无法写入暂存信息")
	}
	return nil
}

// IsStageID accepts s-<12 hex> (API §13.3).
func IsStageID(id string) bool {
	if len(id) != 14 || !strings.HasPrefix(id, "s-") {
		return false
	}
	for _, r := range id[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func newStageID() (string, error) {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return "s-" + hex.EncodeToString(buffer), nil
}
