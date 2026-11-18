package mutation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Transaction storage: <vault>/.mile-kb/transactions/<tx-id>/manifest.json plus
// payload/<sha256(path)> pre-images (docs/数据库设计.md §3.5). The byte layout
// must stay identical to the legacy TransactionService so both apps can read
// the same directory.

const (
	systemDir   = ".mile-kb"
	txDir       = "transactions"
	manifestNme = "manifest.json"
	payloadDir  = "payload"
)

// Entry kinds.
const (
	KindFile      = "file"
	KindDirectory = "directory"
	KindAbsent    = "absent"
)

// Entry is one manifest row. Field order matters: it is the JSON key order.
type Entry struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Hash      string `json:"hash,omitempty"`
	Size      int64  `json:"size,omitempty"`
	AfterHash string `json:"afterHash,omitempty"`
	AfterKind string `json:"afterKind,omitempty"`
}

// Manifest is the transaction record.
type Manifest struct {
	ID        string  `json:"id"`
	VaultID   string  `json:"vaultId"`
	PlanID    string  `json:"planId"`
	CreatedAt string  `json:"createdAt"`
	Status    string  `json:"status"` // open | committed | partial
	Note      string  `json:"note,omitempty"`
	Entries   []Entry `json:"entries"`
}

// Tx is an open transaction.
type Tx struct {
	ID       string
	Dir      string
	VaultID  string
	Root     string
	Manifest Manifest
}

// TxManager creates and updates transactions under one vault root.
type TxManager struct {
	Root string
	Now  func() time.Time
	Rand func() string
}

func (m TxManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m TxManager) randomHex(n int) string {
	if m.Rand != nil {
		return m.Rand()[:min(n, len(m.Rand()))]
	}
	buf := make([]byte, (n+1)/2)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("0", n)
	}
	return hex.EncodeToString(buf)[:n]
}

// Begin opens a transaction directory with an `open` manifest.
func (m TxManager) Begin(vaultID, planID string) (*Tx, error) {
	// A base36 millisecond prefix sorts directory names by time; the random
	// suffix keeps them unique.
	id := strconv.FormatInt(m.now().UnixMilli(), 36) + "-" + m.randomHex(8)
	dir := filepath.Join(m.Root, systemDir, txDir, id)
	if err := os.MkdirAll(filepath.Join(dir, payloadDir), 0o755); err != nil {
		return nil, err
	}
	tx := &Tx{
		ID: id, Dir: dir, VaultID: vaultID, Root: m.Root,
		Manifest: Manifest{
			ID: id, VaultID: vaultID, PlanID: planID,
			CreatedAt: m.now().UTC().Format(time.RFC3339Nano),
			Status:    "open", Entries: []Entry{},
		},
	}
	if err := m.persist(tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// Capture records the pre-image of rel. Directories only get a marker; a file
// is copied into payload/ keyed by the hash of its path.
func (m TxManager) Capture(tx *Tx, rel, abs string) error {
	if f := assertSafeRelative(rel); f != nil {
		return contractError{code: f.Code, message: f.Message}
	}
	for _, entry := range tx.Manifest.Entries {
		if entry.Path == rel {
			return nil
		}
	}
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			tx.Manifest.Entries = append(tx.Manifest.Entries, Entry{Path: rel, Kind: KindAbsent})
			return m.persist(tx)
		}
		return err
	}
	entry := Entry{Path: rel}
	if info.IsDir() {
		entry.Kind = KindDirectory
	} else {
		body, err := os.ReadFile(abs)
		if err != nil {
			return err
		}
		raw := sha256.Sum256([]byte(rel))
		if err := os.WriteFile(filepath.Join(tx.Dir, payloadDir, hex.EncodeToString(raw[:])), body, 0o644); err != nil {
			return err
		}
		entry.Kind = KindFile
		entry.Hash = hashBytes(body)
		entry.Size = int64(len(body))
	}
	tx.Manifest.Entries = append(tx.Manifest.Entries, entry)
	return m.persist(tx)
}

// CommitWithAfterImages records the post-state of every captured path and marks
// the transaction committed.
func (m TxManager) CommitWithAfterImages(tx *Tx) error {
	for i := range tx.Manifest.Entries {
		entry := &tx.Manifest.Entries[i]
		abs := filepath.Join(tx.Root, filepath.FromSlash(entry.Path))
		info, err := os.Lstat(abs)
		switch {
		case err != nil && os.IsNotExist(err):
			entry.AfterKind = KindAbsent
		case err != nil:
			return err
		case info.IsDir():
			entry.AfterKind = KindDirectory
		default:
			body, readErr := os.ReadFile(abs)
			if readErr != nil {
				return readErr
			}
			entry.AfterKind = KindFile
			entry.AfterHash = hashBytes(body)
		}
	}
	tx.Manifest.Status = "committed"
	return m.persist(tx)
}

// MarkPartial records that some actions failed; completed ones stay captured.
func (m TxManager) MarkPartial(tx *Tx, note string) error {
	tx.Manifest.Status = "partial"
	tx.Manifest.Note = note
	return m.persist(tx)
}

// PayloadPath is where the pre-image of rel lives.
func (m TxManager) PayloadPath(tx *Tx, rel string) string {
	raw := sha256.Sum256([]byte(rel))
	return filepath.Join(tx.Dir, payloadDir, hex.EncodeToString(raw[:]))
}

// persist writes the manifest atomically: temp file, then rename.
func (m TxManager) persist(tx *Tx) error {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // match JSON.stringify byte for byte
	enc.SetIndent("", "  ")
	if err := enc.Encode(tx.Manifest); err != nil {
		return err
	}
	tmp := filepath.Join(tx.Dir, manifestNme+"."+m.randomHex(8)+".tmp")
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(tx.Dir, manifestNme))
}

func hashBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return hashBytes(body), nil
}

// assertSafeRelative mirrors the legacy guard: a malicious manifest path would
// let restore write outside the vault, so it is rejected at capture time.
func assertSafeRelative(rel string) *contract.Failure {
	if rel == "" || strings.Contains(rel, "\\") {
		return contract.Fail(contract.CodePathEscape, "事务路径不合法")
	}
	if len(rel) >= 2 && rel[1] == ':' {
		return contract.Fail(contract.CodePathEscape, "事务路径必须是库内相对路径")
	}
	if strings.HasPrefix(rel, "/") {
		return contract.Fail(contract.CodePathEscape, "事务路径必须是库内相对路径")
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return contract.Fail(contract.CodePathEscape, "事务路径越界")
		}
	}
	return nil
}
