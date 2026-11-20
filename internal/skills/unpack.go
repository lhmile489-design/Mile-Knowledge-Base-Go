package skills

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// Archive limits from Q-SKM-5 and 技术设计 16.3. The archive size itself is
// enforced while downloading (GithubClient.MaxDownload); these caps cover the
// unpacked tree.
const (
	MaxExtractedBytes   = int64(200 << 20)
	MaxArchiveEntries   = 20000
	MaxSegmentLengthArc = 255
)

// windowsReserved are the DOS device names Windows refuses as file names.
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ExtractOptions tunes the caps; zero values use the defaults.
type ExtractOptions struct {
	MaxBytes   int64
	MaxEntries int
}

// ExtractResult reports what the unpacker wrote and refused to create.
type ExtractResult struct {
	Files   int
	Bytes   int64
	Skipped []contract.SkillStageSkip
}

// ExtractTarGz unpacks a GitHub tarball into root. The archive's first path
// component (`<repo>-<sha>/`) is stripped. On any failure the whole root is
// removed (技术设计 16.3.2): nothing half-written survives.
func ExtractTarGz(archive io.Reader, root string, options ExtractOptions) (ExtractResult, *contract.Failure) {
	if options.MaxBytes <= 0 {
		options.MaxBytes = MaxExtractedBytes
	}
	if options.MaxEntries <= 0 {
		options.MaxEntries = MaxArchiveEntries
	}
	state := &extractState{root: root, options: options, seen: map[string]string{}}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return state.finish(contract.Fail(contract.CodeUnsafeArchive, "压缩包不是有效的 tar.gz"))
	}
	reader := tar.NewReader(gz)
	entryCount := 0
	for {
		entryCount++
		if entryCount > options.MaxEntries {
			return state.finish(contract.Fail(contract.CodeTooLarge, "压缩包条目过多").WithLimit(int64(entryCount), int64(options.MaxEntries)))
		}
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return state.finish(contract.Fail(contract.CodeUnsafeArchive, "压缩包已损坏"))
		}
		rel, stripFailure := stripArchiveRoot(header.Name)
		if stripFailure != nil {
			return state.finish(stripFailure)
		}
		if rel == "" {
			continue
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			state.skipped = append(state.skipped, contract.SkillStageSkip{Path: header.Name, Reason: "symlink"})
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			state.skipped = append(state.skipped, contract.SkillStageSkip{Path: header.Name, Reason: "special"})
			continue
		}
		normalized, f := state.check(rel, header.Typeflag == tar.TypeDir)
		if f != nil {
			return state.finish(f)
		}
		if header.Typeflag == tar.TypeDir {
			if f := state.makeDir(normalized); f != nil {
				return state.finish(f)
			}
			continue
		}
		abs, f := state.resolve(normalized)
		if f != nil {
			return state.finish(f)
		}
		if f := state.writeFile(abs, reader, header.Size); f != nil {
			return state.finish(f)
		}
	}
	return state.result()
}

// ExtractZip pre-validates the central directory, then unpacks. A header that
// lies about a file's size is caught by counting the real decompressed bytes.
func ExtractZip(path, root string, options ExtractOptions) (ExtractResult, *contract.Failure) {
	if options.MaxBytes <= 0 {
		options.MaxBytes = MaxExtractedBytes
	}
	if options.MaxEntries <= 0 {
		options.MaxEntries = MaxArchiveEntries
	}
	state := &extractState{root: root, options: options, seen: map[string]string{}}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return state.finish(contract.Fail(contract.CodeUnsafeArchive, "文件不是有效的 zip"))
	}
	defer archive.Close()
	if len(archive.File) > options.MaxEntries {
		return state.finish(contract.Fail(contract.CodeTooLarge, "压缩包条目过多").WithLimit(int64(len(archive.File)), int64(options.MaxEntries)))
	}
	type entry struct {
		file  *zip.File
		rel   string
		isDir bool
		skip  string
	}
	entries := make([]entry, 0, len(archive.File))
	// First pass: every name is validated before a single byte is written
	// (16.3.4), so an unsafe entry later in the archive leaves nothing behind.
	for _, file := range archive.File {
		rel, f := stripArchiveRoot(file.Name)
		if f != nil {
			return state.finish(f)
		}
		if rel == "" {
			continue
		}
		isDir := strings.HasSuffix(file.Name, "/")
		mode := file.Mode()
		item := entry{file: file, rel: rel, isDir: isDir}
		switch {
		case mode&os.ModeSymlink != 0:
			item.skip = "symlink"
		case mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0:
			item.skip = "special"
		}
		normalized, f := state.check(rel, isDir)
		if f != nil {
			return state.finish(f)
		}
		item.rel = normalized
		entries = append(entries, item)
	}
	for _, item := range entries {
		if item.skip != "" {
			state.skipped = append(state.skipped, contract.SkillStageSkip{Path: item.file.Name, Reason: item.skip})
			continue
		}
		if item.isDir {
			if f := state.makeDir(item.rel); f != nil {
				return state.finish(f)
			}
			continue
		}
		abs, f := state.resolve(item.rel)
		if f != nil {
			return state.finish(f)
		}
		body, err := item.file.Open()
		if err != nil {
			return state.finish(contract.Fail(contract.CodeUnsafeArchive, "压缩包已损坏"))
		}
		f = state.writeFile(abs, body, int64(item.file.UncompressedSize64))
		body.Close()
		if f != nil {
			return state.finish(f)
		}
	}
	return state.result()
}

