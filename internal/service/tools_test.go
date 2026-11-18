package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/contract"
	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/policy"
)

type fakeGate struct {
	denyWrite, denyMutation, denyFresh bool
	writes, mutations                  []string
	previews                           []*contract.WritePreview
}

func (g *fakeGate) AllowWrite(_ context.Context, _ string, path string, preview *contract.WritePreview) error {
	if g.denyWrite {
		return errors.New("未获批准")
	}
	g.writes = append(g.writes, path)
	g.previews = append(g.previews, preview)
	return nil
}

func (g *fakeGate) AllowMutation(_ context.Context, _ string, path string) error {
	if g.denyMutation {
		return errors.New("未获批准")
	}
	g.mutations = append(g.mutations, path)
	return nil
}

func (g *fakeGate) ConsumeFreshMutation(_ context.Context, _ string, _ string) error {
	if g.denyFresh {
		return errors.New("永久删除需要本次单独确认")
	}
	return nil
}

func runnerFor(t *testing.T, gate ApprovalGate) (*AgentToolRunner, string, string) {
	t.Helper()
	app, vaultID, root := mutationTestApp(t)
	return NewAgentToolRunner(app, vaultID, "curator", "sess_test", gate), root, vaultID
}

func call(name, args string) ai.CompletedToolCall {
	return ai.CompletedToolCall{ID: "call_1", Name: name, Arguments: args}
}

