package mutation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Provenance lives in <vault>/.mile-kb/provenance.json (docs/数据库设计.md §3.4).
// It is a Git-tracked file, so serialization is stable: records sorted by
// copyPath, fixed key order, two-space indent and a trailing newline.

// ProvenanceTrackedPrefix is the only destination that records a relationship.
const ProvenanceTrackedPrefix = "projects/"

// ShouldTrackProvenance mirrors the legacy case-insensitive prefix check.
func ShouldTrackProvenance(targetPath string) bool {
	return strings.HasPrefix(strings.ToLower(targetPath), ProvenanceTrackedPrefix)
}

// ProvenanceRecord is one copy -> source edge.
type ProvenanceRecord struct {
	CopyPath     string `json:"copyPath"`
	SourcePath   string `json:"sourcePath"`
	BaseRevision int64  `json:"baseRevision"`
	BaseHash     string `json:"baseHash"`
	CopiedAt     string `json:"copiedAt"`

	// Unknown holds fields written by other versions so a read-write cycle
	// does not drop them (docs/数据库设计.md §3.0.1).
	Unknown map[string]json.RawMessage `json:"-"`
}

type provenanceFile struct {
	Version int                `json:"version"`
	Records []ProvenanceRecord `json:"records"`

	Unknown map[string]json.RawMessage `json:"-"`
}

// ProvenanceStore is one vault's relationship table.
type ProvenanceStore struct {
	path    string
	mu      sync.Mutex
	records map[string]ProvenanceRecord
	// topUnknown keeps unknown top-level fields of the file.
	topUnknown map[string]json.RawMessage
}

// OpenProvenance loads (or creates) the table for a vault root.
func OpenProvenance(root string) (*ProvenanceStore, error) {
	store := &ProvenanceStore{
		path:       filepath.Join(root, systemDir, "provenance.json"),
		records:    map[string]ProvenanceRecord{},
		topUnknown: map[string]json.RawMessage{},
	}
	body, err := os.ReadFile(store.path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	// Damaged JSON degrades to an empty table: losing the baseline must never
	// stop the app from starting (legacy parseProvenance).
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return store, nil
	}
	for key, value := range raw {
		if key == "version" || key == "records" {
			continue
		}
		store.topUnknown[key] = value
	}
	var records []json.RawMessage
	if list, ok := raw["records"]; ok {
		_ = json.Unmarshal(list, &records)
	}
	for _, item := range records {
		var fields map[string]json.RawMessage
		if json.Unmarshal(item, &fields) != nil {
			continue
		}
		record := ProvenanceRecord{Unknown: map[string]json.RawMessage{}}
		for key, value := range fields {
			switch key {
			case "copyPath":
				_ = json.Unmarshal(value, &record.CopyPath)
			case "sourcePath":
				_ = json.Unmarshal(value, &record.SourcePath)
			case "baseRevision":
				_ = json.Unmarshal(value, &record.BaseRevision)
			case "baseHash":
				_ = json.Unmarshal(value, &record.BaseHash)
			case "copiedAt":
				_ = json.Unmarshal(value, &record.CopiedAt)
			default:
				record.Unknown[key] = value
			}
		}
		if record.CopyPath == "" || record.SourcePath == "" {
			continue
		}
		store.records[record.CopyPath] = record
	}
	return store, nil
}

// Get returns one record.
func (s *ProvenanceStore) Get(copyPath string) (ProvenanceRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[copyPath]
	return record, ok
}

// All returns records sorted by copyPath.
func (s *ProvenanceStore) All() []ProvenanceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ProvenanceRecord, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].CopyPath) < strings.ToLower(out[j].CopyPath)
	})
	return out
}

