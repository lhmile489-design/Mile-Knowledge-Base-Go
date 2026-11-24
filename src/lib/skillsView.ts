import type { SkillScan, SkillTool, SkillToolScan } from "./transport";

/** What needs attention in one tool group (M8 T03, same rule as the badge):
 * broken links and copies whose knowledge base moved on. */
export function toolNeedsAttention(group: SkillToolScan): boolean {
  return group.skills.some(
    (skill) => skill.origin === "link-broken" || (skill.origin === "copied" && skill.sync === "vault-updated"),
  );
}

/** Attention counts behind the badge and the skills-page footer. */
export function attentionCounts(scan: SkillScan | null): { updates: number; broken: number } {
  let updates = 0;
  let broken = 0;
  if (!scan) return { updates, broken };
  for (const group of scan.tools) {
    for (const skill of group.skills) {
      if (skill.origin === "link-broken") broken += 1;
      if (skill.origin === "copied" && skill.sync === "vault-updated") updates += 1;
    }
  }
  return { updates, broken };
}

/** The title-bar badge counts what needs attention (PRD FR-SKM-PANEL):
 * copies whose knowledge base moved on, plus broken links. Opening the panel
 * never clears it. */
export function badgeCount(scan: SkillScan | null): number {
  const { updates, broken } = attentionCounts(scan);
  return updates + broken;
}

/** Knowledge-base skills that reached at least one tool, for the navigation
 * footer (UI 稿 11.1 「已分发 17 / 24」). */
export function distributionProgress(scan: SkillScan | null): { distributed: number; total: number } {
  if (!scan) return { distributed: 0, total: 0 };
  let distributed = 0;
  let total = 0;
  for (const group of scan.kb) {
    for (const skill of group.skills) {
      total += 1;
      if ((skill.distributed?.length ?? 0) > 0) distributed += 1;
    }
  }
  return { distributed, total };
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

/** Distribute/install target grid (M6 T08): enabled tools with an install
 * directory, the shared directory first, otherwise registry order — built-in
 * tools, then custom tools, then projects. A registered directory that is gone
 * stays listed so the grid can show why it is disabled. */
export function installTargets(tools: SkillTool[]): SkillTool[] {
  const list = tools.filter(
    (tool) => tool.enabled && (tool.id === "agents" || (tool.status === "detected" && Boolean(tool.installDir))),
  );
  return list.sort((a, b) => (a.id === "agents" ? -1 : b.id === "agents" ? 1 : 0));
}

/** The last path segment of a masked tool path, for the target grid note. */
export function tailOf(path: string | undefined): string {
  if (!path) return "";
  const parts = path.split(/[\/]/).filter(Boolean);
  return parts[parts.length - 1] ?? "";
}
