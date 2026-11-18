package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// AttachmentDir is where pasted/dropped images go (PRD FR-DOC).
const AttachmentDir = "raws/attachments"

// MaxAttachmentBytes is the per-image cap.
const MaxAttachmentBytes = 8 << 20

// SniffImage reports the extension and MIME type from the file's magic bytes.
// SVG and ICO are not in the table on purpose (FR-DOC).
func SniffImage(body []byte) (ext, mime string, ok bool) {
	switch {
	case len(body) >= 8 && bytes.Equal(body[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "png", "image/png", true
	case len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF:
		return "jpg", "image/jpeg", true
	case len(body) >= 6 && (bytes.Equal(body[:6], []byte("GIF87a")) || bytes.Equal(body[:6], []byte("GIF89a"))):
		return "gif", "image/gif", true
	case len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "webp", "image/webp", true
	case len(body) >= 2 && body[0] == 'B' && body[1] == 'M':
		return "bmp", "image/bmp", true
	case len(body) >= 12 && bytes.Equal(body[4:8], []byte("ftyp")) &&
		(bytes.Equal(body[8:12], []byte("avif")) || bytes.Equal(body[8:12], []byte("avis"))):
		return "avif", "image/avif", true
	}
	return "", "", false
}

// SaveAttachment writes an image under raws/attachments with a content-hash
// name. A second upload of the same bytes reports existed without rewriting
// (legacy saveImage semantics; no transaction, no audit record).
func SaveAttachment(root string, body []byte) (string, bool, *contract.Failure) {
	if len(body) == 0 {
		return "", false, contract.Fail(contract.CodeInvalidInput, "空文件")
	}
	if len(body) > MaxAttachmentBytes {
		return "", false, contract.Fail(contract.CodeTooLarge, "图片超过 8 MiB")
	}
	ext, _, ok := SniffImage(body)
	if !ok {
		return "", false, contract.Fail(contract.CodeInvalidInput, "不支持的图片格式")
	}
	dir, f := pathguard.Resolve(root, AttachmentDir)
	if f != nil {
		return "", false, f
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法创建附件目录")
	}
	// Re-resolve after creating the directory: if it appeared as a reparse
	// point in between, this refuses instead of writing outside the vault.
	dir, f = pathguard.Resolve(root, AttachmentDir)
	if f != nil {
		return "", false, f
	}
	sum := sha256.Sum256(body)
	name := "pasted-" + hex.EncodeToString(sum[:])[:12] + "." + ext
	rel := AttachmentDir + "/" + name
	abs := filepath.Join(dir, name)
	if _, err := os.Lstat(abs); err == nil {
		return rel, true, nil
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法写入附件")
	}
	if err := os.Rename(tmp, abs); err != nil {
		return "", false, contract.Fail(contract.CodeIOFailure, "无法写入附件")
	}
	return rel, false, nil
}
