import { describe, expect, it } from "vitest";
import type { AgentConfig, ProviderDraft } from "../mygo";
import { STEERING_DEFAULTS, STEERING_MAX_CHARS } from "./agentDefaults";
import {
  approvalLabel,
  defaultModelLabel,
  dirtyAgents,
  modelChip,
  modelOptions,
  steeringText,
} from "./agentSettings";

function provider(partial: Partial<ProviderDraft> = {}): ProviderDraft {
  return {
    name: "fake",
    kind: "openai",
    baseUrl: "https://api.example.com",
    models: ["m1", "m2"],
    defaultModel: "m1",
    apiKeyEnv: "KEY",
    ...partial,
  };
}

function config(partial: Partial<AgentConfig> = {}): AgentConfig {
  return {
    model: "",
    effort: "high",
    approval: "ask",
    skillPaths: [],
    steering: "已保存",
    steeringBaseVersion: 1,
    ...partial,
  };
}

describe("agent settings helpers", () => {
  it("model options come from openai providers only, deduplicated", () => {
    const providers = [provider(), provider({ name: "b", models: ["m2", "m3"] }), provider({ name: "c", kind: "anthropic", models: ["x"] })];
    expect(modelOptions(providers)).toEqual(["m1", "m2", "m3"]);
  });

  it("the default label follows the first provider's default model", () => {
    expect(defaultModelLabel([provider()])).toBe("m1");
    expect(defaultModelLabel([])).toBe("引擎默认");
    expect(defaultModelLabel([provider({ defaultModel: "", models: ["m9"] })])).toBe("m9");
  });

  it("chips and approval labels are readable", () => {
    expect(modelChip("")).toBe("默认模型");
    expect(modelChip("deepseek-v4-pro")).toBe("deepseek-v4-pro");
    expect(approvalLabel("ask")).toBe("手动执行");
    expect(approvalLabel("yolo")).toBe("全自动");
    expect(approvalLabel(undefined)).toBe("手动执行");
  });

  it("dirty drafts compare against the saved steering", () => {
    const agents = { curator: config({ steering: "已保存" }) };
    expect(dirtyAgents({ curator: "已保存" }, agents)).toEqual([]);
    expect(dirtyAgents({ curator: "草稿" }, agents)).toEqual(["curator"]);
    expect(dirtyAgents({ scout: "" }, agents)).toEqual(["scout"]);
    expect(dirtyAgents({}, agents)).toEqual([]);
  });

  it("steering text falls back from draft to saved to default", () => {
    const agents = { curator: config({ steering: "已保存" }) };
    expect(steeringText({ curator: "草稿" }, agents, "curator")).toBe("草稿");
    expect(steeringText({}, agents, "curator")).toBe("已保存");
    expect(steeringText({}, agents, "scout")).toBe(STEERING_DEFAULTS.scout);
  });

  it("the four defaults exist and respect the length cap", () => {
    for (const text of Object.values(STEERING_DEFAULTS)) {
      expect(text.length).toBeGreaterThan(0);
      expect(text.length).toBeLessThanOrEqual(STEERING_MAX_CHARS);
    }
  });
});
