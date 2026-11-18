import { describe, expect, it } from "vitest";
import type { Usage } from "../mygo";
import {
  formatCacheHit,
  formatCost,
  formatTokens,
  UNKNOWN_MARK,
} from "./usageFormat";

function usage(partial: Partial<Usage> = {}): Usage {
  return {
    totalTokens: 1400,
    promptTokens: 1000,
    completionTokens: 400,
    reasoningTokens: 100,
    cacheHitTokens: 800,
    cacheMissTokens: 200,
    ...partial,
  };
}

describe("usage receipt formatters", () => {
  it("token counts use thousands separators, unknown is an em dash", () => {
    expect(formatTokens(1400)).toBe("1,400");
    expect(formatTokens(0)).toBe("0");
    expect(formatTokens(undefined)).toBe(UNKNOWN_MARK);
    expect(formatTokens(Number.NaN)).toBe(UNKNOWN_MARK);
  });

  it("a missing cost says 未提供, never zero", () => {
    const cost = formatCost(usage());
    expect(cost).toMatchObject({ text: "— 未提供", known: false });
  });

  it("known costs keep their currency and flagged state", () => {
    expect(formatCost(usage({ estimatedCost: 0.42, currency: "$" }))).toMatchObject({
      text: "$0.420",
      known: true,
    });
    expect(
      formatCost(usage({ estimatedCost: 0.0001, currency: "$", estimated: true })),
    ).toMatchObject({ text: "$0.00010（估算）", estimated: true });
    expect(
      formatCost(usage({ estimatedCost: 1.5, estimated: true, costComplete: false })),
    ).toMatchObject({ text: "1.50（估算，不完整）", complete: false });
  });

  it("cache hit ratio prefers the engine value and derives it otherwise", () => {
    expect(formatCacheHit(usage({ cacheHitRatio: 0.8 }))).toEqual({ text: "80%", known: true });
    expect(formatCacheHit(usage())).toEqual({ text: "80%", known: true });
    expect(
      formatCacheHit(usage({ cacheHitTokens: 0, cacheMissTokens: 0 })),
    ).toEqual({ text: UNKNOWN_MARK, known: false });
    expect(formatCacheHit(usage({ cacheHitRatio: 1.4 }))).toEqual({ text: "100%", known: true });
  });
});
