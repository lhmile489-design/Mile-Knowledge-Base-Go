import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const read = vi.fn();
const renderDocument = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {
    readonly code: string;
    constructor(code: string, message: string) {
      super(message);
      this.code = code;
    }
  }
  return {
    TransportError,
    docApi: {
      read: (...args: unknown[]) => read(...args),
      noteOpened: vi.fn(async () => undefined),
      write: vi.fn(),
    },
    renderApi: { document: (...args: unknown[]) => renderDocument(...args) },
    windowApi: { openExternal: vi.fn(async () => undefined) },
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

const { SkillDetailDrawer } = await import("./SkillDetailDrawer");
const { TransportError } = await import("../../lib/transport");
const { useAppStore } = await import("../../stores/appStore");
const { useDocStore } = await import("../../stores/docStore");
import type { KbSkill, SkillTool } from "../../lib/transport";

const tools = [
  { id: "claude-code", name: "Claude Code", iconKey: "claude-code", installDir: "<home>/.claude/skills", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
  { id: "cursor", name: "Cursor", iconKey: "cursor", installDir: "<home>/.cursor/skills", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
] as unknown as SkillTool[];

const plain: KbSkill = { name: "vue", description: "组合式 API", path: "skills/前端/vue", category: "前端", distributed: [] };
const full: KbSkill = {
  name: "frontend-design",
  description: "",
  path: "skills/前端/frontend-design",
  category: "前端",
  distributed: [
    { toolId: "claude-code", mode: "link" },
    { toolId: "cursor", mode: "copy", sync: "vault-updated" },
  ],
  remote: {
    owner: "anthropics",
    repo: "skills",
    ref: "main",
    path: "frontend-design",
    commit: "3f2a91c0000000000000000000000000000000aa",
    pathCommit: "3f2a91c0000000000000000000000000000000aa",
    installedAt: "2026-11-20T10:00:00Z",
    updateState: "update-available",
    latestPathCommit: "8c04d7e0000000000000000000000000000000bb",
  },
};

// ReaderView caches rendered HTML per path@revision@hash for the whole session,
// so every snapshot gets its own hash and tests cannot see each other's output.
let hashSeq = 0;
const snapshot = (path: string, extra: Record<string, unknown> = {}) => {
  hashSeq += 1;
  return {
    doc: { vaultId: "v1", path, text: "# 标题", revision: 3, hash: `hh${hashSeq}`, encoding: "utf8", size: 10, eol: "lf", bom: false, ...extra },
  };
};

type Handlers = Partial<{
  skill: KbSkill;
  onClose: () => void;
  onDistribute: () => void;
  onUpdate: () => void;
  onEdit: () => void;
  onLocate: () => void;
  onRescan: () => void;
}>;

function ui(handlers: Handlers = {}) {
  return (
    <SkillDetailDrawer
      vaultId="v1"
      skill={handlers.skill ?? plain}
      tools={tools}
      onClose={handlers.onClose ?? vi.fn()}
      onDistribute={handlers.onDistribute ?? vi.fn()}
      onUpdate={handlers.onUpdate ?? vi.fn()}
      onEdit={handlers.onEdit ?? vi.fn()}
      onLocate={handlers.onLocate ?? vi.fn()}
      onRescan={handlers.onRescan ?? vi.fn()}
    />
  );
}

beforeEach(() => {
  read.mockReset().mockImplementation(async (_vault: string, path: string) => snapshot(path));
  renderDocument.mockReset().mockResolvedValue({ html: "<h1>标题</h1><p>正文段落</p>" });
  useAppStore.setState({ page: "skills" });
  useDocStore.getState().reset();
});

afterEach(cleanup);

const drawer = () => screen.getByTestId("skill-drawer");

describe("SkillDetailDrawer content", () => {
  it("shows the header, the rendered SKILL.md and no source block for a plain skill", async () => {
    render(ui());
    expect(drawer().getAttribute("data-state")).toBe("loading");
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
    expect(read).toHaveBeenCalledWith("v1", "skills/前端/vue/SKILL.md");
    await waitFor(() => expect(renderDocument).toHaveBeenCalledWith("v1", "skills/前端/vue/SKILL.md", 3));
    expect(screen.getByRole("dialog", { name: "技能详情" })).toBeTruthy();
    expect(drawer().textContent).toContain("vue");
    expect(drawer().textContent).toContain("前端");
    expect(screen.getByTestId("skill-drawer-path").textContent).toBe("skills/前端/vue/SKILL.md");
    await waitFor(() => expect(drawer().querySelector(".prose")?.textContent).toContain("正文段落"));
    expect(screen.getByTestId("skill-drawer-dist").textContent).toContain("未分发");
    expect(screen.queryByTestId("skill-drawer-source")).toBeNull();
  });

  it("lists where it was distributed and where it came from", async () => {
    render(ui({ skill: full }));
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
    const dist = screen.getByTestId("skill-drawer-dist").textContent ?? "";
    expect(dist).toContain("Claude Code");
    expect(dist).toContain("链接");
    expect(dist).toContain("Cursor");
    expect(dist).toContain("拷贝");
    expect(dist).toContain("知识库有更新");
    const source = screen.getByTestId("skill-drawer-source").textContent ?? "";
    expect(source).toContain("anthropics/skills");
    expect(source).toContain("frontend-design");
    expect(source).toContain("3f2a91c");
    expect(source).toContain("8c04d7e");
    expect(source).toContain("有更新");
  });

  it("shows the file-missing state with a rescan that also reads again", async () => {
    read.mockRejectedValueOnce(new TransportError("not-found", "文件不存在"));
    const onRescan = vi.fn();
    render(ui({ onRescan }));
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("missing"));
    expect(drawer().textContent).toContain("找不到这个技能文件");
    expect(renderDocument).not.toHaveBeenCalled();
    fireEvent.click(screen.getByTestId("skill-drawer-rescan"));
    expect(onRescan).toHaveBeenCalledTimes(1);
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("treats a path that is not a file the same way", async () => {
    read.mockRejectedValueOnce(new TransportError("not-a-file", "不是文件"));
    render(ui());
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("missing"));
  });

  it("does not render a file over the reading limit and points to the workbench", async () => {
    read.mockResolvedValueOnce(snapshot("skills/前端/vue/SKILL.md", { degraded: "large-source-only", size: 1500000 }));
    render(ui());
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("large"));
    expect(drawer().textContent).toContain("文件较大，请在工作台打开");
    expect(renderDocument).not.toHaveBeenCalled();
  });

  it("treats the read-only size class the same way", async () => {
    read.mockResolvedValueOnce(snapshot("skills/前端/vue/SKILL.md", { degraded: "large-read-only", size: 3000000 }));
    render(ui());
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("large"));
    expect(renderDocument).not.toHaveBeenCalled();
  });

  it("shows other read failures with the reason and a retry", async () => {
    read.mockRejectedValueOnce(new TransportError("io-failure", "磁盘忙"));
    render(ui());
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("error"));
    expect(drawer().textContent).toContain("磁盘忙");
    fireEvent.click(screen.getByTestId("skill-drawer-retry"));
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
  });

  it("shows a render failure inside the preview, keeping the rest of the drawer", async () => {
    renderDocument.mockRejectedValueOnce(new TransportError("unsupported", "渲染失败"));
    render(ui({ skill: full }));
    await waitFor(() => expect(drawer().textContent).toContain("渲染失败"));
    expect(screen.getByTestId("skill-drawer-dist")).toBeTruthy();
  });
});

