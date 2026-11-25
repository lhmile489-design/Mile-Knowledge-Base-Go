import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { SkillScan, SkillTool, SkillUpdateCheckResult } from "../../lib/transport";
import type { SkillsSelection } from "./SkillsNav";
import { SkillsNav } from "./SkillsNav";

const scan: SkillScan = {
  cachedAt: "2026-11-24T09:00:00Z",
  scanning: false,
  kb: [
    {
      category: "前端",
      skills: [
        { name: "vue", description: "", path: "skills/前端/vue", category: "前端", distributed: [{ toolId: "claude-code", mode: "link" }] },
        { name: "react", description: "", path: "skills/前端/react", category: "前端", distributed: [] },
      ],
    },
    {
      category: "后端",
      skills: [{ name: "go", description: "", path: "skills/后端/go", category: "后端", distributed: [] }],
    },
  ],
  tools: [
    {
      toolId: "claude-code",
      skills: [
        { name: "vue", description: "", path: "C:\\t\\vue", toolId: "claude-code", origin: "link-broken" },
        { name: "react", description: "", path: "C:\\t\\react", toolId: "claude-code", origin: "linked" },
      ],
    },
    {
      toolId: "cursor",
      skills: [{ name: "go", description: "", path: "C:\\t\\go", toolId: "cursor", origin: "copied", sync: "in-sync" }],
    },
    { toolId: "kiro", skills: [] },
    { toolId: "codebuddy", skills: [{ name: "x", description: "", path: "C:\\t\\x", toolId: "codebuddy", origin: "link-broken" }] },
  ],
};

const tool = (id: string, name: string, extra: Record<string, unknown> = {}) =>
  ({ id, name, iconKey: id, scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true, ...extra }) as unknown as SkillTool;

const tools: SkillTool[] = [
  tool("claude-code", "Claude Code"),
  tool("cursor", "Cursor"),
  tool("kiro", "Kiro", { status: "missing" }),
  tool("codebuddy", "CodeBuddy", { status: "missing" }),
];

const kbAll: SkillsSelection = { kind: "kb", category: null };

type Extra = Partial<{
  collapsed: { kb: boolean; tools: boolean };
  onSetCollapsed: (group: "kb" | "tools", collapsed: boolean) => void;
  update: SkillUpdateCheckResult | null;
  onCheckUpdates: () => void;
  scanValue: SkillScan | null;
}>;

function renderNav(selection: SkillsSelection = kbAll, hasVault = true, extra: Extra = {}) {
  const onSelect = vi.fn();
  const onSetCollapsed = extra.onSetCollapsed ?? vi.fn();
  const view = (props: { selection: SkillsSelection; collapsed?: { kb: boolean; tools: boolean } }) => (
    <SkillsNav
      selection={props.selection}
      onSelect={onSelect}
      scan={extra.scanValue === undefined ? scan : extra.scanValue}
      tools={tools}
      hasVault={hasVault}
      collapsed={props.collapsed ?? extra.collapsed ?? { kb: false, tools: false }}
      onSetCollapsed={onSetCollapsed}
      update={extra.update}
      onCheckUpdates={extra.onCheckUpdates}
    />
  );
  const result = render(view({ selection }));
  return {
    onSelect,
    onSetCollapsed,
    rerender: (props: { selection: SkillsSelection; collapsed?: { kb: boolean; tools: boolean } }) => result.rerender(view(props)),
  };
}

afterEach(cleanup);

