import { describe, expect, it } from "vitest";
import { lineDiff, PREVIEW_MAX_LINES, previewDiff } from "./lineDiff";

// Ported from tests/unit/lineDiff.test.ts (legacy v2.7.0, 10 cases).

describe("lineDiff", () => {
  it("marks removed and added lines", () => {
    const diff = lineDiff("a\nb\nc", "a\nx\nc");
    expect(diff).toEqual([
      { kind: "same", text: "a" },
      { kind: "removed", text: "b" },
      { kind: "added", text: "x" },
      { kind: "same", text: "c" },
    ]);
  });

  it("returns a single same row for identical single-line text", () => {
    expect(lineDiff("same", "same")).toEqual([{ kind: "same", text: "same" }]);
  });

  it("returns nothing for two empty inputs", () => {
    expect(lineDiff("", "")).toEqual([]);
  });

  it("treats an empty side as pure addition or removal", () => {
    // "" splits into one empty line, so the empty side shows up as an empty row.
    expect(lineDiff("", "a\nb")).toEqual([
      { kind: "removed", text: "" },
      { kind: "added", text: "a" },
      { kind: "added", text: "b" },
    ]);
    expect(lineDiff("a\nb", "")).toEqual([
      { kind: "removed", text: "a" },
      { kind: "removed", text: "b" },
      { kind: "added", text: "" },
    ]);
  });

  it("keeps equal lines aligned around a change", () => {
    const diff = lineDiff("1\n2\n3\n4", "1\n2\nX\n4");
    expect(diff.filter((line) => line.kind === "same").map((line) => line.text)).toEqual(["1", "2", "4"]);
  });

  it("handles trailing newlines as an empty final line", () => {
    const diff = lineDiff("a\n", "a");
    expect(diff).toEqual([
      { kind: "same", text: "a" },
      { kind: "removed", text: "" },
    ]);
  });

  it("degrades to removed-then-added for huge inputs", () => {
    const big = Array.from({ length: 2100 }, (_, i) => `line-${i}`).join("\n");
    const diff = lineDiff(big, big + "\nextra");
    expect(diff.filter((line) => line.kind === "same")).toHaveLength(0);
    expect(diff[0]!.kind).toBe("removed");
  });

  it("falls back to a summary when the diff would exceed the line budget", () => {
    const mine = Array.from({ length: 40 }, (_, i) => `old-${i}`).join("\n");
    const disk = Array.from({ length: 40 }, (_, i) => `new-${i}`).join("\n");
    expect(previewDiff(mine, disk)).toBeNull();
    expect(lineDiff(mine, disk).length).toBeGreaterThan(PREVIEW_MAX_LINES);
  });

  it("clips overly long single lines", () => {
    const long = "x".repeat(500);
    const preview = previewDiff(long, "short");
    expect(preview).not.toBeNull();
    for (const line of preview!) {
      expect(line.text.length).toBeLessThanOrEqual(201);
    }
  });

  it("keeps short diffs intact", () => {
    const preview = previewDiff("a\nb", "a\nc");
    expect(preview).toEqual([
      { kind: "same", text: "a" },
      { kind: "removed", text: "b" },
      { kind: "added", text: "c" },
    ]);
  });
});
