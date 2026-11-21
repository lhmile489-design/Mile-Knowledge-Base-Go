//go:build windows

package vault

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isSharingViolation reports a replace that failed only because another reader
// held the target; retrying after it closes succeeds.
func isSharingViolation(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION) ||
		errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
