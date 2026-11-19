// Unsaved settings drafts (M3 T04). Every text control in the settings page
// writes into one draft table keyed by a stable id; the page-level save bar
// saves all of them, and the leave guard counts them. The shape follows the
// legacy settingsDrafts model: drafting is cheap, saving is explicit.
import type { AppConfigResult, CategoryConfig } from "../mygo";
import { STEERING_DEFAULTS } from "./agentDefaults";
import { AGENT_IDS, AGENT_META, type AgentId } from "./agentMeta";

/** Display-name limit for one top-level directory (数据库设计 3.2). */
export const MAX_CATEGORY_ALIAS = 64;

const STEERING_PREFIX = "steering:";
const ALIAS_PREFIX = "category-alias:";

/** Draft id for one agent's steering text. */
export function steeringDraftId(agent: AgentId): string {
  return `${STEERING_PREFIX}${agent}`;
}

/** Draft id for one directory's display name (lower-cased directory). */
export function categoryAliasDraftId(directory: string): string {
  return `${ALIAS_PREFIX}${directory.toLowerCase()}`;
}

/** Every draft that differs from what is saved (or from the built-in default). */
export function dirtyDraftIds(
  drafts: Record<string, string>,
  config: AppConfigResult | null,
): string[] {
  const out: string[] = [];
  for (const [id, text] of Object.entries(drafts)) {
    if (id.startsWith(STEERING_PREFIX)) {
      const agent = id.slice(STEERING_PREFIX.length) as AgentId;
      if (!AGENT_IDS.includes(agent)) continue;
      const saved = config?.agents?.[agent]?.steering ?? STEERING_DEFAULTS[agent];
      if (text !== saved) out.push(id);
      continue;
    }
    if (id.startsWith(ALIAS_PREFIX)) {
      const directory = id.slice(ALIAS_PREFIX.length);
      if (!directory) continue;
      const saved = config?.categories?.find((item) => item.directory.toLowerCase() === directory);
      if (text !== (saved?.alias ?? "")) out.push(id);
    }
  }
  return out;
}

/** Human label for one draft, used by the save bar and the leave dialog. */
export function draftLabel(id: string, config: AppConfigResult | null): string {
  if (id.startsWith(STEERING_PREFIX)) {
    const agent = id.slice(STEERING_PREFIX.length) as AgentId;
    return `${AGENT_META[agent]?.name ?? agent} 的 Steering`;
  }
  if (id.startsWith(ALIAS_PREFIX)) {
    const directory = id.slice(ALIAS_PREFIX.length);
    const saved = config?.categories?.find((item) => item.directory.toLowerCase() === directory);
    return `目录「${saved?.directory ?? directory}」的显示名`;
  }
  return id;
}

/** Drafts grouped by kind for the save paths. */
export function steeringDrafts(drafts: Record<string, string>): Partial<Record<AgentId, string>> {
  const out: Partial<Record<AgentId, string>> = {};
  for (const [id, text] of Object.entries(drafts)) {
    if (!id.startsWith(STEERING_PREFIX)) continue;
    const agent = id.slice(STEERING_PREFIX.length) as AgentId;
    if (AGENT_IDS.includes(agent)) out[agent] = text;
  }
  return out;
}

/** category alias drafts keyed by lower-cased directory. */
export function aliasDrafts(drafts: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [id, text] of Object.entries(drafts)) {
    if (id.startsWith(ALIAS_PREFIX)) out[id.slice(ALIAS_PREFIX.length)] = text;
  }
  return out;
}

/** The indexed default a directory has with no configuration (D-013). */
export function defaultIndexedFor(directory: string): boolean {
  return !directory.startsWith(".");
}

/**
 * Fold alias drafts into the category table before M65. A directory that has
 * no row yet gets one with the defaults, so the new display name survives the
 * Go-side "drop entries equal to the defaults" filter.
 */
export function applyAliasDrafts(
  categories: readonly CategoryConfig[],
  drafts: Record<string, string>,
): CategoryConfig[] {
  const out = categories.map((item) => ({ ...item }));
  for (const [directory, alias] of Object.entries(aliasDrafts(drafts))) {
    const found = out.find((item) => item.directory.toLowerCase() === directory);
    if (found) {
      found.alias = alias;
      continue;
    }
    out.push({
      directory,
      alias,
      area: null,
      indexed: defaultIndexedFor(directory),
      order: out.length,
    });
  }
  return out;
}

/** Alias drafts that exceed the display-name limit (refused, never truncated). */
export function overlongAliasDrafts(drafts: Record<string, string>): string[] {
  return Object.entries(aliasDrafts(drafts))
    .filter(([, alias]) => alias.trim().length > MAX_CATEGORY_ALIAS)
    .map(([directory]) => directory);
}
