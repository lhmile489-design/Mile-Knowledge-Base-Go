//go:build !windows

package vault

// isSharingViolation has no portable meaning outside Windows.
func isSharingViolation(error) bool { return false }
