import type { SkillScan } from "./transport";

/** The title-bar badge counts what needs attention (PRD FR-SKM-PANEL):
 * copies whose knowledge base moved on, plus broken links. Opening the panel
 * never clears it. */
export function badgeCount(scan: SkillScan | null): number {
  if (!scan) return 0;
  let count = 0;
  for (const group of scan.tools) {
    for (const skill of group.skills) {
      if (skill.origin === "link-broken") count += 1;
      if (skill.origin === "copied" && skill.sync === "vault-updated") count += 1;
    }
  }
  return count;
}

/** Total skills per page, used by the tab labels. */
export function totals(scan: SkillScan | null): { kb: number; tools: number } {
  if (!scan) return { kb: 0, tools: 0 };
  let kb = 0;
  for (const group of scan.kb) kb += group.skills.length;
  let tools = 0;
  for (const group of scan.tools) tools += group.skills.length;
  return { kb, tools };
}