// Record upserts a relationship (one per copy path) and persists.
func (s *ProvenanceStore) Record(copyPath, sourcePath string, baseRevision int64, baseHash string) error {
	s.mu.Lock()
	record := ProvenanceRecord{
		CopyPath: copyPath, SourcePath: sourcePath, BaseRevision: baseRevision,
		BaseHash: baseHash, CopiedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if old, ok := s.records[copyPath]; ok {
		record.Unknown = old.Unknown
		record.CopiedAt = old.CopiedAt
	}
	s.records[copyPath] = record
	s.mu.Unlock()
	return s.persist()
}

// UpdateBaseline replaces the baseline after a sync; copiedAt records the sync
// time because that is when the relationship was last confirmed.
func (s *ProvenanceStore) UpdateBaseline(copyPath string, baseRevision int64, baseHash string) error {
	s.mu.Lock()
	record, ok := s.records[copyPath]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	record.BaseRevision = baseRevision
	record.BaseHash = baseHash
	record.CopiedAt = time.Now().UTC().Format(time.RFC3339Nano)
	s.records[copyPath] = record
	s.mu.Unlock()
	return s.persist()
}

// Forget drops a relationship.
func (s *ProvenanceStore) Forget(copyPath string) error {
	s.mu.Lock()
	_, ok := s.records[copyPath]
	delete(s.records, copyPath)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return s.persist()
}

// Serialize produces the stable JSON text.
func (s *ProvenanceStore) Serialize() string {
	records := s.All()
	var b strings.Builder
	b.WriteString("{\n  \"version\": 1,\n  \"records\": ")
	if len(records) == 0 {
		b.WriteString("[]")
	} else {
		b.WriteString("[\n")
		for i, record := range records {
			b.WriteString("    {\n")
			b.WriteString("      \"copyPath\": " + quoteJSON(record.CopyPath) + ",\n")
			b.WriteString("      \"sourcePath\": " + quoteJSON(record.SourcePath) + ",\n")
			b.WriteString("      \"baseRevision\": " + itoa64(record.BaseRevision) + ",\n")
			b.WriteString("      \"baseHash\": " + quoteJSON(record.BaseHash) + ",\n")
			b.WriteString("      \"copiedAt\": " + quoteJSON(record.CopiedAt))
			writeUnknown(&b, record.Unknown, "      ")
			b.WriteString("\n    }")
			if i != len(records)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString("  ]")
	}
	writeTopUnknown(&b, s.topUnknown)
	b.WriteString("\n}\n")
	return b.String()
}

func writeUnknown(b *strings.Builder, fields map[string]json.RawMessage, indent string) {
	keys := sortedRawKeys(fields)
	for _, key := range keys {
		b.WriteString(",\n" + indent + "\"" + key + "\": " + string(fields[key]))
	}
}

func writeTopUnknown(b *strings.Builder, fields map[string]json.RawMessage) {
	keys := sortedRawKeys(fields)
	for _, key := range keys {
		b.WriteString(",\n  \"" + key + "\": " + string(fields[key]))
	}
}

func sortedRawKeys(fields map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *ProvenanceStore) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(s.Serialize()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// quoteJSON matches JSON.stringify: HTML characters stay literal so a
// read-write cycle does not change bytes.
func quoteJSON(value string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return `""`
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

// Assess computes the seven-state relationship (shared/provenance.ts). Nil
// hashes mean the file is missing.
func Assess(record ProvenanceRecord, sourceHash, copyHash *string) (state string, canSync bool, reason string) {
	if record.BaseHash == "" {
		return "unknown", false, "缺少复制时的基线信息，无法判断是否有更新"
	}
	if copyHash == nil {
		return "copy-missing", false, "副本已不存在"
	}
	if sourceHash == nil {
		return "source-missing", false, "来源已被删除或移走，副本成为孤儿"
	}
	sourceChanged := *sourceHash != record.BaseHash
	copyChanged := *copyHash != record.BaseHash
	switch {
	case !sourceChanged && !copyChanged:
		return "up-to-date", false, "与来源一致"
	case sourceChanged && !copyChanged:
		return "source-changed", true, "来源已更新，副本未改动，可以安全地从来源更新"
	case !sourceChanged && copyChanged:
		return "copy-modified", false, "副本有本地改动，来源未变；从来源更新会覆盖本地改动"
	default:
		return "both-changed", false, "来源与副本都有改动，需要人工判断如何合并"
	}
}
