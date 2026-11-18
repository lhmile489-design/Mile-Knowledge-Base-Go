import { describe, expect, it } from "vitest";
import {
  executionModeOfApproval,
  EXECUTION_MODE_TO_APPROVAL,
  describeExecutionMode,
  normalizeExecutionMode,
} from "./executionMode";
import { resolveFollowUpRoute } from "./routing";
import { formatTokenCount, usageSummary } from "./usageFormat";
import type { UsageFrame } from "../mygo";

describe("execution modes", () => {
  it("maps UI modes to protocol values and back", () => {
    expect(EXECUTION_MODE_TO_APPROVAL.manual).toBe("ask");
    expect(EXECUTION_MODE_TO_APPROVAL.full).toBe("yolo");
    expect(executionModeOfApproval("yolo")).toBe("full");
    expect(executionModeOfApproval("bogus")).toBe("manual");
  });

  it("unknown values degrade to the strictest mode", () => {
    expect(normalizeExecutionMode("weird")).toBe("manual");
    expect(normalizeExecutionMode("auto")).toBe("auto");
  });

  it("only full carries a consequence line", () => {
    expect(describeExecutionMode("full").consequence).toContain("不再弹出审批卡");
    expect(describeExecutionMode("manual").consequence).toBeNull();
    expect(describeExecutionMode("auto").label).toBe("自动执行");
  });
});

describe("follow-up routing", () => {
  const base = {
    decisionAgent: "distiller" as const,
    override: null,
    currentTarget: "curator" as const,
    hasCurrentConversation: true,
    rerouteRequested: false,
  };

  it("an explicit override always wins", () => {
    expect(resolveFollowUpRoute({ ...base, override: "scout" })).toEqual({
      agent: "scout",
      kind: "override",
    });
  });

  it("no conversation or an explicit auto re-routes", () => {
    expect(resolveFollowUpRoute({ ...base, hasCurrentConversation: false })).toEqual({
      agent: "distiller",
      kind: "auto",
    });
    expect(resolveFollowUpRoute({ ...base, rerouteRequested: true })).toEqual({
      agent: "distiller",
      kind: "auto",
    });
  });

  it("inside a conversation the current agent continues", () => {
    expect(resolveFollowUpRoute(base)).toEqual({ agent: "curator", kind: "continue" });
  });
});

describe("usage summary", () => {
  it("formats compact token counts", () => {
    expect(formatTokenCount(0)).toBe("0");
    expect(formatTokenCount(999)).toBe("999");
    expect(formatTokenCount(1200)).toBe("1.2k");
    expect(formatTokenCount(12_000)).toBe("12k");
    expect(formatTokenCount(2_400_000)).toBe("2.4M");
  });

  it("no usage means no claim", () => {
    expect(usageSummary(null)).toBeNull();
    const usage = {
      turn: { totalTokens: 1500, promptTokens: 1000, completionTokens: 500, reasoningTokens: 0, cacheHitTokens: 0, cacheMissTokens: 0 },
      cumulative: { totalTokens: 1500, promptTokens: 1000, completionTokens: 500, reasoningTokens: 0, cacheHitTokens: 0, cacheMissTokens: 0 },
    } as UsageFrame;
    expect(usageSummary(usage)).toBe("本轮 1.5k tokens");
  });
});
