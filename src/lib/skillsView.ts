import type { KbSkill, KbSkillDistribution, SkillScan, SkillTool, SkillToolScan } from "./transport";

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

/** The status pill of a knowledge-base skill (cards and the detail drawer). */
export function skillStatus(skill: KbSkill): { label: string; tone: "live" | "warn" | "idle" } {
  if (!skill.distributed || skill.distributed.length === 0) return { label: "未分发", tone: "idle" };
  if (skill.distributed.some((item) => item.sync === "vault-updated")) {
    return { label: "知识库有更新", tone: "warn" };
  }
  return { label: `已分发 ${skill.distributed.length} 处`, tone: "live" };
}

/** One distribution row of the drawer: method and sync state of that tool. */
export function distributionLabel(item: KbSkillDistribution): {
  how: string;
  sync: string;
  tone: "live" | "warn" | "fault";
} {
  const how = item.mode === "link" ? "链接" : "拷贝";
  if (item.mode === "link") return { how, sync: "已同步", tone: "live" };
  switch (item.sync) {
    case "vault-updated":
      return { how, sync: "知识库有更新", tone: "warn" };
    case "tool-updated":
      return { how, sync: "工具里改过", tone: "warn" };
    case "both-updated":
      return { how, sync: "两边都改了", tone: "warn" };
    case "missing":
      return { how, sync: "目录缺失", tone: "fault" };
    default:
      return { how, sync: "已同步", tone: "live" };
  }
}

/** A tool counts as detected when it is enabled and its directory exists; the
 * shared directory is always shown. The navigation and the tools view fold the
 * rest under 「未检测到的工具」 (M9 T05). */
export function isToolDetected(tool: SkillTool): boolean {
  return tool.id === "agents" || (tool.enabled && tool.status === "detected");
}

/** Splits a label around the first case-insensitive match of `needle`, so the
 * navigation can highlight it without building HTML. */
export function filterParts(label: string, needle: string): { text: string; hit: boolean }[] {
  const wanted = needle.trim().toLowerCase();
  const at = wanted ? label.toLowerCase().indexOf(wanted) : -1;
  if (at < 0) return [{ text: label, hit: false }];
  const end = at + wanted.length;
  return [
    { text: label.slice(0, at), hit: false },
    { text: label.slice(at, end), hit: true },
    { text: label.slice(end), hit: false },
  ].filter((part) => part.text.length > 0);
}
