import { describe, expect, it } from "vitest";
import type { KbSkill, SkillScan, SkillToolScan } from "./transport";
import { mergeScan } from "./scanMerge";

function kbSkill(name: string, extra: Partial<KbSkill> = {}): KbSkill {
  return { name, description: "desc " + name, path: "skills/cat/" + name, category: "cat", distributed: [], ...extra };
}

function scanOf(skills: KbSkill[], opts: Partial<SkillScan> = {}): SkillScan {
  return {
    cachedAt: opts.cachedAt ?? "2026-11-28T10:00:00Z",
    scanning: opts.scanning ?? false,
    kb: skills.length ? [{ category: "cat", skills }] : [],
    tools: (opts.tools as SkillToolScan[] | undefined) ?? [],
    warnings: opts.warnings as SkillScan["warnings"],
  };
}

describe("mergeScan (M10.1 P03)", () => {
  it("keeps the same object when only cachedAt differs", () => {
    const previous = scanOf([kbSkill("a"), kbSkill("b")], { cachedAt: "2026-11-28T10:00:00Z" });
    const incoming = scanOf([kbSkill("a"), kbSkill("b")], { cachedAt: "2026-11-28T10:00:01Z" });
    expect(mergeScan(previous, incoming)).toBe(previous);
  });

  it("returns the previous scan untouched when it is null", () => {
    const incoming = scanOf([kbSkill("a")]);
    expect(mergeScan(null, incoming)).toBe(incoming);
  });

  it("is a new scan object when the scanning flag changes", () => {
    const previous = scanOf([kbSkill("a")], { scanning: true });
    const incoming = scanOf([kbSkill("a")], { scanning: false });
    const merged = mergeScan(previous, incoming);
    expect(merged).not.toBe(previous);
    expect(merged.scanning).toBe(false);
  });

  it("keeps group and skill identities for unchanged subtrees when one skill changes", () => {
    const a = kbSkill("a");
    const b = kbSkill("b");
    const c = kbSkill("c");
    const previous: SkillScan = {
      cachedAt: "t0",
      scanning: false,
      kb: [{ category: "cat", skills: [a, b] }, { category: "other", skills: [c] }],
      tools: [],
    };
    const bNext = { ...b, distributed: [{ toolId: "claude-code", mode: "link" }] };
    const incoming: SkillScan = {
      cachedAt: "t1",
      scanning: false,
      kb: [{ category: "cat", skills: [{ ...a }, bNext] }, { category: "other", skills: [{ ...c }] }],
      tools: [],
    };
    const merged = mergeScan(previous, incoming);
    expect(merged).not.toBe(previous);
    expect(merged.kb[1]).toBe(previous.kb[1]);
    const mergedCat = merged.kb[0]!;
    expect(mergedCat).not.toBe(previous.kb[0]);
    expect(mergedCat.skills[0]).toBe(a);
    expect(mergedCat.skills[1]).not.toBe(b);
    expect(mergedCat.skills[1]).toBe(bNext);
  });

  it("treats distributed order as significant", () => {
    const one = kbSkill("a", { distributed: [{ toolId: "x", mode: "link" }, { toolId: "y", mode: "link" }] });
    const previous = scanOf([one]);
    const swapped = kbSkill("a", { distributed: [{ toolId: "y", mode: "link" }, { toolId: "x", mode: "link" }] });
    const incoming = scanOf([{ ...swapped }]);
    expect(mergeScan(previous, incoming)).not.toBe(previous);
  });

  it("keeps tool scan groups and skills stable when unchanged", () => {
    const tool: SkillToolScan = {
      toolId: "claude-code",
      skills: [{ name: "t", description: "d", path: "skills/x/t", toolId: "claude-code", origin: "linked" }],
    };
    const previous: SkillScan = { cachedAt: "t0", scanning: false, kb: [], tools: [tool] };
    const incoming: SkillScan = {
      cachedAt: "t1",
      scanning: false,
      kb: [],
      tools: [{ toolId: "claude-code", skills: [{ ...tool.skills[0]! }] }],
    };
    const merged = mergeScan(previous, incoming);
    expect(merged).toBe(previous);
  });
});