describe("skills navigation items", () => {
  it("lists both groups with their counts and marks the current item", () => {
    renderNav();
    expect(screen.getByRole("navigation", { name: "技能导航" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /全部技能/ }).textContent).toContain("3");
    expect(screen.getByRole("button", { name: /前端/ }).textContent).toContain("2");
    expect(screen.getByRole("button", { name: /后端/ }).textContent).toContain("1");
    expect(screen.getByRole("button", { name: /全部工具/ }).textContent).toContain("4");
    expect(screen.getByRole("button", { name: /Claude Code/ }).textContent).toContain("2");
    expect(screen.getByRole("button", { name: /Cursor/ }).textContent).toContain("1");
    expect(screen.getByRole("button", { name: /全部技能/ }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("button", { name: /前端/ }).getAttribute("aria-current")).toBeNull();
  });

  it("selects categories and tools", () => {
    const { onSelect } = renderNav({ kind: "kb", category: "前端" });
    expect(screen.getByRole("button", { name: /前端/ }).getAttribute("aria-current")).toBe("page");
    fireEvent.click(screen.getByRole("button", { name: /后端/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "kb", category: "后端" });
    fireEvent.click(screen.getByRole("button", { name: /全部工具/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "tools", toolId: null });
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "tools", toolId: "cursor" });
  });

  it("marks tools that need attention", () => {
    renderNav();
    expect(screen.getByTestId("skills-nav-dot-claude-code")).toBeTruthy();
    expect(screen.queryByTestId("skills-nav-dot-cursor")).toBeNull();
    expect(screen.getByRole("button", { name: /Claude Code/ }).getAttribute("aria-label")).toContain("需要处理");
  });
});

describe("skills navigation group heads (M9 T05)", () => {
  it("has two group heads that are buttons with aria-expanded and their group sizes", () => {
    renderNav();
    const kb = screen.getByTestId("skills-nav-toggle-kb");
    const toolsHead = screen.getByTestId("skills-nav-toggle-tools");
    expect(kb.tagName).toBe("BUTTON");
    expect(kb.getAttribute("aria-expanded")).toBe("true");
    expect(kb.textContent).toContain("知识库");
    expect(kb.textContent).toContain("2");
    expect(toolsHead.getAttribute("aria-expanded")).toBe("true");
    expect(toolsHead.textContent).toContain("工具目录");
    expect(toolsHead.textContent).toContain("4");
  });

  it("asks to collapse and expand a group through onSetCollapsed", () => {
    const { onSetCollapsed } = renderNav();
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    expect(onSetCollapsed).toHaveBeenLastCalledWith("kb", true);
    fireEvent.click(screen.getByTestId("skills-nav-toggle-tools"));
    expect(onSetCollapsed).toHaveBeenLastCalledWith("tools", true);
  });

  it("hides a collapsed group's items, keeps its head, and says the group is collapsed", () => {
    renderNav(kbAll, true, { collapsed: { kb: true, tools: false } });
    expect(screen.getByTestId("skills-nav-toggle-kb").getAttribute("aria-expanded")).toBe("false");
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    expect(screen.getByTestId("skills-nav-items-tools").hasAttribute("data-collapsed")).toBe(false);
    // The current item lives in the folded group: the head carries a marker.
    expect(screen.getByTestId("skills-nav-curmark-kb")).toBeTruthy();
    expect(screen.queryByTestId("skills-nav-curmark-tools")).toBeNull();
  });

  it("does not mark a collapsed group the selection is not in", () => {
    renderNav({ kind: "tools", toolId: "cursor" }, true, { collapsed: { kb: true, tools: false } });
    expect(screen.queryByTestId("skills-nav-curmark-kb")).toBeNull();
  });

  it("expands a collapsed group when the selection moves into it, but not on first render", () => {
    const { onSetCollapsed, rerender } = renderNav(kbAll, true, { collapsed: { kb: false, tools: true } });
    // Mounted with the tools group folded and the selection elsewhere: stay folded.
    expect(onSetCollapsed).not.toHaveBeenCalled();
    rerender({ selection: { kind: "tools", toolId: "cursor" }, collapsed: { kb: false, tools: true } });
    expect(onSetCollapsed).toHaveBeenCalledWith("tools", false);
  });

  it("lets the user collapse the group the current item is in", () => {
    const { onSetCollapsed, rerender } = renderNav(kbAll);
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    expect(onSetCollapsed).toHaveBeenCalledTimes(1);
    expect(onSetCollapsed).toHaveBeenLastCalledWith("kb", true);
    // The setting comes back collapsed; the selection did not change, so no re-expand.
    rerender({ selection: kbAll, collapsed: { kb: true, tools: false } });
    expect(onSetCollapsed).toHaveBeenCalledTimes(1);
  });

  it("moves 管理目录 into the tools group head as a button and out of the list", () => {
    const { onSelect } = renderNav();
    const button = screen.getByRole("button", { name: "管理目录" });
    expect(screen.getByTestId("skills-nav-toggle-tools").contains(button)).toBe(false);
    expect(screen.getByTestId("skills-nav-head-tools").contains(button)).toBe(true);
    expect(button.getAttribute("title")).toBe("管理目录…");
    expect(within(screen.getByTestId("skills-nav-items-tools")).queryByRole("button", { name: /管理目录/ })).toBeNull();
    fireEvent.click(button);
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "targets" });
  });

  it("keeps the manager button reachable and marked while its group is collapsed", () => {
    renderNav({ kind: "targets" }, true, { collapsed: { kb: false, tools: true } });
    const button = screen.getByRole("button", { name: "管理目录" });
    expect(button.getAttribute("aria-current")).toBe("page");
  });
});

