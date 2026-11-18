// Compact token counts for the runtime-parameter summary. "Not provided"
// stays null — the receipt shows nothing rather than a fabricated zero.
import type { Usage, UsageFrame } from "../mygo";

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

// ---- Usage receipt formatters (ported from shared/usage.ts) ----
//
// "Unknown" is not "zero": a missing cost renders as "— 未提供", never
// $0.00. Costs come from the engine; this module never multiplies tokens by
// a price list. Only the cache hit ratio is derived locally (hit/miss are
// both real numbers, and a ratio is not money).

export const UNKNOWN_MARK = "—";

export interface FormattedValue {
  readonly text: string;
  readonly known: boolean;
}

export interface FormattedCost extends FormattedValue {
  readonly estimated: boolean;
  readonly complete: boolean;
}

/** Thousands separators; null/undefined is unknown. */
export function formatTokens(value: number | null | undefined): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return UNKNOWN_MARK;
  return Math.trunc(value).toLocaleString("en-US");
}

function pickDigits(cost: number): number {
  const magnitude = Math.abs(cost);
  if (magnitude === 0) return 2;
  if (magnitude >= 1) return 2;
  if (magnitude >= 0.01) return 3;
  if (magnitude >= 0.0001) return 5;
  return 8;
}

/** Engine-provided cost with adaptive precision; estimated/incomplete flags
 * stay visible. */
export function formatCost(usage: Usage): FormattedCost {
  const cost = usage.estimatedCost;
  const estimated = usage.estimated === true;
  const complete = usage.costComplete !== false;
  if (typeof cost !== "number" || !Number.isFinite(cost)) {
    return { text: `${UNKNOWN_MARK} 未提供`, known: false, estimated, complete };
  }
  const currency = typeof usage.currency === "string" ? usage.currency : "";
  const amount = cost.toFixed(pickDigits(cost));
  const notes: string[] = [];
  if (estimated) notes.push("估算");
  if (!complete) notes.push("不完整");
  const suffix = notes.length > 0 ? `（${notes.join("，")}）` : "";
  return { text: `${currency}${amount}${suffix}`, known: true, estimated, complete };
}

function toPercent(ratio: number): number {
  return Math.max(0, Math.min(100, Math.round(ratio * 100)));
}

/** Cache hit ratio: prefer the engine's value, else hit/miss; both zero is
 * unknown, not "0%". */
export function formatCacheHit(usage: Usage): FormattedValue {
  if (typeof usage.cacheHitRatio === "number" && Number.isFinite(usage.cacheHitRatio)) {
    return { text: `${toPercent(usage.cacheHitRatio)}%`, known: true };
  }
  const total = usage.cacheHitTokens + usage.cacheMissTokens;
  if (total <= 0) return { text: UNKNOWN_MARK, known: false };
  return { text: `${toPercent(usage.cacheHitTokens / total)}%`, known: true };
}
