// Agent settings helpers (T18): model options, chips and the steering draft
// comparison. Pure — the section renders what these return.
import type { AgentConfig, ProviderDraft } from "../mygo";
import { AGENT_IDS, type AgentId } from "./agentMeta";
import { describeExecutionMode, executionModeOfApproval } from "./executionMode";
import { STEERING_DEFAULTS } from "./agentDefaults";

export const EFFORT_LEVELS = ["auto", "disabled", "low", "high", "max"] as const;

export const EFFORT_LABEL: Record<string, string> = {
  auto: "自动",
  disabled: "关闭",
  low: "低",
  high: "高",
  max: "最高",
};

/** Models across the usable (openai) providers, deduplicated in order. */
export function modelOptions(providers: readonly ProviderDraft[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const provider of providers) {
    if (provider.kind !== "openai") continue;
    for (const model of provider.models) {
      if (seen.has(model)) continue;
      seen.add(model);
      out.push(model);
    }
  }
  return out;
}

/** What an empty model setting resolves to: the first provider's default. */
export function defaultModelLabel(providers: readonly ProviderDraft[]): string {
  const first = providers.find((provider) => provider.kind === "openai");
  if (!first) return "引擎默认";
  return first.defaultModel || first.models[0] || "引擎默认";
}

/** The chip under an agent tab: the saved model or "默认模型". */
export function modelChip(model: string | undefined): string {
  return model && model !== "" ? model : "默认模型";
}

export function approvalLabel(approval: string | undefined): string {
  return describeExecutionMode(executionModeOfApproval(approval)).label;
}

/** Agents whose draft differs from the saved steering. */
export function dirtyAgents(
  drafts: Partial<Record<AgentId, string>>,
  agents: Record<string, AgentConfig> | undefined,
): AgentId[] {
  return AGENT_IDS.filter((id) => {
    const draft = drafts[id];
    if (draft === undefined) return false;
    // A missing saved entry means the built-in default is what would be used.
    return draft !== (agents?.[id]?.steering ?? STEERING_DEFAULTS[id]);
  });
}

/** The text shown for one agent: unsaved draft first, then saved, then default. */
export function steeringText(
  drafts: Partial<Record<AgentId, string>>,
  agents: Record<string, AgentConfig> | undefined,
  agent: AgentId,
): string {
  return drafts[agent] ?? agents?.[agent]?.steering ?? STEERING_DEFAULTS[agent];
}
