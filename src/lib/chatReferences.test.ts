import { describe, expect, it } from "vitest";
import { asPathReference, collectEvidence, findReferences, isPlausibleRelativePath } from "./chatReferences";
import { chooseWikiCandidate } from "./openReference";

describe("reference syntax", () => {
  it("finds vault paths and wikilinks in order", () => {
    const refs = findReferences("看 skills/react/SKILL.md，再看 [[笔记|别名]] 与 raws/a.txt");
    expect(refs.map((ref) => [ref.refKind, ref.target])).toEqual([
      ["path", "skills/react/SKILL.md"],
      ["wiki", "笔记"],
      ["path", "raws/a.txt"],
    ]);
    expect(refs[1]?.label).toBe("别名");
  });

  it("rejects absolute, drive, dotdot and URL fragments", () => {
    for (const text of ["C:/a.md", "/root/a.md", "../a.md", "https://a.com/x.md", "a\\b.md"]) {
      expect(findReferences(text)).toHaveLength(0);
    }
    expect(isPlausibleRelativePath("a/../b.md")).toBe(false);
    expect(isPlausibleRelativePath("a/b.md")).toBe(true);
  });

  it("does not treat overlapping path text inside wikilinks twice", () => {
    const refs = findReferences("[[skills/react/SKILL.md]]");
    expect(refs).toHaveLength(1);
    expect(refs[0]?.refKind).toBe("wiki");
  });

  it("inline code is a reference only when it is exactly the path", () => {
    expect(asPathReference("docs/plan.md")).toBe("docs/plan.md");
    expect(asPathReference(" docs/plan.md")).toBeNull();
    expect(asPathReference("见 docs/plan.md")).toBeNull();
    expect(asPathReference("docs/plan.png")).toBeNull();
  });
});

describe("evidence radar data", () => {
  it("counts only agent lines, dedupes by kind and target, numbers by first appearance", () => {
    const evidence = collectEvidence([
      { role: "user", text: "用户粘贴 raws/ignored.md" },
      { role: "agent", text: "依据 skills/a/SKILL.md 与 [[笔记]]" },
      { role: "agent", text: "再看 skills/a/SKILL.md" },
    ]);
    expect(evidence).toHaveLength(2);
    expect(evidence[0]).toMatchObject({ refKind: "path", target: "skills/a/SKILL.md", count: 2, index: 1 });
    expect(evidence[1]).toMatchObject({ refKind: "wiki", target: "笔记", count: 1, index: 2 });
  });
});

describe("wikilink resolution", () => {
  it("matches exact paths, file names and base names", () => {
    expect(chooseWikiCandidate(["a/note.md"], "a/note.md")).toBe("a/note.md");
    expect(chooseWikiCandidate(["a/note.md"], "note.md")).toBe("a/note.md");
    expect(chooseWikiCandidate(["a/note.md"], "note")).toBe("a/note.md");
  });

  it("several candidates are ambiguous, none is missing", () => {
    expect(chooseWikiCandidate(["a/dup.md", "b/dup.md"], "dup")).toBe("ambiguous");
    expect(chooseWikiCandidate(["a/note.md"], "other")).toBeNull();
    expect(chooseWikiCandidate([], "")).toBeNull();
  });
});