func TestWriteNoteAllowed(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	out, err := runner.Run(context.Background(), call("write_note", `{"path":"raws/新笔记.md","content":"# 标题\n正文"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已写入 raws/新笔记.md") {
		t.Fatalf("output: %s", out)
	}
	if len(gate.writes) != 1 {
		t.Fatalf("gate calls: %v", gate.writes)
	}
	body, err := os.ReadFile(filepath.Join(root, "raws", "新笔记.md"))
	if err != nil || string(body) != "# 标题\n正文" {
		t.Fatalf("file: %q %v", body, err)
	}
}

func TestWriteNoteDeniedByPolicy(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	out, _ := runner.Run(context.Background(), call("write_note", `{"path":".mile-kb/settings/x.json","content":"{}"}`))
	if !strings.HasPrefix(out, "被拒绝：") {
		t.Fatalf("output: %s", out)
	}
	if len(gate.writes) != 0 {
		t.Fatalf("the gate must not be reached for a system path: %v", gate.writes)
	}
	if _, err := os.Stat(filepath.Join(root, ".mile-kb", "settings", "x.json")); !os.IsNotExist(err) {
		t.Fatalf("system file was written")
	}
}

func TestWriteNoteEscapeRejected(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	for _, path := range []string{"../outside.md", "/abs.md", "C:/x.md"} {
		out, _ := runner.Run(context.Background(), call("write_note", `{"path":"`+path+`","content":"x"}`))
		if !strings.Contains(out, "参数不合法") && !strings.HasPrefix(out, "被拒绝：") {
			t.Errorf("%s: %s", path, out)
		}
	}
}

func TestEditNoteRules(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	if _, err := runner.Run(context.Background(), call("write_note", `{"path":"raws/a.md","content":"alpha beta gamma"}`)); err != nil {
		t.Fatal(err)
	}

	missing, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/a.md","old_text":"delta","new_text":"x"}`))
	if !strings.Contains(missing, "找不到") {
		t.Fatalf("missing old_text: %s", missing)
	}
	ambiguous, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/a.md","old_text":"a","new_text":"x"}`))
	if !strings.Contains(ambiguous, "位置不唯一") {
		t.Fatalf("ambiguous old_text: %s", ambiguous)
	}
	ok, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/a.md","old_text":"beta","new_text":"B"}`))
	if !strings.Contains(ok, "已更新") {
		t.Fatalf("edit: %s", ok)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if string(body) != "alpha B gamma" {
		t.Fatalf("file: %q", body)
	}
}

func TestEditNoteDeniedByGate(t *testing.T) {
	runner, root, _ := runnerFor(t, &fakeGate{denyWrite: true})
	if err := os.WriteFile(filepath.Join(root, "raws", "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/a.md","old_text":"x","new_text":"y"}`))
	if !strings.HasPrefix(out, "被拒绝：") {
		t.Fatalf("output: %s", out)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "a.md"))
	if string(body) != "x" {
		t.Fatalf("denied edit changed the file: %q", body)
	}
}

func TestReadNotePaging(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	long := strings.Repeat("字", 25000)
	if _, err := runner.Run(context.Background(), call("write_note", `{"path":"raws/long.md","content":"`+long+`"}`)); err != nil {
		t.Fatal(err)
	}
	first, _ := runner.Run(context.Background(), call("read_note", `{"path":"raws/long.md"}`))
	if !strings.Contains(first, "第 1-20000 个字符，共 25000 个") || !strings.Contains(first, "offset=20000") {
		t.Fatalf("first page header: %s", first[:120])
	}
	second, _ := runner.Run(context.Background(), call("read_note", `{"path":"raws/long.md","offset":20000}`))
	if !strings.Contains(second, "第 20001-25000 个字符") {
		t.Fatalf("second page: %s", second[:120])
	}
	outOfRange, _ := runner.Run(context.Background(), call("read_note", `{"path":"raws/long.md","offset":30000}`))
	if !strings.Contains(outOfRange, "offset 越界") {
		t.Fatalf("out of range: %s", outOfRange)
	}
}

func TestListDirHidesSystem(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("list_dir", `{"path":""}`))
	if strings.Contains(out, ".mile-kb") {
		t.Fatalf("system area listed: %s", out)
	}
	if !strings.Contains(out, "[目录] raws") {
		t.Fatalf("raws missing: %s", out)
	}
}

func TestMutateCreatesDirectory(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	out, _ := runner.Run(context.Background(), call("mutate_notes", `{"kind":"createDirectory","target":"raws/新目录"}`))
	if !strings.Contains(out, "完成 1 项") {
		t.Fatalf("output: %s", out)
	}
	info, err := os.Stat(filepath.Join(root, "raws", "新目录"))
	if err != nil || !info.IsDir() {
		t.Fatalf("directory missing: %v", err)
	}
	if len(gate.mutations) != 1 {
		t.Fatalf("mutation gate: %v", gate.mutations)
	}
}

func TestMutateDeniedByPolicy(t *testing.T) {
	runner, root, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("mutate_notes", `{"kind":"move","source":"raws/旧名.md","target":".mile-kb/settings/旧名.md"}`))
	if !strings.HasPrefix(out, "被拒绝：") {
		t.Fatalf("output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "raws", "旧名.md")); err != nil {
		t.Fatalf("source moved anyway: %v", err)
	}
}

func TestMutatePermanentDeleteNeedsFreshApproval(t *testing.T) {
	gate := &fakeGate{denyFresh: true}
	runner, root, _ := runnerFor(t, gate)
	out, _ := runner.Run(context.Background(), call("mutate_notes", `{"kind":"permanentDelete","source":"raws/旧名.md"}`))
	if !strings.HasPrefix(out, "被拒绝：") || !strings.Contains(out, "永久删除") {
		t.Fatalf("output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "raws", "旧名.md")); err != nil {
		t.Fatalf("file deleted: %v", err)
	}
}

func TestHandoffNotReady(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("handoff_task", `{"owner":"scout","goal":"x"}`))
	if !strings.Contains(out, "尚未就绪") {
		t.Fatalf("output: %s", out)
	}
}

func TestSetAliases(t *testing.T) {
	runner, root, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("set_aliases", `{"aliases":{"raws":"暂存"}}`))
	if !strings.Contains(out, "已更新 1 条") {
		t.Fatalf("output: %s", out)
	}
	body, err := os.ReadFile(filepath.Join(root, ".mile-kb", "settings", "aliases.json"))
	if err != nil || !strings.Contains(string(body), "暂存") {
		t.Fatalf("aliases: %q %v", body, err)
	}
}

func TestUnknownToolListsNames(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("delete_everything", `{}`))
	if !strings.Contains(out, "未知工具") || !strings.Contains(out, "search_notes") {
		t.Fatalf("output: %s", out)
	}
}

func TestSchemasHaveAllTools(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	schemas := runner.Schemas()
	if len(schemas) != 8 {
		t.Fatalf("schemas: %d", len(schemas))
	}
	joined := ""
	for _, schema := range schemas {
		joined += string(schema)
	}
	for _, name := range []string{"search_notes", "read_note", "list_dir", "write_note", "edit_note", "mutate_notes", "handoff_task", "set_aliases"} {
		if !strings.Contains(joined, `"`+name+`"`) {
			t.Errorf("schema missing %s", name)
		}
	}
}

func TestWriteNotePreview(t *testing.T) {
	gate := &fakeGate{}
	runner, _, _ := runnerFor(t, gate)
	if _, err := runner.Run(context.Background(), call("write_note", `{"path":"raws/p.md","content":"line1\nline2"}`)); err != nil {
		t.Fatal(err)
	}
	if len(gate.previews) != 1 || gate.previews[0] == nil {
		t.Fatalf("created preview missing: %+v", gate.previews)
	}
	if created := gate.previews[0]; created.Kind != "created" || len(created.Lines) != 2 {
		t.Fatalf("created preview: %+v", created)
	}

	if _, err := runner.Run(context.Background(), call("write_note", `{"path":"raws/p.md","content":"line1\nLINE2"}`)); err != nil {
		t.Fatal(err)
	}
	modified := gate.previews[1]
	if modified == nil || modified.Kind != "modified" {
		t.Fatalf("modified preview: %+v", modified)
	}
	hasRemoved, hasAdded := false, false
	for _, line := range modified.Lines {
		if line.Kind == "removed" && line.Text == "line2" {
			hasRemoved = true
		}
		if line.Kind == "added" && line.Text == "LINE2" {
			hasAdded = true
		}
	}
	if !hasRemoved || !hasAdded {
		t.Fatalf("diff lines: %+v", modified.Lines)
	}
}

func TestEditNotePreview(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	writeServiceFile(t, root, "raws/e.md", "alpha\nbeta\n")
	out, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/e.md","old_text":"beta","new_text":"BETA"}`))
	if !strings.Contains(out, "已更新") {
		t.Fatalf("edit: %s", out)
	}
	if len(gate.previews) != 1 || gate.previews[0] == nil || gate.previews[0].Kind != "modified" {
		t.Fatalf("preview: %+v", gate.previews)
	}
}

