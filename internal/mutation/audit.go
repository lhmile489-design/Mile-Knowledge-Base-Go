package mutation

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// AuditMaxRecords caps the ledger (docs/数据库设计.md §4.3). The legacy app
// rewrote one JSON file per append; the new format appends JSON lines and only
// rewrites once when the cap is crossed.
const AuditMaxRecords = 2000

// AuditLedger stores action records for every vault in one JSONL file.
type AuditLedger struct {
	path string

	mu      sync.Mutex
	records map[string]contract.ActionRecord
	// skipped counts unreadable lines seen while loading; surfaced in logs.
	skipped int
}

// OpenAudit loads the ledger, skipping damaged lines instead of discarding the
// whole file (the legacy app lost everything on a bad byte).
func OpenAudit(path string) (*AuditLedger, error) {
	ledger := &AuditLedger{path: path, records: map[string]contract.ActionRecord{}}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ledger, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record contract.ActionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil || record.ID == "" {
			ledger.skipped++
			continue
		}
		ledger.records[record.ID] = record
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ledger, nil
}

// Path is where the ledger lives.
func (l *AuditLedger) Path() string { return l.path }

// Skipped reports how many damaged lines were ignored while loading.
func (l *AuditLedger) Skipped() int { return l.skipped }

// Len reports the number of records currently held.
func (l *AuditLedger) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.records)
}

// Append writes records (upsert by id: a later status replaces the earlier one)
// and rewrites the file only when the cap is exceeded.
func (l *AuditLedger) Append(records ...contract.ActionRecord) error {
	if len(records) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	var b strings.Builder
	for _, record := range records {
		if record.ID == "" {
			continue
		}
		l.records[record.ID] = record
		line, err := json.Marshal(record)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := l.appendLocked(b.String()); err != nil {
		return err
	}
	if len(l.records) > AuditMaxRecords {
		return l.rewriteLocked()
	}
	return nil
}

func (l *AuditLedger) appendLocked(payload string) error {
	if payload == "" {
		return nil
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(payload); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// rewriteLocked keeps the newest AuditMaxRecords records and replaces the file
// atomically.
func (l *AuditLedger) rewriteLocked() error {
	records := l.sortedLocked()
	if len(records) > AuditMaxRecords {
		records = records[len(records)-AuditMaxRecords:]
	}
	var b strings.Builder
	for _, record := range records {
		line, err := json.Marshal(record)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, l.path); err != nil {
		return err
	}
	kept := map[string]contract.ActionRecord{}
	for _, record := range records {
		kept[record.ID] = record
	}
	l.records = kept
	return nil
}

// sortedLocked orders records oldest first by createdAt then id.
func (l *AuditLedger) sortedLocked() []contract.ActionRecord {
	out := make([]contract.ActionRecord, 0, len(l.records))
	for _, record := range l.records {
		out = append(out, record)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt < out[j].CreatedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// List returns records newest first, optionally filtered by vault and plan.
func (l *AuditLedger) List(vaultID, planID string, limit int) []contract.ActionRecord {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	l.mu.Lock()
	records := l.sortedLocked()
	l.mu.Unlock()

	out := make([]contract.ActionRecord, 0, limit)
	for i := len(records) - 1; i >= 0 && len(out) < limit; i-- {
		record := records[i]
		if vaultID != "" && record.VaultID != vaultID {
			continue
		}
		if planID != "" && record.PlanID != planID {
			continue
		}
		out = append(out, record)
	}
	return out
}

// UpdateStatus rewrites one record's status (used when a transaction is
// restored). Missing ids are ignored.
func (l *AuditLedger) UpdateStatus(ids []string, status string) ([]contract.ActionRecord, error) {
	l.mu.Lock()
	updated := make([]contract.ActionRecord, 0, len(ids))
	for _, id := range ids {
		record, ok := l.records[id]
		if !ok {
			continue
		}
		record.Status = status
		l.records[id] = record
		updated = append(updated, record)
	}
	l.mu.Unlock()
	if len(updated) == 0 {
		return nil, nil
	}
	if err := l.Append(updated...); err != nil {
		return nil, err
	}
	return updated, nil
}
