package mutation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// ReadTx loads one transaction manifest.
func (m TxManager) ReadTx(id string) (*Manifest, *contract.Failure) {
	if !ValidTxID(id) {
		return nil, contract.Fail(contract.CodeInvalidInput, "事务 id 不合法")
	}
	file := filepath.Join(m.Root, systemDir, txDir, id, manifestNme)
	body, err := os.ReadFile(file)
	if err != nil {
		return nil, contract.Fail(contract.CodeNotFound, "事务记录不存在")
	}
	var manifest Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, contract.Fail(contract.CodeIOFailure, "事务记录已损坏")
	}
	if manifest.ID == "" {
		manifest.ID = id
	}
	return &manifest, nil
}

// ListTx returns manifests newest first.
func (m TxManager) ListTx() []Manifest {
	dir := filepath.Join(m.Root, systemDir, txDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]Manifest, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, f := m.ReadTx(entry.Name())
		if f != nil {
			continue
		}
		out = append(out, *manifest)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ID > out[j].ID
	})
	return out
}

// ValidTxID accepts the ids the transaction directory can contain; the id
// becomes part of a path, so anything else is rejected.
func ValidTxID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// Preview reports whether every entry can still be restored, without writing.
func (m TxManager) Preview(id string) ([]contract.RestorePreviewEntry, *contract.Failure) {
	manifest, f := m.ReadTx(id)
	if f != nil {
		return nil, f
	}
	entries := make([]contract.RestorePreviewEntry, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		ok, note := m.checkRestorable(entry)
		entries = append(entries, contract.RestorePreviewEntry{
			Path: entry.Path, Kind: entry.Kind, Restorable: ok, Note: note,
		})
	}
	return entries, nil
}

// Restore writes pre-images back. Entries changed after the transaction are
// reported as conflicted and left alone (FR-MUT).
func (m TxManager) Restore(id string) (restored, conflicted, touched []string, f *contract.Failure) {
	manifest, f := m.ReadTx(id)
	if f != nil {
		return nil, nil, nil, f
	}
	touchedSet := map[string]bool{}
	for _, entry := range manifest.Entries {
		if ok, _ := m.checkRestorable(entry); !ok {
			conflicted = append(conflicted, entry.Path)
			continue
		}
		abs := filepath.Join(m.Root, filepath.FromSlash(entry.Path))
		if err := m.restoreEntry(id, entry, abs); err != nil {
			conflicted = append(conflicted, entry.Path)
			continue
		}
		restored = append(restored, entry.Path)
		touchedSet[parentOf(entry.Path)] = true
	}
	for dir := range touchedSet {
		touched = append(touched, dir)
	}
	sort.Strings(touched)
	return restored, conflicted, touched, nil
}

func (m TxManager) restoreEntry(id string, entry Entry, abs string) error {
	switch entry.Kind {
	case KindAbsent:
		return os.RemoveAll(abs)
	case KindDirectory:
		return os.MkdirAll(abs, 0o755)
	default:
		payload, err := os.ReadFile(m.PayloadPath(&Tx{Dir: filepath.Join(m.Root, systemDir, txDir, id)}, entry.Path))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		// Copy to a staging name first so nobody observes a half-written file.
		staging := abs + "." + m.randomHex(8) + ".restore"
		if err := os.WriteFile(staging, payload, 0o644); err != nil {
			return err
		}
		return os.Rename(staging, abs)
	}
}

// checkRestorable compares the current disk state with the after-image. A
// transaction without after-images is allowed through with a note, because the
// legacy writer omitted them when it could not compute them.
func (m TxManager) checkRestorable(entry Entry) (bool, string) {
	if entry.AfterKind == "" {
		return true, "未记录事务后状态，无法判断外部是否再次改动"
	}
	abs := filepath.Join(m.Root, filepath.FromSlash(entry.Path))
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			if entry.AfterKind == KindAbsent {
				return true, ""
			}
			return false, "目标已被外部删除"
		}
		return false, "无法读取目标"
	}
	if info.IsDir() {
		if entry.AfterKind == KindDirectory {
			return true, ""
		}
		return false, "目标已变为目录"
	}
	if entry.AfterKind != KindFile {
		return false, "目标类型已改变"
	}
	current, err := hashFile(abs)
	if err != nil {
		return false, "无法读取目标"
	}
	if entry.AfterHash != "" && current != entry.AfterHash {
		return false, "目标在事务之后被外部修改"
	}
	return true, ""
}
