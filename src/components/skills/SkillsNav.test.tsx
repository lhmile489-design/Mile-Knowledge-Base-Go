import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { SkillScan, SkillTool } from "../../lib/transport";
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
  ],
};

const tools = [
  { id: "claude-code", name: "Claude Code", iconKey: "claude" },
  { id: "cursor", name: "Cursor", iconKey: "cursor" },
] as SkillTool[];

const emptySelection: SkillsSelection = { kind: "kb", category: null };

function renderNav(selection: SkillsSelection = emptySelection, hasVault = true) {
  const onSelect = vi.fn();
  render(<SkillsNav selection={selection} onSelect={onSelect} scan={scan} tools={tools} hasVault={hasVault} />);
  return onSelect;
}

afterEach(cleanup);

describe("skills navigation (M8 T03)", () => {
  it("lists both groups with their counts and marks the current item", () => {
    renderNav();
    expect(screen.getByRole("navigation", { name: "技能导航" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /全部技能/ }).textContent).toContain("3");
    expect(screen.getByRole("button", { name: /前端/ }).textContent).toContain("2");
    expect(screen.getByRole("button", { name: /后端/ }).textContent).toContain("1");
    expect(screen.getByRole("button", { name: /全部工具/ }).textContent).toContain("3");
    expect(screen.getByRole("button", { name: /Claude Code/ }).textContent).toContain("2");
    expect(screen.getByRole("button", { name: /Cursor/ }).textContent).toContain("1");
    expect(screen.getByRole("button", { name: /全部技能/ }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("button", { name: /前端/ }).getAttribute("aria-current")).toBeNull();
  });

  it("selects categories, tools and the targets manager", () => {
    const onSelect = renderNav({ kind: "kb", category: "前端" });
    expect(screen.getByRole("button", { name: /前端/ }).getAttribute("aria-current")).toBe("page");
    fireEvent.click(screen.getByRole("button", { name: /后端/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "kb", category: "后端" });
    fireEvent.click(screen.getByRole("button", { name: /全部工具/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "tools", toolId: null });
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "tools", toolId: "cursor" });
    fireEvent.click(screen.getByRole("button", { name: /管理目录/ }));
    expect(onSelect).toHaveBeenLastCalledWith({ kind: "targets" });
  });

  it("marks tools that need attention and reports progress", () => {
    renderNav();
    expect(screen.getByTestId("skills-nav-dot-claude-code")).toBeTruthy();
    expect(screen.queryByTestId("skills-nav-dot-cursor")).toBeNull();
    expect(screen.getByRole("button", { name: /Claude Code/ }).getAttribute("aria-label")).toContain("需要处理");
    expect(screen.getByText("已分发")).toBeTruthy();
    expect(screen.getByText("1 / 3")).toBeTruthy();
    expect(screen.getByText("0 个有更新 · 1 个链接失效")).toBeTruthy();
  });

  it("asks for a vault instead of showing progress without one", () => {
    renderNav(emptySelection, false);
    expect(screen.getByText("打开知识库后显示分发进度")).toBeTruthy();
    expect(screen.queryByText("已分发")).toBeNull();
  });
});
