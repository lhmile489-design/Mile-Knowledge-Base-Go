package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/skills"
)

// M84 and the log writes of T08 (FR-SKM-LOG, 数据库设计 4.6).

var logSeq int64

func (s *SkillService) logStore() *skills.LogStore {
	return &skills.LogStore{Path: filepath.Join(s.app.DataDir, "skills", "log.jsonl")}
}

func (s *SkillService) backupExists(id string) bool {
	_, err := os.Stat(filepath.Join(s.app.DataDir, "skills", "backups", id, "meta.json"))
	return err == nil
}

// logOperation appends one record; logging never fails an operation.
func (s *SkillService) logOperation(op, skill, toolID, location, mode, result, backupID, errText string) {
	seq := atomic.AddInt64(&logSeq, 1)
	record := contract.SkillLogRecord{
		ID:       fmt.Sprintf("l-%d-%d", s.now().UnixNano(), seq),
		At:       s.now().UTC().Format(time.RFC3339),
		Op:       op,
		Skill:    skill,
		ToolID:   toolID,
		Location: location,
		Mode:     mode,
		Result:   result,
		BackupID: backupID,
		Error:    errText,
	}
	_ = s.logStore().Append(record)
}

// Log is M84: the newest records, newest first, with paths redacted for the
// page and restorable recomputed from the backup area.
func (s *SkillService) Log(ctx context.Context, limit int) contract.SkillLogResult {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return contract.SkillLogResult{Failure: contract.Fail(contract.CodeInvalidInput, "数量不合法")}
	}
	detector := s.detector()
	records := s.logStore().Read(limit, s.backupExists)
	for i := range records {
		records[i].Location = redactToolPath(records[i].Location, detector)
	}
	if records == nil {
		records = []contract.SkillLogRecord{}
	}
	return contract.SkillLogResult{Records: records}
}

// logDistributeRecords writes one line per performed or failed target.
func (s *SkillService) logDistributeRecords(skillName string, items []contract.SkillOpItem, locations map[string]string) {
	for _, item := range items {
		if item.Result == "skipped" {
			continue
		}
		op := "distribute"
		if item.BackupID != "" {
			op = "overwrite"
		}
		result := "ok"
		if item.Result == "failed" {
			result = "failed"
		}
		mode := ""
		switch item.Result {
		case "linked":
			mode = "link"
		case "copied":
			mode = "copy"
		}
		s.logOperation(op, skillName, item.ToolID, locations[item.TargetID], mode, result, item.BackupID, item.Detail)
	}
}
