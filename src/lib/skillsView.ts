import type { SkillScan, SkillTool } from "./transport";

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
