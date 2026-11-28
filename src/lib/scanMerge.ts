import type { SkillScan } from "./transport";

/** Structural merge for the skills scan payload (M10.1 P03, PLAN-M10.1 R2).
 *
 * The Go side rebuilds every scan result from scratch, so even an unchanged
 * payload arrives as new objects; handing them straight to zustand re-renders
 * the whole page — in the middle of the entrance animation. This merge returns
 * the previous object when nothing but `cachedAt` changed; when something did
 * change, it reuses the identities of unchanged subtrees, so React only
 * re-renders what actually changed. Group objects are shallow shells and may
 * be recreated; leaf entities (skills, tool skills) are never rebuilt — an
 * unchanged one keeps its reference, a changed one is the incoming object. */

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function sameJson(a: unknown, b: unknown): boolean {
  if (a === b) return true;
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let index = 0; index < a.length; index += 1) {
      if (!sameJson(a[index], b[index])) return false;
    }
    return true;
  }
  if (isPlainObject(a) && isPlainObject(b)) {
    const keysA = Object.keys(a);
    const keysB = Object.keys(b);
    if (keysA.length !== keysB.length) return false;
    for (const key of keysA) {
      if (!(key in b)) return false;
      if (!sameJson((a as Record<string, unknown>)[key], (b as Record<string, unknown>)[key])) return false;
    }
    return true;
  }
  return false;
}

/** Index-aligned leaf merge: an unchanged element keeps the previous reference
 * and a changed element is the incoming object (never rebuilt). */
function mergeEntities<T>(previous: T[] | undefined, next: T[] | undefined): { items: T[]; same: boolean } {
  if (!previous || !next) return { items: (next ?? previous ?? []) as T[], same: previous === next || previous?.length === next?.length };
  if (previous.length !== next.length) return { items: next.slice(), same: false };
  const items: T[] = [];
  let same = true;
  for (let index = 0; index < next.length; index += 1) {
    if (sameJson(previous[index], next[index])) items.push(previous[index] as T);
    else {
      same = false;
      items.push(next[index] as T);
    }
  }
  return { items: same ? previous : items, same };
}

/** A group shell (kb category or tool scan) is cheap to recreate; what matters
 * is that unchanged skills keep their references inside it. */
function mergeGroup<G extends { skills: unknown[] }>(previous: G, next: G): G {
  if (sameJson(previous, next)) return previous;
  const skills = mergeEntities(previous.skills, next.skills);
  return { ...next, skills: skills.items };
}

export function mergeScan(previous: SkillScan | null, incoming: SkillScan): SkillScan {
  if (!previous) return incoming;
  const kb = mergeEntities(previous.kb, incoming.kb);
  const tools = mergeEntities(previous.tools, incoming.tools);
  if (kb.same && tools.same) {
    // Groups were structurally identical — but check leaf equality too, since
    // mergeEntities already compared every entry; kb.same means the whole kb
    // tree is unchanged.
    if (previous.scanning === incoming.scanning && sameJson(previous.warnings ?? null, incoming.warnings ?? null)) {
      return previous;
    }
  }
  const kbOut: SkillScan["kb"] = previous.kb.length === incoming.kb.length
    ? previous.kb.map((group, index) => mergeGroup(group, incoming.kb[index]!))
    : incoming.kb;
  const toolsOut: SkillScan["tools"] = previous.tools.length === incoming.tools.length
    ? previous.tools.map((group, index) => mergeGroup(group, incoming.tools[index]!))
    : incoming.tools;
  return {
    cachedAt: incoming.cachedAt,
    scanning: incoming.scanning,
    kb: kbOut,
    tools: toolsOut,
    warnings: incoming.warnings,
  };
}
