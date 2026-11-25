import { describe, expect, it } from "vitest";
import {
  attentionCounts,
  badgeCount,
  distributionLabel,
  distributionProgress,
  filterParts,
  isToolDetected,
  toolNeedsAttention,
} from "./skillsView";
import type { SkillScan, SkillToolScan, ToolSkill } from "./transport";

function tool(toolId: string, skills: Partial<ToolSkill>[]): SkillToolScan {
  return { toolId, skills: skills as ToolSkill[] };
}

function scan(overrides: Partial<SkillScan> = {}): SkillScan {
  return { cachedAt: "2026-11-24T09:00:00Z", scanning: false, kb: [], tools: [], ...overrides };
}

describe("skillsView helpers (M8 T03)", () => {
  it("flags a tool group when a link is broken or a copy is behind the vault", () => {
    expect(toolNeedsAttention(tool("a", [{ origin: "link-broken" }]))).toBe(true);
    expect(toolNeedsAttention(tool("b", [{ origin: "copied", sync: "vault-updated" }]))).toBe(true);
    expect(toolNeedsAttention(tool("c", [{ origin: "linked" }, { origin: "copied", sync: "in-sync" }]))).toBe(false);
    expect(toolNeedsAttention(tool("d", []))).toBe(false);
  });

  it("counts updates and broken links the same way as the title-bar badge", () => {
    const payload = scan({
      tools: [
        tool("a", [{ origin: "link-broken" }, { origin: "copied", sync: "vault-updated" }]),
        tool("b", [{ origin: "copied", sync: "vault-updated" }, { origin: "linked" }]),
      ],
    });
    expect(attentionCounts(payload)).toEqual({ updates: 2, broken: 1 });
    expect(badgeCount(payload)).toBe(3);
    expect(attentionCounts(null)).toEqual({ updates: 0, broken: 0 });
    expect(badgeCount(null)).toBe(0);
  });

  it("measures how many knowledge-base skills reached at least one tool", () => {
    const payload = scan({
      kb: [
        {
          category: "前端",
          skills: [
            { name: "vue", description: "", path: "skills/前端/vue", category: "前端", distributed: [{ toolId: "a", mode: "link" }] },
            { name: "react", description: "", path: "skills/前端/react", category: "前端", distributed: [] },
          ],
        },
        {
          category: "后端",
          skills: [{ name: "go", description: "", path: "skills/后端/go", category: "后端" }],
        },
      ],
    });
    expect(distributionProgress(payload)).toEqual({ distributed: 1, total: 3 });
    expect(distributionProgress(null)).toEqual({ distributed: 0, total: 0 });
  });
});

describe("isToolDetected (M9 T05, shared by the nav and the tools view)", () => {
  const tool = (extra: Record<string, unknown>) =>
    ({ id: "x", name: "X", iconKey: "", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true, ...extra }) as never;

  it("is true for a detected, enabled tool", () => {
    expect(isToolDetected(tool({}))).toBe(true);
  });

  it("is false for a missing or disabled tool", () => {
    expect(isToolDetected(tool({ status: "missing" }))).toBe(false);
    expect(isToolDetected(tool({ enabled: false }))).toBe(false);
  });

  it("always counts the shared directory", () => {
    expect(isToolDetected(tool({ id: "agents", status: "shared", enabled: false }))).toBe(true);
  });
});

describe("filterParts (M9 T05)", () => {
  it("returns one plain part without a needle", () => {
    expect(filterParts("Claude Code", "")).toEqual([{ text: "Claude Code", hit: false }]);
  });

  it("splits around the first case-insensitive match", () => {
    expect(filterParts("Claude Code", "cod")).toEqual([
      { text: "Claude ", hit: false },
      { text: "Cod", hit: true },
      { text: "e", hit: false },
    ]);
  });

  it("marks the whole label and drops empty parts", () => {
    expect(filterParts("go", "go")).toEqual([{ text: "go", hit: true }]);
  });

  it("returns the label untouched when nothing matches", () => {
    expect(filterParts("前端", "zzz")).toEqual([{ text: "前端", hit: false }]);
  });
});

describe("distributionLabel (M9 T09 review)", () => {
  it("marks a copy whose vault side is gone as an orphan, not in-sync", () => {
    const label = distributionLabel({ toolId: "a", mode: "copy", sync: "vault-missing" });
    expect(label.sync).not.toBe("已同步");
    expect(label.tone).toBe("fault");
  });

  it("keeps the in-sync copy and the link rows unchanged", () => {
    expect(distributionLabel({ toolId: "a", mode: "copy", sync: "in-sync" }).sync).toBe("已同步");
    expect(distributionLabel({ toolId: "a", mode: "link" }).sync).toBe("已同步");
  });
});
