package skills

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// skills/log.jsonl (数据库设计 4.6, FR-SKM-LOG): one line per operation, and
// one more for every restore. Restorable is not stored — it is derived from
// whether the backup still exists (Q-SKM-3 clean removes it).

// LogStore appends and reads the operation log.
type LogStore struct {
	Path string
}

// Append writes one record as a JSON line.
func (l *LogStore) Append(record contract.SkillLogRecord) error {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o755); err != nil {
		return err
	}
	body, err := json.Marshal(record)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(l.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(body, '\n'))
	return err
}

// Read returns the newest `limit` records, newest first. Bad lines and lines
// from unknown versions are skipped. restorable recomputes whether a backup
// id can still be restored.
func (l *LogStore) Read(limit int, restorable func(backupID string) bool) []contract.SkillLogRecord {
	if limit <= 0 {
		limit = 50
	}
	f, err := os.Open(l.Path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var records []contract.SkillLogRecord
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record contract.SkillLogRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.ID == "" || record.Op == "" {
			continue
		}
		if record.BackupID != "" {
			record.Restorable = restorable != nil && restorable(record.BackupID)
		} else {
			record.Restorable = false
		}
		records = append(records, record)
	}
	// A read error ends the tail; the page shows what was readable.
	_ = scanner.Err()
	// Newest first; the file is append-only so the tail is the newest.
	out := make([]contract.SkillLogRecord, 0, limit)
	for i := len(records) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, records[i])
	}
	return out
}
