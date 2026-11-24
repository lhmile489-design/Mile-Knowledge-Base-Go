import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { KbSkill, SkillTool } from "../../lib/transport";

const { KbSkillsPage } = await import("./KbSkillsPage");

const tools: SkillTool[] = [
  { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
  { id: "claude-code", name: "Claude Code", iconKey: "claude-code", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
];

const skills: KbSkill[] = [
  { name: "vue", description: "组合式 API", path: "skills/前端/vue", category: "前端", distributed: [{ toolId: "claude-code", mode: "link" }] },
  { name: "go", description: "Go 规范", path: "skills/后端/go", category: "后端", distributed: [] },
];

const noop = () => {};

function renderPage(overrides: Partial<Parameters<typeof KbSkillsPage>[0]> = {}) {
  return render(
    <KbSkillsPage
      groups={[
        { category: "前端", skills: [skills[0]!] },
        { category: "后端", skills: [skills[1]!] },
      ]}
      tools={tools}
      onOpenSkill={noop}
      onLocate={noop}
      onDistribute={noop}
      onInstall={noop}
      onUpdate={noop}
      onOpenDetail={noop}
      {...overrides}
    />,
  );
}

afterEach(cleanup);

describe("KbSkillsPage (M8 T04 layout)", () => {
  it("shows the title, counts and grouped cards with category and status", () => {
    renderPage();
    expect(screen.getByRole("heading", { name: "全部技能" })).toBeTruthy();
    expect(screen.getByTestId("kb-count").textContent).toBe("2 个 · 2 个分类");
    expect(screen.getByTestId("kb-group-前端")).toBeTruthy();
    expect(screen.getByTestId("kb-card-vue").textContent).toContain("已分发 1 处");
    expect(screen.getByTestId("kb-card-vue").textContent).toContain("前端");
    expect(screen.getByTestId("kb-card-go").textContent).toContain("未分发");
  });

  it("uses a single category as the title when the page filters to it", () => {
    renderPage({ groups: [{ category: "前端", skills: [skills[0]!] }] });
    expect(screen.getByRole("heading", { name: "前端" })).toBeTruthy();
    expect(screen.getByTestId("kb-count").textContent).toBe("1 个 · 1 个分类");
  });

  it("shows the scanning count and skeleton cards before the first payload", () => {
    render(
      <KbSkillsPage
        groups={[]}
        tools={tools}
        scanning
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={noop}
        onInstall={noop}
        onUpdate={noop}
      onOpenDetail={noop}
      />,
    );
    expect(screen.getByTestId("kb-count").textContent).toContain("正在扫描");
    expect(screen.getByTestId("kb-skeletons")).toBeTruthy();
    expect(screen.queryByTestId("kb-empty")).toBeNull();
  });

  it("filters by name, description or category and reports the filtered count", () => {
    renderPage();
    fireEvent.change(screen.getByTestId("kb-skills-search"), { target: { value: "组合式" } });
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    expect(screen.queryByTestId("kb-card-go")).toBeNull();
    expect(screen.getByTestId("kb-count").textContent).toBe("1 / 2 个");
    fireEvent.change(screen.getByTestId("kb-skills-search"), { target: { value: "zzz" } });
    expect(screen.getByText("没有匹配的技能。")).toBeTruthy();
    expect(screen.getByTestId("kb-count").textContent).toBe("0 / 2 个");
    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
  });

  it("focuses the search field with Ctrl K", () => {
    renderPage();
    fireEvent.keyDown(document, { key: "k", ctrlKey: true });
    expect(document.activeElement).toBe(screen.getByTestId("kb-skills-search"));
  });

  it("switches between grid and list", () => {
    renderPage({ groups: [{ category: "前端", skills: [skills[0]!] }] });
    expect(screen.getByTestId("kb-grid-前端")).toBeTruthy();
    fireEvent.click(screen.getByRole("tab", { name: "列表" }));
    expect(screen.getByTestId("kb-list-前端")).toBeTruthy();
  });

  it("collects a batch selection from the card menu and hands it to distribute", () => {
    const onDistribute = vi.fn();
    renderPage({ onDistribute });
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 vue" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 go" }));
    expect(screen.getByTestId("kb-batchbar").textContent).toContain("已选 2 个");
    fireEvent.click(screen.getByTestId("kb-batch-distribute"));
    expect(onDistribute).toHaveBeenCalledWith(["skills/前端/vue", "skills/后端/go"], expect.any(Function));
    // The selection stays while the dialog is open (the page keeps it behind the dialog).
    expect(screen.getByTestId("kb-batchbar")).toBeTruthy();
  });

  it("leaves batch selection only when the page says the batch finished", () => {
    const onDistribute = vi.fn();
    renderPage({ onDistribute });
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 vue" }));
    fireEvent.click(screen.getByTestId("kb-batch-distribute"));
    const finished = onDistribute.mock.calls[0]?.[1] as () => void;
    expect(screen.getByTestId("kb-batchbar")).toBeTruthy();
    act(() => finished());
    expect(screen.queryByTestId("kb-batchbar")).toBeNull();
    expect(screen.queryByRole("checkbox", { name: "选择 vue" })).toBeNull();
  });

  it("a single card distributes without a batch callback", () => {
    const onDistribute = vi.fn();
    renderPage({ onDistribute });
    const card = screen.getByTestId("kb-card-vue");
    const button = Array.from(card.querySelectorAll("button")).find((item) => item.textContent?.trim() === "分发");
    fireEvent.click(button as HTMLElement);
    expect(onDistribute).toHaveBeenCalledWith(["skills/前端/vue"]);
  });

  it("selects a whole group and can cancel the batch", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    fireEvent.click(screen.getByTestId("kb-select-all-前端"));
    expect(screen.getByTestId("kb-batchbar").textContent).toContain("已选 1 个");
    fireEvent.click(screen.getByTestId("kb-batch-cancel"));
    expect(screen.queryByTestId("kb-batchbar")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
  });

  it("offers open and locate from the card menu", () => {
    const onOpenSkill = vi.fn();
    const onLocate = vi.fn();
    renderPage({ groups: [{ category: "前端", skills: [skills[0]!] }], onOpenSkill, onLocate });
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "打开 SKILL.md" }));
    expect(onOpenSkill).toHaveBeenCalledWith("skills/前端/vue");
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "在侧栏定位" }));
    expect(onLocate).toHaveBeenCalledWith("skills/前端/vue");
  });

  it("offers install from the toolbar, the empty state and the dashed card", () => {
    const onInstall = vi.fn();
    renderPage({ onInstall });
    fireEvent.click(screen.getByTestId("kb-install-top"));
    fireEvent.click(screen.getByTestId("kb-add-card"));
    expect(onInstall).toHaveBeenCalledTimes(2);
    cleanup();
    render(
      <KbSkillsPage
        groups={[]}
        tools={tools}
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={noop}
        onInstall={onInstall}
        onUpdate={noop}
      onOpenDetail={noop}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "安装技能…" }));
    expect(onInstall).toHaveBeenCalledTimes(3);
  });

  it("shows GitHub update states and turns the action into an update", () => {
    const onUpdate = vi.fn();
    const commit = "0123456789abcdef0123456789abcdef01234567";
    const latest = "89abcdef0123456789abcdef0123456789abcdef";
    const remoteBase = {
      owner: "anthropics",
      repo: "skills",
      ref: "main",
      path: "skills/pdf",
      commit,
      pathCommit: commit,
      installedAt: "2026-11-21T09:00:00Z",
    };
    const withRemote: KbSkill[] = [
      { name: "pdf", description: "PDF", path: "skills/文档/pdf", category: "文档", distributed: [], remote: { ...remoteBase, updateState: "update-available", latestPathCommit: latest } },
      { name: "docx", description: "Word", path: "skills/文档/docx", category: "文档", distributed: [], remote: { ...remoteBase, path: "skills/docx", updateState: "both-changed" } },
    ];
    render(<KbSkillsPage groups={[{ category: "文档", skills: withRemote }]} tools={tools} onOpenSkill={noop} onLocate={noop} onDistribute={noop} onInstall={noop} onUpdate={onUpdate} onOpenDetail={noop} />);
    expect(screen.getByTestId("kb-card-pdf").textContent).toContain("GitHub 有更新");
    expect(screen.getByTestId("kb-card-pdf").textContent).toContain("anthropics/skills");
    expect(screen.getByTestId("kb-card-docx").textContent).toContain("两边都改了");
    fireEvent.click(screen.getByTestId("kb-update-pdf"));
    expect(onUpdate).toHaveBeenCalledWith(withRemote[0]);
    expect(screen.queryByTestId("kb-update-docx")).toBeNull();
  });
});

