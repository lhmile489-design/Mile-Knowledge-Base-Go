import { describe, expect, it } from "vitest";
import { ABSOLUTE_P95_MS, median, percentile, shouldDegradeGlobal } from "./effectsGuard";

describe("percentile and median", () => {
  it("computes the nearest-rank percentile and median", () => {
    const gaps = [5, 5.1, 5.2, 10, 20];
    expect(percentile(gaps, 50)).toBe(5.2);
    expect(median(gaps)).toBe(5.2);
    expect(percentile(gaps, 95)).toBe(20);
    expect(percentile([], 95)).toBe(0);
  });
});

describe("global degrade rule (tech design 21.6)", () => {
  const smooth = (count: number, gap = 5.1): number[] => Array.from({ length: count }, () => gap);
  const withSlow = (slow: number, slowGap: number, base = 5.1): number[] =>
    Array.from({ length: 120 }, (_, index) => (index < slow ? slowGap : base));

  it("keeps the blur while frames stay at the refresh interval", () => {
    expect(shouldDegradeGlobal(smooth(120), 5)).toBe(false);
  });

  it("degrades when p95 and the slow-frame share both exceed twice the refresh interval", () => {
    // p95 15 ms (> 10) with 10% of the frames above 10 ms.
    expect(shouldDegradeGlobal(withSlow(12, 15), 5)).toBe(true);
    // Same p95 but only 4% slow frames stays within budget.
    expect(shouldDegradeGlobal(withSlow(4, 15), 5)).toBe(false);
    // p95 at exactly 2R is still within budget.
    expect(shouldDegradeGlobal([10, 10, 10, 10], 5)).toBe(false);
  });

  it("ignores empty samples", () => {
    expect(shouldDegradeGlobal([], 5)).toBe(false);
    expect(shouldDegradeGlobal([], 0)).toBe(false);
  });

  it("uses the scroll median when the idle sample is missing or zero", () => {
    expect(shouldDegradeGlobal([20, 20], 0)).toBe(false);
    expect(shouldDegradeGlobal(withSlow(12, 15), 0)).toBe(true);
  });

  it("takes the smaller of the idle and scroll medians so a throttled idle sample cannot hide jank", () => {
    // M9 T07: an idle sample taken while the window was throttled read 25 ms on a
    // 120 Hz screen. With R = 25 the limit would be 50 ms and 15 ms frames pass.
    const jank = withSlow(12, 30, 8.3);
    expect(shouldDegradeGlobal(jank, 25)).toBe(true);
    // The old rule would have judged this against the idle median alone.
    expect(shouldDegradeGlobal(jank, 8.3)).toBe(true);
  });

  it("catches uniformly slow frames through the absolute p95 cap", () => {
    // Every frame 40 ms: the ratio to the scroll median is 1, only the cap sees it.
    expect(shouldDegradeGlobal(smooth(120, 40), 16.7)).toBe(true);
    expect(shouldDegradeGlobal(smooth(120, 40), 40)).toBe(true);
  });

  it("treats the cap as exclusive", () => {
    expect(ABSOLUTE_P95_MS).toBe(33.4);
    expect(shouldDegradeGlobal(smooth(120, 33.4), 16.7)).toBe(false);
    expect(shouldDegradeGlobal(smooth(120, 33.5), 16.7)).toBe(true);
  });

  it("does not degrade a steady 60 Hz display", () => {
    expect(shouldDegradeGlobal(smooth(120, 16.7), 16.7)).toBe(false);
  });
});
