import { describe, expect, it } from "vitest";
import { median, percentile, shouldDegrade } from "./skillsGlass";

describe("skills glass degrade rule (M8 T08)", () => {
  it("computes the nearest-rank percentile and median", () => {
    const gaps = [5, 5.1, 5.2, 10, 20];
    expect(percentile(gaps, 50)).toBe(5.2);
    expect(median(gaps)).toBe(5.2);
    expect(percentile(gaps, 95)).toBe(20);
    expect(percentile([], 95)).toBe(0);
  });

  it("degrades only when p95 and the slow-frame share both exceed the budget", () => {
    const refresh = 5;
    // Smooth: everything at one refresh interval.
    expect(shouldDegrade(Array.from({ length: 120 }, () => 5.1), refresh)).toBe(false);
    // p95 12ms (> 10) with 10% of frames above 10ms → degrade.
    const slow = Array.from({ length: 120 }, (_, index) => (index < 12 ? 15 : 5.1));
    expect(shouldDegrade(slow, refresh)).toBe(true);
    // p95 12ms but only 4% slow frames → keep glass.
    const rare = Array.from({ length: 120 }, (_, index) => (index < 4 ? 15 : 5.1));
    expect(shouldDegrade(rare, refresh)).toBe(false);
    // p95 at exactly 2R is still within budget.
    expect(shouldDegrade([10, 10, 10, 10], refresh)).toBe(false);
    expect(shouldDegrade([], refresh)).toBe(false);
    expect(shouldDegrade([20, 20], 0)).toBe(false);
  });
});
