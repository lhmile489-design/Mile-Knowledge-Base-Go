//go:build e2e

package skills

import "os"

// e2e builds can force the junction-creation failure path so the copy fallback
// (AC-SKM-4) is testable without a network drive.
func junctionFailureForced() bool { return os.Getenv("SKILLS_FAIL_JUNCTION") == "1" }