describe("skills navigation undetected tools (M9 T05)", () => {
  it("folds tools that are not detected under one expandable item with a count", () => {
    renderNav();
    expect(screen.queryByRole("button", { name: /Kiro/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /CodeBuddy/ })).toBeNull();
    const more = screen.getByTestId("skills-nav-undetected");
    expect(more.textContent).toContain("未检测到的工具（2）");
    expect(more.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(more);
    expect(more.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("button", { name: /Kiro/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /CodeBuddy/ })).toBeTruthy();
    fireEvent.click(more);
    expect(screen.queryByRole("button", { name: /Kiro/ })).toBeNull();
  });

  it("opens the fold by itself when the selected tool is not detected", () => {
    renderNav({ kind: "tools", toolId: "kiro" });
    expect(screen.getByTestId("skills-nav-undetected").getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByRole("button", { name: /Kiro/ }).getAttribute("aria-current")).toBe("page");
  });

  it("flags the fold when a folded tool needs attention", () => {
    renderNav();
    expect(screen.getByTestId("skills-nav-undetected-dot")).toBeTruthy();
  });

  it("shows every tool while the registry has not loaded", () => {
    const onSelect = vi.fn();
    render(
      <SkillsNav
        selection={kbAll}
        onSelect={onSelect}
        scan={scan}
        tools={[]}
        hasVault
        collapsed={{ kb: false, tools: false }}
        onSetCollapsed={vi.fn()}
      />,
    );
    expect(screen.getByRole("button", { name: /kiro/ })).toBeTruthy();
    expect(screen.queryByTestId("skills-nav-undetected")).toBeNull();
  });
});

describe("skills navigation filter (M9 T05)", () => {
  const filterBox = () => screen.getByRole("textbox", { name: "筛选分类与工具" }) as HTMLInputElement;
  const type = (value: string) => fireEvent.change(filterBox(), { target: { value } });

  it("shows only the matching categories and tools, case-insensitively, and highlights the hit", () => {
    renderNav();
    type("COD");
    expect(screen.queryByRole("button", { name: /前端/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /Cursor/ })).toBeNull();
    // "Claude Code" and "CodeBuddy" both contain "cod", whatever the case.
    expect(screen.getByRole("button", { name: /Claude Code/ }).querySelector("mark")?.textContent).toBe("Cod");
    expect(screen.getByRole("button", { name: /CodeBuddy/ }).querySelector("mark")?.textContent).toBe("Cod");
  });

  it("matches categories as well as tools", () => {
    renderNav();
    type("前");
    expect(screen.getByRole("button", { name: /前端/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /后端/ })).toBeNull();
    expect(screen.getByTestId("skills-nav-toggle-kb").textContent).toContain("1 / 2");
    expect(screen.getByTestId("skills-nav-toggle-tools").textContent).toContain("0 / 4");
  });

  it("writes 命中 / 总数 on the group heads and hides the 全部 items and the undetected fold", () => {
    renderNav();
    type("code");
    expect(screen.getByTestId("skills-nav-toggle-tools").textContent).toContain("2 / 4");
    expect(screen.queryByRole("button", { name: /全部技能/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /全部工具/ })).toBeNull();
    expect(screen.queryByTestId("skills-nav-undetected")).toBeNull();
    // Folded-away tools still match by name.
    expect(screen.getByRole("button", { name: /CodeBuddy/ })).toBeTruthy();
  });

  it("opens both groups while filtering without touching the saved state", () => {
    const { onSetCollapsed } = renderNav(kbAll, true, { collapsed: { kb: true, tools: true } });
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    type("后");
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(false);
    expect(screen.getByTestId("skills-nav-items-tools").hasAttribute("data-collapsed")).toBe(false);
    expect(screen.getByRole("button", { name: /后端/ })).toBeTruthy();
    expect(onSetCollapsed).not.toHaveBeenCalled();
    // The heads do not write the setting while the filter forces them open.
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    expect(onSetCollapsed).not.toHaveBeenCalled();
    type("");
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
  });

  it("says when nothing matches", () => {
    renderNav();
    type("zzz");
    expect(screen.getByText("没有匹配的分类或工具")).toBeTruthy();
    type("前");
    expect(screen.queryByText("没有匹配的分类或工具")).toBeNull();
    expect(screen.getByText("没有匹配的工具")).toBeTruthy();
    type("cursor");
    expect(screen.getByText("没有匹配的分类")).toBeTruthy();
  });

  it("clears with the clear button and with Esc, and leaves Esc alone when empty", () => {
    renderNav();
    type("cod");
    fireEvent.click(screen.getByRole("button", { name: "清除筛选" }));
    expect(filterBox().value).toBe("");
    expect(screen.getByRole("button", { name: /前端/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "清除筛选" })).toBeNull();

    type("cod");
    const seen = vi.fn();
    document.addEventListener("keydown", seen);
    const handled = !fireEvent.keyDown(filterBox(), { key: "Escape" });
    expect(handled).toBe(true);
    expect(filterBox().value).toBe("");
    expect(seen).not.toHaveBeenCalled();

    // Nothing to clear: the key goes on to whoever listens (the drawer, a dialog).
    const passed = fireEvent.keyDown(filterBox(), { key: "Escape" });
    expect(passed).toBe(true);
    expect(seen).toHaveBeenCalled();
    document.removeEventListener("keydown", seen);
  });

  it("does not change the selection", () => {
    const { onSelect } = renderNav();
    type("cod");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("is not shown without a knowledge base's categories, but still filters tools", () => {
    renderNav(kbAll, false);
    type("cur");
    expect(screen.getByRole("button", { name: /Cursor/ })).toBeTruthy();
  });
});

describe("skills navigation footer (M9 T05)", () => {
  it("shows the progress row and the attention count in two rows", () => {
    renderNav();
    expect(screen.getByText("已分发")).toBeTruthy();
    expect(screen.getByText("1 / 3")).toBeTruthy();
    const attention = screen.getByTestId("skills-attention");
    expect(attention.textContent).toBe("2 项需要处理");
    expect(attention.getAttribute("title")).toBe("0 个有更新 · 2 个链接失效");
  });

  it("says so when there is nothing to handle", () => {
    renderNav(kbAll, true, { scanValue: { ...scan, tools: [] } });
    expect(screen.getByTestId("skills-attention").textContent).toBe("没有需要处理的项");
  });

  it("keeps the update check on the second row with the check time", () => {
    const onCheckUpdates = vi.fn();
    renderNav(kbAll, true, {
      onCheckUpdates,
      update: { checkedAt: new Date().toISOString(), checking: false, groups: 1, unchecked: 0 } as SkillUpdateCheckResult,
    });
    expect(screen.getByTestId("skills-checked").textContent).toContain("刚刚检查过");
    fireEvent.click(screen.getByTestId("skills-check-updates"));
    expect(onCheckUpdates).toHaveBeenCalledTimes(1);
    const row2 = screen.getByTestId("skills-attention").parentElement;
    expect(row2?.contains(screen.getByTestId("skills-check-updates"))).toBe(true);
    expect(row2?.contains(screen.getByTestId("skills-checked"))).toBe(true);
  });

  it("still shows the scanning notice, the unchecked sources and the rate limit", () => {
    renderNav(kbAll, true, {
      scanValue: { ...scan, scanning: true },
      onCheckUpdates: vi.fn(),
      update: {
        checkedAt: new Date().toISOString(),
        checking: false,
        groups: 2,
        unchecked: 2,
        rateLimitResetAt: new Date(Date.now() + 60000).toISOString(),
      } as SkillUpdateCheckResult,
    });
    expect(screen.getByTestId("skills-status").textContent).toContain("后台扫描中");
    expect(screen.getByTestId("skills-unchecked").textContent).toContain("2 个来源");
    expect(screen.getByTestId("skills-rate-limit").textContent).toContain("额度用完");
  });

  it("drops the scan line once the scan is done", () => {
    renderNav();
    expect(screen.queryByTestId("skills-status")).toBeNull();
  });

  it("asks for a vault instead of showing progress without one", () => {
    renderNav(kbAll, false);
    expect(screen.getByText("打开知识库后显示分发进度")).toBeTruthy();
    expect(screen.queryByText("已分发")).toBeNull();
  });
});

// A controlled wrapper, the way SkillsPage drives the nav: the collapsed state
// round-trips and selection changes come from the clicks.
function Controlled({ onSetCollapsed }: { onSetCollapsed: (group: "kb" | "tools", collapsed: boolean) => void }) {
  const [selection, setSelection] = useState<SkillsSelection>(kbAll);
  const [collapsed, setCollapsed] = useState({ kb: false, tools: false });
  return (
    <SkillsNav
      selection={selection}
      onSelect={setSelection}
      scan={scan}
      tools={tools}
      hasVault
      collapsed={collapsed}
      onSetCollapsed={(group, value) => {
        onSetCollapsed(group, value);
        setCollapsed((current) => ({ ...current, [group]: value }));
      }}
    />
  );
}

describe("skills navigation flow (AC-UX-10 shape)", () => {
  it("collapses 知识库, then switching to a tool leaves it collapsed with the marker gone", () => {
    const onSetCollapsed = vi.fn();
    render(<Controlled onSetCollapsed={onSetCollapsed} />);
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    expect(screen.getByTestId("skills-nav-curmark-kb")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    expect(screen.queryByTestId("skills-nav-curmark-kb")).toBeNull();
    expect(onSetCollapsed).toHaveBeenCalledTimes(1);
  });

  it("opens the folded group again when the page moves the selection into it", () => {
    const onSetCollapsed = vi.fn();
    const { rerender } = render(
      <SkillsNav
        selection={{ kind: "tools", toolId: null }}
        onSelect={vi.fn()}
        scan={scan}
        tools={tools}
        hasVault
        collapsed={{ kb: true, tools: false }}
        onSetCollapsed={onSetCollapsed}
      />,
    );
    act(() => {
      rerender(
        <SkillsNav
          selection={{ kind: "kb", category: "前端" }}
          onSelect={vi.fn()}
          scan={scan}
          tools={tools}
          hasVault
          collapsed={{ kb: true, tools: false }}
          onSetCollapsed={onSetCollapsed}
        />,
      );
    });
    expect(onSetCollapsed).toHaveBeenCalledWith("kb", false);
  });
});
