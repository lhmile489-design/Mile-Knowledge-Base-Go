//go:build !e2e

package skills

// Production always tries the real junction first.
func junctionFailureForced() bool { return false }
