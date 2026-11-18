package mutation

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
)

func auditRecord(id, vault, plan, status string) contract.ActionRecord {
	return contract.ActionRecord{
		ID: id, PlanID: plan, VaultID: vault, Origin: "user", Kind: contract.MutCreate,
		Status: status, CreatedAt: "2026-01-01T00:00:0" + id[len(id)-1:] + "Z",
	}
}

func TestAuditAppendReloadAndUpsert(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	ledger, err := OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		id := "r" + strings.Repeat("0", 2) + string(rune('a'+i%26)) + string(rune('0'+i/26))
		if err := ledger.Append(auditRecord(id, "v1", "p1", "completed")); err != nil {
			t.Fatal(err)
		}
	}
	reloaded, err := OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Len(); got != 100 {
		t.Fatalf("reload len = %d", got)
	}

	// The same id updates in place instead of duplicating.
	if err := reloaded.Append(auditRecord("dup", "v1", "p1", "planned")); err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Append(auditRecord("dup", "v1", "p1", "reverted")); err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Len(); got != 101 {
		t.Fatalf("upsert len = %d", got)
	}
	latest := reloaded.List("v1", "p1", 1000)
	if latest[0].ID != "dup" || latest[0].Status != "reverted" {
		t.Fatalf("latest = %+v", latest[0])
	}
}

func TestAuditSkipsDamagedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	body := `{"id":"ok","planId":"p","vaultId":"v","origin":"user","kind":"create","status":"completed","createdAt":"2026-01-01T00:00:00Z"}
{not json at all
{"id":"","planId":"p","vaultId":"v","status":"completed"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ledger, err := OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Len() != 1 || ledger.Skipped() != 2 {
		t.Fatalf("len=%d skipped=%d", ledger.Len(), ledger.Skipped())
	}
}

func TestAuditCapsAtTwoThousand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	ledger, err := OpenAudit(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < AuditMaxRecords+5; i++ {
		record := auditRecord(fmt.Sprintf("id-%04d", i), "v1", "p1", "completed")
		record.CreatedAt = fmt.Sprintf("2026-01-01T00:%02d:%02d.%04dZ", i/60%60, i%60, i)
		if err := ledger.Append(record); err != nil {
			t.Fatal(err)
		}
	}
	if got := ledger.Len(); got != AuditMaxRecords {
		t.Fatalf("len = %d", got)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines != AuditMaxRecords {
		t.Fatalf("file lines = %d", lines)
	}
	// List clamps a request to 1000 rows, so the oldest assertion uses that
	// window: with 2000 kept, the 1000-row page ends at id-1005.
	records := ledger.List("", "", AuditMaxRecords)
	if len(records) != 1000 {
		t.Fatalf("list len = %d", len(records))
	}
	if records[0].ID != "id-2004" {
		t.Errorf("newest = %s", records[0].ID)
	}
	if records[len(records)-1].ID != "id-1005" {
		t.Errorf("oldest in window = %s", records[len(records)-1].ID)
	}
}

func TestAuditFiltersAndLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	ledger, _ := OpenAudit(path)
	_ = ledger.Append(auditRecord("a1", "v1", "p1", "completed"))
	_ = ledger.Append(auditRecord("a2", "v2", "p1", "completed"))
	_ = ledger.Append(auditRecord("a3", "v1", "p2", "completed"))
	_ = ledger.Append(auditRecord("a4", "v1", "p1", "failed"))

	if got := ledger.List("v1", "", 100); len(got) != 3 {
		t.Errorf("vault filter = %d", len(got))
	}
	if got := ledger.List("v1", "p1", 100); len(got) != 2 {
		t.Errorf("plan filter = %d", len(got))
	}
	if got := ledger.List("", "", 2); len(got) != 2 {
		t.Errorf("limit = %d", len(got))
	}
}
