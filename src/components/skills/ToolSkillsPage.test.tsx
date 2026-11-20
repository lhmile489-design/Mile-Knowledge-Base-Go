import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const uninstallPreview = vi.fn();
const uninstall = vi.fn();
const adopt = vi.fn();
const reveal = vi.fn();
const distribute = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      uninstallPreview: (...args: unknown[]) => uninstallPreview(...args),
      uninstall: (...args: unknown[]) => uninstall(...args),
      adopt: (...args: unknown[]) => adopt(...args),
      reveal: (...args: unknown[]) => reveal(...args),
      distribute: (...args: unknown[]) => distribute(...args),
    },
    settingsApi: { patch: vi.fn(), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { ToolSkillsPage } = await import("./ToolSkillsPage");

const tools = [
  { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
  { id: "claude-code", name: "Claude Code", iconKey: "claude-code", installDir: "<home>/.claude/skills", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
  { id: "trae", name: "Trae", iconKey: "trae", installDir: "<home>/.trae/skills", scanDirs: [], readers: [], status: "detected", official: false, enabled: true },
];

const groups = [
  {
    toolId: "claude-code",
    skills: [
      { name: "vue", description: "", path: "<home>/.claude/skills/vue", toolId: "claude-code", origin: "copied", sync: "vault-updated", sourcePath: "skills/前端/vue" },
    ],
  },
  {
    toolId: "agents",
    skills: [
      { name: "go", description: "", path: "<home>/.agents/skills/go", toolId: "agents", origin: "linked", target: "<vault>/skills/后端/go" },
      { name: "builtin-one", description: "", path: "<home>/.agents/skills/builtin-one", toolId: "agents", origin: "builtin", builtin: true },
    ],
  },
];

beforeEach(() => {
  for (const fn of [uninstallPreview, uninstall, adopt, reveal, distribute]) fn.mockReset();
  uninstallPreview.mockResolvedValue({ preview: { kind: "link", target: "<vault>/skills/后端/go" } });
  uninstall.mockResolvedValue({ items: [{ targetId: "agents", toolId: "agents", result: "removed" }] });
  adopt.mockResolvedValue({ vaultPath: "skills/分类/builtin-one", linked: false, warnings: [] });
  reveal.mockResolvedValue({});
});

afterEach(cleanup);

describe("ToolSkillsPage", () => {
  it("filters with chips and shows source statuses", () => {
    render(<ToolSkillsPage vaultId="v1" groups={groups as never} tools={tools as never} onChanged={() => {}} />);
    expect(screen.getByText("知识库有更新")).toBeTruthy();
    expect(screen.getByText("内置")).toBeTruthy();
    fireEvent.click(screen.getByTestId("tool-chip-claude-code"));
    expect(screen.getByTestId("tool-skill-vue-claude-code")).toBeTruthy();
    expect(screen.queryByTestId("tool-skill-go-agents")).toBeNull();
  });

  it("uninstalls a link after the confirmation card", async () => {
    const onChanged = vi.fn();
    const onResult = vi.fn();
    render(<ToolSkillsPage vaultId="v1" groups={groups as never} tools={tools as never} onChanged={onChanged} onResult={onResult} />);
    fireEvent.click(screen.getByTestId("tool-uninstall-go"));
    await waitFor(() => expect(uninstallPreview).toHaveBeenCalledWith("agents", "go"));
    await waitFor(() => expect(screen.getByTestId("uninstall-target").textContent).toContain("<vault>/skills/后端/go"));
    fireEvent.click(screen.getByTestId("uninstall-confirm"));
    await waitFor(() => expect(uninstall).toHaveBeenCalledWith("agents", "go"));
    await waitFor(() => expect(onResult).toHaveBeenCalled());
    expect((onResult.mock.calls[0]?.[0] as string[]).join(" ")).toContain("已删除链接");
    expect(onChanged).toHaveBeenCalled();
  });

  it("adopts a builtin skill without link-back and passes same-name extras", async () => {
    const onChanged = vi.fn();
    const onResult = vi.fn();
    render(<ToolSkillsPage vaultId="v1" groups={groups as never} tools={tools as never} onChanged={onChanged} onResult={onResult} />);
    expect(screen.queryByTestId("tool-uninstall-builtin-one")).toBeNull();
    fireEvent.click(screen.getByTestId("tool-adopt-builtin-one-agents"));
    fireEvent.change(screen.getByTestId("adopt-category"), { target: { value: "分类" } });
    expect((screen.getByTestId("adopt-linkback") as HTMLInputElement).disabled).toBe(true);
    fireEvent.click(screen.getByTestId("adopt-submit"));
    await waitFor(() => expect(adopt).toHaveBeenCalled());
    const args = adopt.mock.calls[0] as [string, string, string, string, boolean, unknown[]];
    expect(args[0]).toBe("v1");
    expect(args[1]).toBe("agents");
    expect(args[2]).toBe("builtin-one");
    expect(args[3]).toBe("分类");
    expect(args[4]).toBe(false);
    await waitFor(() => expect(onResult).toHaveBeenCalled());
    expect((onResult.mock.calls[0]?.[0] as string[]).join(" ")).toContain("skills/分类/builtin-one");
    expect(onChanged).toHaveBeenCalled();
  });

  it("opens the containing directory", () => {
    render(<ToolSkillsPage vaultId="v1" groups={groups as never} tools={tools as never} onChanged={() => {}} />);
    fireEvent.click(screen.getByRole("button", { name: "打开所在目录 go" }));
    expect(reveal).toHaveBeenCalledWith("agents", "go");
  });
});

describe("ToolSkillsPage states", () => {
  it("folds undetected tools behind a toggle", () => {
    const folded = [
      ...tools,
      { id: "kiro", name: "Kiro", iconKey: "kiro", installDir: "<home>/.kiro/skills", scanDirs: [], readers: [], status: "leftover", official: true, enabled: true },
    ];
    render(<ToolSkillsPage vaultId="v1" groups={[] as never} tools={folded as never} onChanged={() => {}} />);
    expect(screen.queryByTestId("tool-chip-kiro")).toBeNull();
    fireEvent.click(screen.getByTestId("tool-toggle-hidden"));
    expect(screen.getByTestId("tool-chip-kiro")).toBeTruthy();
    fireEvent.click(screen.getByTestId("tool-toggle-hidden"));
    expect(screen.queryByTestId("tool-chip-kiro")).toBeNull();
  });

  it("explains the probe rules when nothing is detected", () => {
    const missing = [
      { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
      { id: "claude-code", name: "Claude Code", iconKey: "claude-code", scanDirs: [], readers: [], status: "missing", official: true, enabled: true },
    ];
    render(<ToolSkillsPage vaultId="v1" groups={[] as never} tools={missing as never} onChanged={() => {}} />);
    expect(screen.getByText("没有检测到已安装的工具。")).toBeTruthy();
    expect(screen.getByText(/探测依据/)).toBeTruthy();
  });
});
