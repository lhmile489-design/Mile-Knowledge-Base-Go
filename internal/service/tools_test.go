package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lhmile489-design/Mile-Knowledge-Base-Go/internal/ai"
)

type fakeGate struct {
	denyWrite, denyMutation, denyFresh bool
	writes, mutations                  []string
}

func (g *fakeGate) AllowWrite(_ context.Context, _ string, path string) error {
	if g.denyWrite {
		return errors.New("未获批准")
	}
	g.writes = append(g.writes, path)
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
	if !strings.Contains(missing, "不存在") {
		t.Fatalf("missing old_text: %s", missing)
	}
	ambiguous, _ := runner.Run(context.Background(), call("edit_note", `{"path":"raws/a.md","old_text":"a","new_text":"x"}`))
	if !strings.Contains(ambiguous, "多次") {
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
