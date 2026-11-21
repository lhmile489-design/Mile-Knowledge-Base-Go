import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { KbSkill, SkillTool } from "../../lib/transport";

const { KbSkillsPage } = await import("./KbSkillsPage");

const tools: SkillTool[] = [
  { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
  { id: "claude-code", name: "Claude Code", iconKey: "claude-code", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
];

const skills: KbSkill[] = [
  { name: "vue", description: "组合式 API", path: "skills/前端/vue", category: "前端", distributed: [{ toolId: "claude-code", mode: "link" }] },
  { name: "go", description: "Go 规范", path: "skills/后端/go", category: "后端", distributed: [] },
];

const noop = () => {};

afterEach(cleanup);

describe("KbSkillsPage", () => {
  it("renders grouped cards with the distribution status", () => {
    render(
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
      />,
    );
    expect(screen.getByTestId("kb-group-前端")).toBeTruthy();
    expect(screen.getByTestId("kb-card-vue").textContent).toContain("已分发 1 处");
    expect(screen.getByTestId("kb-card-go").textContent).toContain("未分发");
  });

  it("filters by name, description or category", () => {
    render(
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
      />,
    );
    fireEvent.change(screen.getByTestId("kb-skills-search"), { target: { value: "组合式" } });
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    expect(screen.queryByTestId("kb-card-go")).toBeNull();
    fireEvent.change(screen.getByTestId("kb-skills-search"), { target: { value: "zzz" } });
    expect(screen.getByText("没有匹配的技能。")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "清除搜索" }));
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
  });

  it("switches between grid and list", () => {
    render(
      <KbSkillsPage
        groups={[{ category: "前端", skills: [skills[0]!] }]}
        tools={tools}
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={noop}
        onInstall={noop}
        onUpdate={noop}
      />,
    );
    expect(screen.getByTestId("kb-grid-前端")).toBeTruthy();
    fireEvent.click(screen.getByRole("tab", { name: "列表" }));
    expect(screen.getByTestId("kb-list-前端")).toBeTruthy();
  });

  it("collects a batch selection and hands it to distribute", () => {
    const onDistribute = vi.fn();
    render(
      <KbSkillsPage
        groups={[
          { category: "前端", skills: [skills[0]!] },
          { category: "后端", skills: [skills[1]!] },
        ]}
        tools={tools}
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={onDistribute}
        onInstall={noop}
        onUpdate={noop}
      />,
    );
    fireEvent.click(screen.getByTestId("kb-batch-toggle"));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 vue" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 go" }));
    fireEvent.click(screen.getByTestId("kb-batch-distribute"));
    expect(onDistribute).toHaveBeenCalledWith(["skills/前端/vue", "skills/后端/go"]);
  });

  it("offers open and locate from the card menu", () => {
    const onOpenSkill = vi.fn();
    const onLocate = vi.fn();
    render(
      <KbSkillsPage
        groups={[{ category: "前端", skills: [skills[0]!] }]}
        tools={tools}
        onOpenSkill={onOpenSkill}
        onLocate={onLocate}
        onDistribute={noop}
        onInstall={noop}
        onUpdate={noop}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "打开 SKILL.md" }));
    expect(onOpenSkill).toHaveBeenCalledWith("skills/前端/vue");
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "在侧栏定位" }));
    expect(onLocate).toHaveBeenCalledWith("skills/前端/vue");
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
    render(
      <KbSkillsPage
        groups={[{ category: "文档", skills: withRemote }]}
        tools={tools}
        onOpenSkill={noop}
        onLocate={noop}
        onDistribute={noop}
        onInstall={noop}
        onUpdate={onUpdate}
      />,
    );
    expect(screen.getByTestId("kb-card-pdf").textContent).toContain("GitHub 有更新");
    expect(screen.getByTestId("kb-card-pdf").textContent).toContain("anthropics/skills");
    expect(screen.getByTestId("kb-card-docx").textContent).toContain("两边都改了");
    fireEvent.click(screen.getByTestId("kb-update-pdf"));
    expect(onUpdate).toHaveBeenCalledWith(withRemote[0]);
    expect(screen.queryByTestId("kb-update-docx")).toBeNull();
  });
});
