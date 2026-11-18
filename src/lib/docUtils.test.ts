import { describe, expect, it } from "vitest";
import { affectedBy, minimalChange } from "./docUtils";

describe("affectedBy", () => {
  it("matches the parent directory of the file", () => {
    expect(affectedBy(["raws"], "raws/a.md")).toBe(true);
    expect(affectedBy(["skills/react"], "skills/react/a.md")).toBe(true);
    expect(affectedBy(["skills"], "skills/react/a.md")).toBe(true);
  });

  it("ignores unrelated directories", () => {
    expect(affectedBy(["projects"], "raws/a.md")).toBe(false);
    expect(affectedBy(["skills/react2"], "skills/react/a.md")).toBe(false);
  });

  it("treats the root as affecting everything", () => {
    expect(affectedBy([""], "deep/dir/a.md")).toBe(true);
  });
});

describe("minimalChange", () => {
  it("reports a pure append as an insert at the end", () => {
    expect(minimalChange("abc", "abcdef")).toEqual({ from: 3, to: 3, insert: "def" });
  });

  it("reports a middle replacement", () => {
    expect(minimalChange("aXc", "aYc")).toEqual({ from: 1, to: 2, insert: "Y" });
  });

  it("handles identical text", () => {
    expect(minimalChange("same", "same")).toEqual({ from: 4, to: 4, insert: "" });
  });
});
