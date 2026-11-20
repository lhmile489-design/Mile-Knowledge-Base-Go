package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

// M4 T08: skills/log.jsonl appends, bad lines are skipped, restorable is
// derived from the backup area (数据库设计 4.6; FR-SKM-LOG).

func TestLogAppendAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skills", "log.jsonl")
	store := &LogStore{Path: path}
	for _, skill := range []string{"a", "b", "c"} {
		if err := store.Append(contract.SkillLogRecord{
			ID: "l-" + skill, At: "2026-11-20T09:00:00Z", Op: "distribute",
			Skill: skill, ToolID: "claude-code", Location: `C:\t\` + skill, Mode: "link", Result: "ok",
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A corrupt line in the middle must not hide the rest.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{not json\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := store.Append(contract.SkillLogRecord{
		ID: "l-d", At: "2026-11-20T09:01:00Z", Op: "uninstall", Skill: "d",
		Location: "C:\\t\\d", Result: "ok", BackupID: "b-aaaaaaaaaaaa",
	}); err != nil {
		t.Fatal(err)
	}

	records := store.Read(2, func(id string) bool { return id == "b-aaaaaaaaaaaa" })
	if len(records) != 2 || records[0].ID != "l-d" || records[1].ID != "l-c" {
		t.Fatalf("records = %+v", records)
	}
	if !records[0].Restorable {
		t.Fatal("restorable should be derived from the backup callback")
	}
	if records[1].Restorable {
		t.Fatal("record without backup must not be restorable")
	}
	all := store.Read(0, func(string) bool { return false })
	if len(all) != 4 || all[0].ID != "l-d" {
		t.Fatalf("all = %+v", all)
	}
}

func TestLogReadMissingFile(t *testing.T) {
	store := &LogStore{Path: filepath.Join(t.TempDir(), "nope", "log.jsonl")}
	if records := store.Read(10, nil); records != nil {
		t.Fatalf("records = %+v", records)
	}
}
