package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// Size limits (PRD FR-DOC): documents degrade in the UI above 1 MiB and become
// read-only above 2 MiB; a single write may never exceed 2 MiB.
const (
	MaxWriteBytes   = 2 << 20
	SourceOnlyBytes = 1 << 20
	ReadOnlyBytes   = 2 << 20
	MaxReadBytes    = 16 << 20
)

// revisionTracker mirrors the legacy RevisionRegistry: every observed change
// of (size, mtime, sha256) bumps a monotonic revision for that path.
type revisionTracker struct {
	mu      sync.Mutex
	next    int64
	entries map[string]revisionEntry
}

type revisionEntry struct {
	revision int64
	size     int64
	mtimeMs  int64
	hash     string
}

var revisions = &revisionTracker{entries: map[string]revisionEntry{}}

// Observe returns the current revision for a fingerprint, bumping it when the
// file changed since the last observation.
func (t *revisionTracker) Observe(key string, size, mtimeMs int64, hash string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[key]
	if ok && e.size == size && e.mtimeMs == mtimeMs && e.hash == hash {
		return e.revision
	}
	t.next++
	e = revisionEntry{revision: t.next, size: size, mtimeMs: mtimeMs, hash: hash}
	t.entries[key] = e
	return e.revision
}

// ResetRoot forgets every tracked path under a vault root (vault forget).
func (t *revisionTracker) ResetRoot(root string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	prefix := root + "|"
	for key := range t.entries {
		if strings.HasPrefix(key, prefix) {
			delete(t.entries, key)
		}
	}
}

// Reset forgets a path (used when a vault is forgotten).
func (t *revisionTracker) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// hashBytes is the document fingerprint.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func failf(code, msg string) *contract.Failure { return contract.Fail(code, msg) }

// Read loads a document, returning LF text plus the metadata needed to save it
// back byte-identically (BOM and dominant line ending).
func Read(root, rel string) (contract.DocumentSnapshot, *contract.Failure) {
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		return contract.DocumentSnapshot{}, f
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return contract.DocumentSnapshot{}, failf(contract.CodeNotFound, "文件不存在")
		}
		return contract.DocumentSnapshot{}, failf(contract.CodeIOFailure, "无法读取文件")
	}
	if info.IsDir() {
		return contract.DocumentSnapshot{}, failf(contract.CodeNotAFile, "不是文件")
	}
	if info.Size() > MaxReadBytes {
		return contract.DocumentSnapshot{}, failf(contract.CodeTooLarge, "文件过大").WithLimit(info.Size(), MaxReadBytes)
	}

	file, err := os.Open(abs)
	if err != nil {
		return contract.DocumentSnapshot{}, failf(contract.CodeIOFailure, "无法读取文件")
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, MaxReadBytes+1))
	if err != nil {
		return contract.DocumentSnapshot{}, failf(contract.CodeIOFailure, "无法读取文件")
	}
	if int64(len(raw)) > MaxReadBytes {
		return contract.DocumentSnapshot{}, failf(contract.CodeTooLarge, "文件过大").WithLimit(int64(len(raw)), MaxReadBytes)
	}

	bom := bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	body := raw
	if bom {
		body = body[3:]
	}
	if !utf8.Valid(body) {
		return contract.DocumentSnapshot{}, failf(contract.CodeUnsupportedEncoding, "文件不是 UTF-8 编码")
	}

	crlf := bytes.Count(body, []byte("\r\n"))
	lf := bytes.Count(body, []byte("\n")) - crlf
	eol := "lf"
	if crlf > lf {
		eol = "crlf"
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")

	hash := hashBytes(raw)
	rev := revisions.Observe(root+"|"+rel, info.Size(), info.ModTime().UnixMilli(), hash)

	snap := contract.DocumentSnapshot{
		Path: rel, Text: text, Revision: rev, Hash: hash, Encoding: "utf8",
		Size: info.Size(), EOL: eol, BOM: bom,
	}
	switch {
	case info.Size() > ReadOnlyBytes:
		snap.Degraded = "large-read-only"
	case info.Size() > SourceOnlyBytes:
		snap.Degraded = "large-source-only"
	}
	return snap, nil
}

