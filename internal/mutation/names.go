// Package mutation builds and validates change plans. Planning never touches
// the disk for writing: it reads state, checks rules and returns a plan the
// user confirms (PRD FR-MUT, legacy shared/entryName.ts and
// MutationService.ts).
package mutation

import (
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// MaxNameLength mirrors shared/entryName.ts.
const MaxNameLength = 200

// reservedNames are the DOS device names; they stay reserved with an extension
// ("nul.md" still means the device).
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// CheckEntryName validates a new or renamed entry name against its siblings.
// The returned name is trimmed.
func CheckEntryName(input string, siblings []string) (string, *contract.Failure) {
	name := strings.TrimSpace(input)
	if name == "" {
		return "", contract.Fail(contract.CodeInvalidInput, "名称不能为空")
	}
	if len([]rune(name)) > MaxNameLength {
		return "", contract.Fail(contract.CodeInvalidInput, "名称过长").WithLimit(int64(len([]rune(name))), MaxNameLength)
	}
	for _, r := range name {
		if strings.ContainsRune(`<>:"|?*\/`, r) || r <= 0x1f {
			return "", contract.Fail(contract.CodeInvalidInput, "名称含有不允许的字符")
		}
	}
	// Windows silently strips a trailing dot or space, so a name that passes
	// validation would not be the name on disk.
	if name == "." || name == ".." || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", contract.Fail(contract.CodeInvalidInput, "名称不能以点或空格结尾")
	}
	base := strings.SplitN(strings.ToLower(name), ".", 2)[0]
	if reservedNames[base] {
		return "", contract.Fail(contract.CodeInvalidInput, "这是系统保留名称")
	}
	lower := strings.ToLower(name)
	for _, sibling := range siblings {
		if strings.ToLower(sibling) == lower {
			return "", contract.Fail(contract.CodeNameConflict, "已存在同名项")
		}
	}
	return name, nil
}

// WithDefaultExtension appends ".md" when the name has no extension.
func WithDefaultExtension(name string) string {
	if dot := strings.LastIndex(name, "."); dot > 0 {
		return name
	}
	return name + ".md"
}
