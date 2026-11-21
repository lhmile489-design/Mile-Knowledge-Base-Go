package mutation

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/pathguard"
)

// MaxImportBytes mirrors the FR-SKILL per-file limit (API §13.4).
const MaxImportBytes = int64(50 << 20)

// resolveStagedFile maps a stageId + stagePath to the real file inside
// <stagingRoot>/<stageId>/root and refuses anything that escapes it.
func resolveStagedFile(stagingRoot, stageID, stagePath string) (string, *contract.Failure) {
	if stagingRoot == "" {
		return "", contract.Fail(contract.CodeInvalidInput, "暂存区不可用")
	}
	if !isStageID(stageID) {
		return "", contract.Fail(contract.CodeInvalidInput, "暂存标识不合法")
	}
	segments := strings.Split(stagePath, "/")
	if len(segments) == 0 {
		return "", contract.Fail(contract.CodeInvalidInput, "暂存文件路径不合法")
	}
	for _, segment := range segments {
		// Real skills contain dot files and deep trees (.gitignore,
		// references/...); only the forms that could escape the stage root are
		// refused. Every parent is checked by pathguard.Resolve below.
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `:\`) {
			return "", contract.Fail(contract.CodeInvalidInput, "暂存文件路径不合法")
		}
	}
	rel := stageID + "/root/" + strings.Join(segments, "/")
	abs, f := pathguard.Resolve(stagingRoot, rel)
	if f != nil {
		return "", contract.Fail(contract.CodeInvalidInput, "暂存文件路径不合法")
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return "", contract.Fail(contract.CodeNotFound, "暂存文件不存在")
	}
	if info.Size() > MaxImportBytes {
		return "", contract.Fail(contract.CodeTooLarge, "文件超过 50 MiB").WithLimit(info.Size(), MaxImportBytes)
	}
	return abs, nil
}

// stagedFileHash is the sha256 of a staged file, formatted like TreeHash.
func stagedFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

// isStageID accepts s-<12 hex> (API §13.3).
func isStageID(id string) bool {
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

// assertStagedHash verifies the pinned hash before writing anything.
func assertStagedHash(path, expected string) *contract.Failure {
	actual, err := stagedFileHash(path)
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法读取暂存文件")
	}
	if !strings.EqualFold(actual, expected) {
		return contract.Fail(contract.CodeRevisionMismatch, "暂存文件已变化，请重新检查更新")
	}
	return nil
}