// Write saves LF text atomically, restoring the BOM and line endings the file
// already had. expectedRevision 0 means "the caller believes it is new".
func Write(root, rel string, expectedRevision int64, text string) (contract.WriteOutcome, *contract.Failure) {
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		return contract.WriteOutcome{}, f
	}
	if f := pathguard.ValidateName(filepath.Base(abs)); f != nil {
		return contract.WriteOutcome{}, f
	}
	if len(text) > MaxWriteBytes {
		return contract.WriteOutcome{}, failf(contract.CodeTooLarge, "内容超过 2 MiB").WithLimit(int64(len(text)), MaxWriteBytes)
	}
	if !utf8.ValidString(text) {
		return contract.WriteOutcome{}, failf(contract.CodeUnsupportedEncoding, "内容不是 UTF-8")
	}

	mode := fs.FileMode(0o644)
	existing := false
	bom := false
	eol := "lf"
	var diskText string
	var diskRaw []byte
	var diskHash string
	var diskRev int64

	if info, err := os.Stat(abs); err == nil {
		if info.IsDir() {
			return contract.WriteOutcome{}, failf(contract.CodeNotAFile, "目标是目录")
		}
		existing = true
		mode = info.Mode().Perm()
		raw, readErr := os.ReadFile(abs)
		if readErr != nil {
			return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法读取磁盘版本")
		}
		bom = bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
		body := raw
		if bom {
			body = body[3:]
		}
		if !utf8.Valid(body) {
			return contract.WriteOutcome{}, failf(contract.CodeUnsupportedEncoding, "磁盘文件不是 UTF-8")
		}
		crlf := bytes.Count(body, []byte("\r\n"))
		lf := bytes.Count(body, []byte("\n")) - crlf
		if crlf > lf {
			eol = "crlf"
		}
		diskText = strings.ReplaceAll(string(body), "\r\n", "\n")
		diskHash = hashBytes(raw)
		diskRev = revisions.Observe(root+"|"+rel, info.Size(), info.ModTime().UnixMilli(), diskHash)
	}

	currentRev := int64(0)
	if existing {
		currentRev = diskRev
	}
	if expectedRevision != currentRev {
		return contract.WriteOutcome{
			Status:       "conflict",
			DiskText:     diskText,
			DiskRevision: diskRev,
			DiskHash:     diskHash,
			DiskSize:     int64(len(diskRaw)),
			BaseRevision: expectedRevision,
		}, nil
	}

	out := []byte(text)
	if eol == "crlf" {
		out = []byte(strings.ReplaceAll(text, "\n", "\r\n"))
	}
	if bom {
		out = append([]byte{0xEF, 0xBB, 0xBF}, out...)
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法创建目录")
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), "."+filepath.Base(abs)+".*.tmp")
	if err != nil {
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法写入文件")
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法写入文件")
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法写入文件")
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法写入文件")
	}
	// The temp file is 0600; carry over the original mode, or 0644 for a new file.
	_ = os.Chmod(tmpName, mode)
	// A reader that opened the target without FILE_SHARE_DELETE makes the
	// replace fail with a sharing violation on Windows; the indexer and scans
	// hold documents for a moment, so retry briefly before giving up.
	if err := replaceFile(tmpName, abs); err != nil {
		os.Remove(tmpName)
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "无法写入文件")
	}

	info, err := os.Stat(abs)
	if err != nil {
		return contract.WriteOutcome{}, failf(contract.CodeIOFailure, "写入后无法确认文件")
	}
	hash := hashBytes(out)
	rev := revisions.Observe(root+"|"+rel, info.Size(), info.ModTime().UnixMilli(), hash)
	return contract.WriteOutcome{Status: "written", Revision: rev, Hash: hash, Size: info.Size()}, nil
}

// replaceFile renames over the target, retrying while a transient reader keeps
// it open (sharing violation). Six attempts cover the indexer's short reads.
func replaceFile(oldpath, newpath string) error {
	err := os.Rename(oldpath, newpath)
	for attempt := 0; attempt < 6 && err != nil && isSharingViolation(err); attempt++ {
		time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
		err = os.Rename(oldpath, newpath)
	}
	return err
}

// StatRevision returns the current revision and hash of a file without keeping
// its text (used by the external-change check).
func StatRevision(root, rel string) (int64, string, *contract.Failure) {
	abs, f := pathguard.Resolve(root, rel)
	if f != nil {
		return 0, "", f
	}
	info, err := os.Stat(abs)
	if err != nil {
		return 0, "", failf(contract.CodeNotFound, "文件不存在")
	}
	if info.IsDir() {
		return 0, "", failf(contract.CodeNotAFile, "不是文件")
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return 0, "", failf(contract.CodeIOFailure, "无法读取文件")
	}
	hash := hashBytes(raw)
	return revisions.Observe(root+"|"+rel, info.Size(), info.ModTime().UnixMilli(), hash), hash, nil
}

// Touch records a fresh timestamp for tests that need mtime granularity.
func Touch(abs string) error {
	now := time.Now()
	return os.Chtimes(abs, now, now)
}

// ResetRevisions drops tracked revisions for a vault root (vault forget).
func ResetRevisions(root string) { revisions.ResetRoot(root) }
