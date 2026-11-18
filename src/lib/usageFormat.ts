// Compact token counts for the runtime-parameter summary. "Not provided"
// stays null — the receipt shows nothing rather than a fabricated zero.
import type { UsageFrame } from "../mygo";

export function formatTokenCount(value: number): string {
  if (!Number.isFinite(value) || value < 0) return "0";
  if (value < 1000) return String(Math.round(value));
  if (value < 1_000_000) {
    return `${(value / 1000).toFixed(1).replace(/\.0$/, "")}k`;
  }
  return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, "")}M`;
}

export function usageSummary(usage: UsageFrame | null): string | null {
  if (!usage) return null;
  return `本轮 ${formatTokenCount(usage.turn.totalTokens)} tokens`;
}
