// Package skills manages the tool side of R2: registry, detection, scanning
// and distribution (PRD §15; 技术设计 15). This file is the Windows layer:
// reparse points are classified from their reparse data, not from os.Lstat's
// mode bits (those changed across Go releases — R-9, 技术设计 13.4).
package skills

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// LinkKind is what a path is, judged from its reparse data.
type LinkKind string

const (
	KindMissing      LinkKind = "missing"
	KindFile         LinkKind = "file"
	KindDir          LinkKind = "dir"
	KindJunction     LinkKind = "junction"
	KindSymlink      LinkKind = "symlink"
	KindOtherReparse LinkKind = "other-reparse"
)

const symlinkFlagRelative = 1

// ErrUNCTarget is returned when a junction target is a network path; R-9 C14
// makes this a production rule.
var ErrUNCTarget = errors.New("junction target must be a local absolute path")

// Classify reports the entry kind and, for links, the absolute target.
func Classify(path string) (LinkKind, string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", "", err
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return KindMissing, "", nil
		}
		return "", "", err
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		if attrs&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
			return KindDir, "", nil
		}
		return KindFile, "", nil
	}
	tag, target, err := readReparse(path)
	if err != nil {
		return "", "", err
	}
	switch tag {
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		return KindJunction, target, nil
	case windows.IO_REPARSE_TAG_SYMLINK:
		return KindSymlink, target, nil
	default:
		return KindOtherReparse, "", nil
	}
}

// readReparse opens the entry itself (never its target) and decodes the
// reparse buffer of a junction or a symlink.
func readReparse(path string) (uint32, string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, "", err
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return 0, "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var n uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_GET_REPARSE_POINT, nil, 0, &buf[0], uint32(len(buf)), &n, nil); err != nil {
		return 0, "", err
	}
	tag := binary.LittleEndian.Uint32(buf[0:4])
	subOff := int(binary.LittleEndian.Uint16(buf[8:10]))
	subLen := int(binary.LittleEndian.Uint16(buf[10:12]))
	prOff := int(binary.LittleEndian.Uint16(buf[12:14]))
	prLen := int(binary.LittleEndian.Uint16(buf[14:16]))
	var base int
	relative := false
	switch tag {
	case windows.IO_REPARSE_TAG_MOUNT_POINT:
		base = 16
	case windows.IO_REPARSE_TAG_SYMLINK:
		relative = binary.LittleEndian.Uint32(buf[16:20])&symlinkFlagRelative != 0
		base = 20
	default:
		return tag, "", nil
	}
	decode := func(off, l int) string {
		raw := buf[base+off : base+off+l]
		u := make([]uint16, l/2)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(raw[2*i:])
		}
		return string(utf16.Decode(u))
	}
	target := decode(prOff, prLen)
	if target == "" {
		target = decode(subOff, subLen)
	}
	target = StripNTPrefix(target)
	if relative {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return tag, target, nil
}

// StripNTPrefix removes the \??\ and \\?\ prefixes Windows keeps internally.
func StripNTPrefix(s string) string {
	for _, prefix := range []string{`\??\UNC\`, `\\?\UNC\`} {
		if strings.HasPrefix(s, prefix) {
			return `\\` + s[len(prefix):]
		}
	}
	for _, prefix := range []string{`\??\`, `\\?\`} {
		if strings.HasPrefix(s, prefix) {
			return s[len(prefix):]
		}
	}
	return s
}

// CreateJunction makes link a directory junction pointing at target. The
// target must be an existing local directory; link must not exist. The kernel
// does not verify the target, so callers pass an already validated path
// (技术设计 13.4).
func CreateJunction(link, target string) error {
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if strings.HasPrefix(abs, `\\`) {
		return ErrUNCTarget
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("junction target is not a directory")
	}
	// Some tools' install directories do not exist until the first skill
	// arrives (技术设计 13.2, Antigravity); create the parent on demand.
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(link)
		}
	}()
	sub := `\??\` + abs
	subU := utf16.Encode([]rune(sub))
	prU := utf16.Encode([]rune(abs))
	pathBuf := make([]uint16, 0, len(subU)+len(prU)+2)
	pathBuf = append(pathBuf, subU...)
	pathBuf = append(pathBuf, 0)
	pathBuf = append(pathBuf, prU...)
	pathBuf = append(pathBuf, 0)
	dataLen := 8 + 2*len(pathBuf)
	buf := make([]byte, 8+dataLen)
	binary.LittleEndian.PutUint32(buf[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:], uint16(dataLen))
	binary.LittleEndian.PutUint16(buf[8:], 0)
	binary.LittleEndian.PutUint16(buf[10:], uint16(2*len(subU)))
	binary.LittleEndian.PutUint16(buf[12:], uint16(2*len(subU)+2))
	binary.LittleEndian.PutUint16(buf[14:], uint16(2*len(prU)))
	for i, c := range pathBuf {
		binary.LittleEndian.PutUint16(buf[16+2*i:], c)
	}
	p, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var n uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &buf[0], uint32(len(buf)), nil, 0, &n, nil); err != nil {
		return err
	}
	// The kernel accepts missing targets; verify through the link once.
	if _, err := os.Stat(link); err != nil {
		return err
	}
	ok = true
	return nil
}

// RemoveLink deletes a junction or directory symlink itself and refuses
// anything that is not a link, so it can never delete real content.
func RemoveLink(path string) error {
	kind, _, err := Classify(path)
	if err != nil {
		return err
	}
	if kind != KindJunction && kind != KindSymlink {
		return errors.New("not a link: " + string(kind))
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.RemoveDirectory(p)
}

// SafeWalk lists regular files under dir and never descends into, or reads
// through, a reparse point. skipped collects the links and other special
// entries it stepped over.
func SafeWalk(dir string) (files []string, skipped []string, err error) {
	var walk func(rel string) error
	walk = func(rel string) error {
		entries, err := os.ReadDir(filepath.Join(dir, rel))
		if err != nil {
			return err
		}
		for _, e := range entries {
			child := filepath.Join(rel, e.Name())
			kind, _, err := Classify(filepath.Join(dir, child))
			if err != nil {
				return err
			}
			switch kind {
			case KindDir:
				if err := walk(child); err != nil {
					return err
				}
			case KindFile:
				files = append(files, child)
			default:
				skipped = append(skipped, child)
			}
		}
		return nil
	}
	err = walk("")
	return files, skipped, err
}

// UnderRoot reports whether target is root itself or inside it, comparing
// case-insensitively on separator boundaries (vault2 is not under vault).
func UnderRoot(target, root string) bool {
	t := strings.ToLower(filepath.Clean(StripNTPrefix(target)))
	r := strings.ToLower(filepath.Clean(StripNTPrefix(root)))
	if t == r {
		return true
	}
	return strings.HasPrefix(t, strings.TrimRight(r, `\`)+`\`)
}