func TestEditNoteValidationMessages(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	writeServiceFile(t, root, "raws/m.md", "aaa")
	cases := []struct{ old, newText, want string }{
		{"", "x", "old_text 不能为空"},
		{"aa", "aa", "old_text 与 new_text 相同"},
		{"aa", "b", "出现了 2 处"},
		{"zzz", "b", "找不到"},
	}
	for _, c := range cases {
		body := `{"path":"raws/m.md","old_text":"` + c.old + `","new_text":"` + c.newText + `"}`
		out, _ := runner.Run(context.Background(), call("edit_note", body))
		if !strings.Contains(out, c.want) {
			t.Errorf("%+v: %s", c, out)
		}
	}
	if len(gate.writes) != 0 {
		t.Fatalf("gate must not be reached for a rejected edit: %v", gate.writes)
	}
}

func TestEditNoteAcceptsCrlfOldText(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	if err := os.WriteFile(filepath.Join(root, "raws", "crlf.md"), []byte("a\r\nb\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/crlf.md","old_text":"a\r\nb","new_text":"a\nB"}`))
	if !strings.Contains(out, "已更新") {
		t.Fatalf("edit: %s", out)
	}
	body, _ := os.ReadFile(filepath.Join(root, "raws", "crlf.md"))
	if string(body) != "a\r\nB\r\n" {
		t.Fatalf("body: %q", body)
	}
}

func TestSystemAreaShortNameRejected(t *testing.T) {
	gate := &fakeGate{}
	runner, root, _ := runnerFor(t, gate)
	if err := os.MkdirAll(filepath.Join(root, ".mile-kb", "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	short := shortNameFor(t, root, ".mile-kb")
	if short == "" {
		t.Skip("this volume has no 8.3 short names")
	}
	if decision := policy.CanWrite("curator", short+"/settings/evil.json"); !decision.Allowed {
		t.Skipf("the spelling gate already blocks %q", short)
	}
	out, _ := runner.Run(context.Background(), call("write_note", `{"path":"`+short+`/settings/evil.json","content":"{}"}`))
	if !strings.HasPrefix(out, "被拒绝：") {
		t.Fatalf("short-name escape not rejected: %s", out)
	}
	if len(gate.writes) != 0 {
		t.Fatalf("gate reached: %v", gate.writes)
	}
	if _, err := os.Stat(filepath.Join(root, ".mile-kb", "settings", "evil.json")); !os.IsNotExist(err) {
		t.Fatalf("system file was written")
	}
}

// shortNameFor reads the 8.3 alias of one root entry from dir /x.
func shortNameFor(t *testing.T, root, long string) string {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "dir", "/x", "/a", root).Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		hasLong := false
		for _, field := range fields {
			if strings.EqualFold(field, long) {
				hasLong = true
			}
		}
		if !hasLong {
			continue
		}
		for _, field := range fields {
			if strings.Contains(field, "~") && !strings.EqualFold(field, long) {
				return field
			}
		}
	}
	return ""
}

func TestMutateRejectsNonSchemaKinds(t *testing.T) {
	gate := &fakeGate{}
	runner, _, _ := runnerFor(t, gate)
	for _, body := range []string{`{"kind":"create","target":"raws/x.md"}`, `{"kind":"edit","source":"raws/旧名.md"}`} {
		out, _ := runner.Run(context.Background(), call("mutate_notes", body))
		if !strings.Contains(out, "参数不合法") {
			t.Errorf("%s: %s", body, out)
		}
	}
	if len(gate.mutations) != 0 {
		t.Fatalf("gate reached: %v", gate.mutations)
	}
	moved, _ := runner.Run(context.Background(), call("mutate_notes", `{"kind":"move","source":"raws/旧名.md"}`))
	if !strings.Contains(moved, "参数不合法") {
		t.Fatalf("missing target: %s", moved)
	}
	deleted, _ := runner.Run(context.Background(), call("mutate_notes", `{"kind":"permanentDelete"}`))
	if !strings.Contains(deleted, "参数不合法") {
		t.Fatalf("missing source: %s", deleted)
	}
}

func TestSetAliasesRejectsTraversal(t *testing.T) {
	runner, _, _ := runnerFor(t, &fakeGate{})
	out, _ := runner.Run(context.Background(), call("set_aliases", `{"aliases":{"../evil":"x"}}`))
	if !strings.Contains(out, "设置别名失败") {
		t.Fatalf("output: %s", out)
	}
}
