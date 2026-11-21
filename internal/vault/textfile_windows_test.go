//go:build windows

package vault

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A reader without FILE_SHARE_DELETE must not make a save fail: Write retries
// the replace until the reader closes (the indexer and scans do exactly this).
func TestWriteRetriesSharingViolation(t *testing.T) {
	root := t.TempDir()
	if _, f := Write(root, "a.md", 0, "first"); f != nil {
		t.Fatalf("create: %+v", f)
	}
	revision, _, f := StatRevision(root, "a.md")
	if f != nil {
		t.Fatalf("stat: %+v", f)
	}
	path := filepath.Join(root, "a.md")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("hold open: %v", err)
	}
	closed := make(chan struct{})
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = windows.CloseHandle(handle)
		close(closed)
	}()
	outcome, f := Write(root, "a.md", revision, "second")
	if f != nil || outcome.Status != "written" {
		t.Fatalf("write while held = %+v %+v", outcome, f)
	}
	<-closed
	snapshot, f := Read(root, "a.md")
	if f != nil || snapshot.Text != "second" {
		t.Fatalf("read back = %+v %+v", snapshot, f)
	}
}
