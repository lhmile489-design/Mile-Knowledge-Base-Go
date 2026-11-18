import { describe, expect, it } from "vitest";
import { newNotePath, newNoteText } from "./wikilink";

describe("newNotePath", () => {
  it("keeps a slash target relative to the vault root", () => {
    expect(newNotePath("raws/新页", "skills/a.md")).toBe("raws/新页.md");
  });
  it("places a bare target next to the current document", () => {
    expect(newNotePath("新页", "raws/a.md")).toBe("raws/新页.md");
  });
  it("uses the root when the current document has no directory", () => {
    expect(newNotePath("新页", "a.md")).toBe("新页.md");
  });
  it("keeps an existing extension", () => {
    expect(newNotePath("笔记.txt", "raws/a.md")).toBe("raws/笔记.txt");
  });
  it("drops queries and anchors", () => {
    expect(newNotePath("新页#段", "raws/a.md")).toBe("raws/新页.md");
  });
  it("rejects escaping and illegal targets", () => {
    // "a " trims to "a" and is legal (the name layer trims first, same as the
    // legacy entry-name check); a backslash is a path separator, not illegal.
    for (const bad of ["../x", "/abs", "C:/x", "a*b", "a|b", "a.", "", "a//b"]) {
      expect(newNotePath(bad, "raws/a.md")).toBeNull();
    }
  });
});

describe("newNoteText", () => {
  it("uses the file stem as the heading", () => {
    expect(newNoteText("raws/设计原则.md")).toBe("# 设计原则\n");
  });
});

import { wikilinkAt } from "./wikilink";

describe("wikilinkAt", () => {
  const text = "前 [[甲]] 后 [[乙|别名]]";
  it("finds the link containing the offset", () => {
    expect(wikilinkAt(text, 4)).toBe("甲");
    expect(wikilinkAt(text, text.indexOf("别名"))).toBe("乙");
  });
  it("returns null outside a link", () => {
    expect(wikilinkAt(text, 0)).toBeNull();
    expect(wikilinkAt("no links", 3)).toBeNull();
  });
});