type extractState struct {
	root    string
	options ExtractOptions
	seen    map[string]string
	out     ExtractResult
	skipped []contract.SkillStageSkip
}

func (s *extractState) finish(f *contract.Failure) (ExtractResult, *contract.Failure) {
	_ = os.RemoveAll(s.root)
	return ExtractResult{}, f
}

func (s *extractState) result() (ExtractResult, *contract.Failure) {
	s.out.Skipped = s.skipped
	return s.out, nil
}

// stripArchiveRoot removes the archive's top-level directory and validates the
// remainder (16.3.2). It returns "" for the top directory itself.
func stripArchiveRoot(name string) (string, *contract.Failure) {
	cleaned := strings.ReplaceAll(name, "\\", "/")
	cleaned = strings.TrimSuffix(cleaned, "/")
	if cleaned == "" {
		return "", nil
	}
	if strings.HasPrefix(cleaned, "/") || strings.Contains(cleaned, ":") {
		return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包含绝对路径或盘符").WithDetail(name)
	}
	segments := strings.Split(cleaned, "/")
	if segments[0] == ".." {
		return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包含不安全的路径").WithDetail(name)
	}
	rest := segments[1:]
	if len(rest) == 0 {
		return "", nil
	}
	return strings.Join(rest, "/"), nil
}

// check validates every segment and records the entry so duplicate names that
// differ only by case are rejected before Windows merges them.
func (s *extractState) check(rel string, isDir bool) (string, *contract.Failure) {
	segments := strings.Split(rel, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包含不安全的路径").WithDetail(rel)
		}
		if len(segment) > MaxSegmentLengthArc {
			return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包里的名称过长").WithDetail(rel)
		}
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包里的名称以点或空格结尾").WithDetail(rel)
		}
		if isWindowsReserved(segment) {
			return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包里的名称是 Windows 保留名").WithDetail(rel)
		}
	}
	key := strings.ToLower(rel)
	kind := "file"
	if isDir {
		kind = "dir"
	}
	if previous, ok := s.seen[key]; ok {
		// Repeated directory entries are harmless; anything else is not.
		if !(previous == "dir" && kind == "dir") {
			return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包里有两个只差大小写的条目").WithDetail(rel)
		}
	} else {
		s.seen[key] = kind
	}
	return rel, nil
}

// resolve turns a validated relative path into an absolute one once its
// parents exist on disk; ResolveUnder rejects escaping and reparse parents.
func (s *extractState) resolve(rel string) (string, *contract.Failure) {
	parent := filepath.Dir(filepath.Join(s.root, filepath.FromSlash(rel)))
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", contract.Fail(contract.CodeIOFailure, "无法创建目录")
	}
	abs, err := ResolveUnder(s.root, rel)
	if err != nil {
		return "", contract.Fail(contract.CodeUnsafeArchive, "压缩包含不安全的路径").WithDetail(rel)
	}
	return abs, nil
}

func (s *extractState) makeDir(rel string) *contract.Failure {
	abs, f := s.resolve(rel)
	if f != nil {
		return f
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建目录")
	}
	return nil
}

// writeFile streams one regular file, counting real bytes against the cap.
func (s *extractState) writeFile(abs string, body io.Reader, declared int64) *contract.Failure {
	if declared > 0 && s.out.Bytes+declared > s.options.MaxBytes {
		return contract.Fail(contract.CodeTooLarge, "解开后的内容超过 200 MiB").WithLimit(s.out.Bytes+declared, s.options.MaxBytes)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法创建目录")
	}
	file, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return contract.Fail(contract.CodeIOFailure, "无法写入文件")
	}
	defer file.Close()
	remaining := s.options.MaxBytes - s.out.Bytes
	written, err := io.Copy(file, io.LimitReader(body, remaining+1))
	if err != nil {
		return contract.Fail(contract.CodeUnsafeArchive, "压缩包已损坏")
	}
	if written > remaining {
		return contract.Fail(contract.CodeTooLarge, "解开后的内容超过 200 MiB").WithLimit(s.out.Bytes+written, s.options.MaxBytes)
	}
	s.out.Bytes += written
	s.out.Files++
	return nil
}

func isWindowsReserved(segment string) bool {
	base := segment
	if dot := strings.IndexByte(base, '.'); dot >= 0 {
		base = base[:dot]
	}
	return windowsReserved[strings.ToLower(base)]
}
