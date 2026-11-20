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
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "打开 SKILL.md" }));
    expect(onOpenSkill).toHaveBeenCalledWith("skills/前端/vue");
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "在侧栏定位" }));
    expect(onLocate).toHaveBeenCalledWith("skills/前端/vue");
  });
});
