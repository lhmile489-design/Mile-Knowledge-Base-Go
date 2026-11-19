// Skill import wizard rules (M3 T08; PRD FR-SKILL). The Go side is the
// authority; these checks only keep the button honest before the round trip,
// with the same wording the legacy wizard used.

/** Category and skill names cap (legacy MAX_NAME_LENGTH). */
export const SKILL_NAME_MAX = 80;

/** Legacy skip reasons, in the wizard's own words. */
export const SKIP_REASON: Record<string, string> = {
  exists: "同名技能已存在，未覆盖",
  unknown: "不在所选来源中",
  "too-large": "文件过多或过大",
  failed: "复制失败",
};

/** The empty-source hint shown when the folder holds no SKILL.md. */
export const EMPTY_SOURCE_HINT =
  "这个文件夹里没有找到技能。技能是含 SKILL.md 的文件夹，请选择技能所在的上一级目录。";

/** Returns the problem with a category name, or null when it looks usable. */
export function categoryProblem(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) return "分类名不能为空";
  if (trimmed.startsWith(".")) return "分类名不能以点开头";
  if (/[\\/:*?"<>|]/.test(trimmed)) return '分类名不能包含 \\ / : * ? " < > |';
  if ([...trimmed].length > SKILL_NAME_MAX) return `分类名不能超过 ${SKILL_NAME_MAX} 个字符`;
  return null;
}

/** Human text for one skipped entry. */
export function skipLabel(reason: string): string {
  return SKIP_REASON[reason] ?? reason;
}

/** Summary line for the done step. */
export function importSummary(imported: readonly string[]): string {
  return imported.length === 0 ? "没有导入任何技能" : `已导入 ${imported.length} 个技能`;
}