describe("SkillDetailDrawer switching", () => {
  it("replaces the content when another skill is chosen and drops a late answer of the old one", async () => {
    let releaseFirst: (value: unknown) => void = () => undefined;
    read.mockImplementationOnce(() => new Promise((resolve) => (releaseFirst = resolve)));
    const { rerender } = render(ui());
    expect(drawer().getAttribute("data-state")).toBe("loading");

    rerender(ui({ skill: full }));
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
    expect(screen.getByTestId("skill-drawer-path").textContent).toBe("skills/前端/frontend-design/SKILL.md");

    // The first read finishes after the second one: it must not replace anything.
    await act(async () => {
      releaseFirst(snapshot("skills/前端/vue/SKILL.md", { text: "旧的" }));
    });
    expect(screen.getByTestId("skill-drawer-path").textContent).toBe("skills/前端/frontend-design/SKILL.md");
    expect(renderDocument).not.toHaveBeenCalledWith("v1", "skills/前端/vue/SKILL.md", expect.anything());
    expect(drawer().getAttribute("data-state")).toBe("ready");
  });

  it("keeps the same drawer element while switching (no replay of the slide-in)", async () => {
    const { rerender } = render(ui());
    await waitFor(() => expect(drawer().getAttribute("data-state")).toBe("ready"));
    const element = drawer();
    rerender(ui({ skill: full }));
    await waitFor(() => expect(screen.getByTestId("skill-drawer-path").textContent).toContain("frontend-design"));
    expect(drawer()).toBe(element);
  });
});

describe("SkillDetailDrawer actions and keyboard", () => {
  it("focuses the close button on open", () => {
    render(ui());
    expect(document.activeElement).toBe(screen.getByTestId("skill-drawer-close"));
  });

  it("closes with the button and with Esc, but leaves Esc to an open dialog", () => {
    const onClose = vi.fn();
    render(ui({ onClose }));
    fireEvent.click(screen.getByTestId("skill-drawer-close"));
    expect(onClose).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);

    const dialog = document.createElement("div");
    dialog.setAttribute("data-testid", "skills-modal");
    document.body.append(dialog);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(2);
    dialog.remove();

    // An input method that is composing keeps its Esc.
    fireEvent.keyDown(document, { key: "Escape", isComposing: true });
    expect(onClose).toHaveBeenCalledTimes(2);
  });

  it("fires the footer actions; the update button exists only when an update is available", async () => {
    const onDistribute = vi.fn();
    const onUpdate = vi.fn();
    const onEdit = vi.fn();
    const onLocate = vi.fn();
    const { rerender } = render(ui({ onDistribute, onUpdate, onEdit, onLocate }));
    expect(screen.queryByTestId("skill-drawer-update")).toBeNull();
    fireEvent.click(screen.getByTestId("skill-drawer-distribute"));
    fireEvent.click(screen.getByTestId("skill-drawer-edit"));
    fireEvent.click(screen.getByTestId("skill-drawer-locate"));
    expect(onDistribute).toHaveBeenCalledTimes(1);
    expect(onEdit).toHaveBeenCalledTimes(1);
    expect(onLocate).toHaveBeenCalledTimes(1);

    rerender(ui({ skill: full, onDistribute, onUpdate, onEdit, onLocate }));
    fireEvent.click(screen.getByTestId("skill-drawer-update"));
    expect(onUpdate).toHaveBeenCalledTimes(1);
  });

  it("switches to the workbench before an in-vault link opens", async () => {
    renderDocument.mockResolvedValue({
      html: '<p><a data-kind="doc" data-status="ok" data-target="notes/a.md" href="#">去笔记</a></p>',
    });
    render(ui());
    fireEvent.click(await screen.findByText("去笔记"));
    expect(useAppStore.getState().page).toBe("work");
    await waitFor(() => expect(useDocStore.getState().activePath).toBe("notes/a.md"));
  });
});