describe("KbSkillsPage detail entry (M9 T04)", () => {
  it("opens the detail from a click on the card body, Enter and Space", () => {
    const onOpenDetail = vi.fn();
    renderPage({ onOpenDetail });
    const card = screen.getByTestId("kb-card-vue");
    expect(card.getAttribute("tabindex")).toBe("0");
    fireEvent.click(card.querySelector(".kb-card-name") as HTMLElement);
    expect(onOpenDetail).toHaveBeenLastCalledWith("skills/前端/vue");
    fireEvent.keyDown(card, { key: "Enter" });
    fireEvent.keyDown(card, { key: " " });
    expect(onOpenDetail).toHaveBeenCalledTimes(3);
  });

  it("keeps the card's own buttons and menu from opening the detail", () => {
    const onOpenDetail = vi.fn();
    const onDistribute = vi.fn();
    renderPage({ onOpenDetail, onDistribute });
    const card = screen.getByTestId("kb-card-vue");
    const distribute = Array.from(card.querySelectorAll("button")).find((item) => item.textContent?.trim() === "分发");
    fireEvent.click(distribute as HTMLElement);
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "在侧栏定位" }));
    expect(onDistribute).toHaveBeenCalledTimes(1);
    expect(onOpenDetail).not.toHaveBeenCalled();
    // A key press on an inner button is the button's, not the card's.
    const kebab = screen.getByRole("button", { name: "更多操作：vue" });
    fireEvent.keyDown(kebab, { key: "Enter" });
    expect(onOpenDetail).not.toHaveBeenCalled();
  });

  it("toggles the selection instead of opening while a batch is being selected", () => {
    const onOpenDetail = vi.fn();
    renderPage({ onOpenDetail });
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    const card = screen.getByTestId("kb-card-go");
    fireEvent.click(card.querySelector(".kb-card-name") as HTMLElement);
    expect(onOpenDetail).not.toHaveBeenCalled();
    expect(screen.getByTestId("kb-batchbar").textContent).toContain("已选 1 个");
    fireEvent.keyDown(card, { key: "Enter" });
    expect(screen.getByTestId("kb-batchbar").textContent).toContain("已选 0 个");
  });

  it("marks the card whose detail is open", () => {
    const { rerender } = renderPage({ activePath: "skills/前端/vue" });
    expect(screen.getByTestId("kb-card-vue").getAttribute("data-active")).toBe("true");
    expect(screen.getByTestId("kb-card-go").getAttribute("data-active")).toBe("false");
    rerender(
      <KbSkillsPage
        groups={[
          { category: "前端", skills: [skills[0]!] },
          { category: "后端", skills: [skills[1]!] },
        ]}
        tools={tools}
        activePath="skills/后端/go"
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={noop}
        onInstall={noop}
        onUpdate={noop}
        onOpenDetail={noop}
      />,
    );
    expect(screen.getByTestId("kb-card-vue").getAttribute("data-active")).toBe("false");
    expect(screen.getByTestId("kb-card-go").getAttribute("data-active")).toBe("true");
  });
});
