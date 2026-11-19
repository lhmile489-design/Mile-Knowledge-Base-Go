import { describe, expect, it } from "vitest";
import type { AppConfigResult, CategoryConfig } from "../mygo";
import { STEERING_DEFAULTS } from "./agentDefaults";
import {
  applyAliasDrafts,
  categoryAliasDraftId,
  defaultIndexedFor,
  dirtyDraftIds,
  draftLabel,
  overlongAliasDrafts,
  steeringDraftId,
  steeringDrafts,
  aliasDrafts,
} from "./settingsDrafts";

function category(over: Partial<CategoryConfig> = {}): CategoryConfig {
  return { directory: "projects", alias: "", area: null, indexed: true, order: 0, ...over };
}

function config(over: Partial<AppConfigResult> = {}): AppConfigResult {
  return {
    vaultId: "v",
    aliases: {},
    categories: [category()],
    agents: {},
    ...over,
  } as AppConfigResult;
}

describe("draft ids", () => {
  it("round-trips through kind grouping", () => {
    const drafts = {
      [steeringDraftId("curator")]: "更简洁",
      [categoryAliasDraftId("Projects")]: "项目",
    };
    expect(steeringDrafts(drafts)).toEqual({ curator: "更简洁" });
    expect(aliasDrafts(drafts)).toEqual({ projects: "项目" });
  });

  it("lower-cases directory ids so Windows case differences do not split a draft", () => {
    expect(categoryAliasDraftId("Projects")).toBe(categoryAliasDraftId("projects"));
  });
});

describe("dirtyDraftIds", () => {
  it("flags a steering draft only when it differs from saved or default", () => {
    const cfg = config({ agents: { curator: { model: "", effort: "auto", approval: "ask", skillPaths: [], steering: "已保存", steeringBaseVersion: 1 } } });
    expect(dirtyDraftIds({ [steeringDraftId("curator")]: "已保存" }, cfg)).toEqual([]);
    expect(dirtyDraftIds({ [steeringDraftId("curator")]: "改过" }, cfg)).toEqual([steeringDraftId("curator")]);
    // 没有保存值时和内置默认比。
    expect(dirtyDraftIds({ [steeringDraftId("scout")]: STEERING_DEFAULTS.scout }, cfg)).toEqual([]);
    expect(dirtyDraftIds({ [steeringDraftId("scout")]: "别的" }, cfg)).toEqual([steeringDraftId("scout")]);
  });

  it("flags an alias draft only when it differs from the saved alias", () => {
    const cfg = config({ categories: [category({ alias: "项目" })] });
    expect(dirtyDraftIds({ [categoryAliasDraftId("projects")]: "项目" }, cfg)).toEqual([]);
    expect(dirtyDraftIds({ [categoryAliasDraftId("projects")]: "新名字" }, cfg)).toEqual([
      categoryAliasDraftId("projects"),
    ]);
    // 没配置过的目录，保存值为空串。
    expect(dirtyDraftIds({ [categoryAliasDraftId("wiki")]: "" }, cfg)).toEqual([]);
    expect(dirtyDraftIds({ [categoryAliasDraftId("wiki")]: "维基" }, cfg)).toEqual([
      categoryAliasDraftId("wiki"),
    ]);
  });

  it("ignores unknown ids", () => {
    expect(dirtyDraftIds({ "unknown:thing": "x", [categoryAliasDraftId("")]: "x" }, config())).toEqual([]);
  });
});

describe("labels", () => {
  it("names the agent or the directory", () => {
    const cfg = config({ categories: [category({ alias: "项目" })] });
    expect(draftLabel(steeringDraftId("curator"), cfg)).toContain("Steering");
    expect(draftLabel(categoryAliasDraftId("projects"), cfg)).toBe("目录「projects」的显示名");
    expect(draftLabel(categoryAliasDraftId("wiki"), cfg)).toBe("目录「wiki」的显示名");
  });
});

describe("applyAliasDrafts", () => {
  it("patches an existing row case-insensitively without touching other fields", () => {
    const cfg = [category({ directory: "Projects", alias: "", indexed: false, order: 3 })];
    const next = applyAliasDrafts(cfg, { [categoryAliasDraftId("projects")]: "项目" });
    expect(next).toHaveLength(1);
    expect(next[0]).toMatchObject({ directory: "Projects", alias: "项目", indexed: false, order: 3 });
    // 原数组不被修改。
    expect(cfg[0]?.alias).toBe("");
  });

  it("appends a missing directory with the defaults", () => {
    const next = applyAliasDrafts([], { [categoryAliasDraftId("wiki")]: "维基" });
    expect(next).toEqual([{ directory: "wiki", alias: "维基", area: null, indexed: true, order: 0 }]);
    const dotted = applyAliasDrafts([], { [categoryAliasDraftId(".obsidian")]: "系统" });
    expect(dotted[0]?.indexed).toBe(false);
  });

  it("keeps the default-indexed rule in one place", () => {
    expect(defaultIndexedFor("projects")).toBe(true);
    expect(defaultIndexedFor(".mile-kb")).toBe(false);
  });
});

describe("overlongAliasDrafts", () => {
  it("refuses over 64 characters instead of truncating", () => {
    expect(overlongAliasDrafts({ [categoryAliasDraftId("projects")]: "长".repeat(64) })).toEqual([]);
    expect(overlongAliasDrafts({ [categoryAliasDraftId("projects")]: "长".repeat(65) })).toEqual(["projects"]);
  });
});
