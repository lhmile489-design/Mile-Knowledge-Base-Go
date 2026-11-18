// History rail grouping (ported from conversationGroups.ts): pinned first,
// then local calendar days. "Yesterday" is the calendar day, not 24 hours.
import type { ConversationMeta } from "../mygo";

export type HistoryGroupId = "pinned" | "today" | "yesterday" | "earlier";

export interface HistoryGroup {
  readonly id: HistoryGroupId;
  readonly label: string;
  readonly items: readonly ConversationMeta[];
}

const GROUP_LABEL: Record<HistoryGroupId, string> = {
  pinned: "置顶",
  today: "今天",
  yesterday: "昨天",
  earlier: "更早",
};

function startOfDay(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

function dayOf(iso: string): number | null {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return null;
  return startOfDay(date);
}

/** Pinned first, then most recently updated. */
export function sortForDisplay(conversations: readonly ConversationMeta[]): ConversationMeta[] {
  return [...conversations].sort((a, b) => {
    if (a.pinned !== b.pinned) return a.pinned ? -1 : 1;
    return Date.parse(b.updatedAt) - Date.parse(a.updatedAt);
  });
}

export function groupHistory(conversations: readonly ConversationMeta[], now: Date): HistoryGroup[] {
  const sorted = sortForDisplay(conversations);
  const today = startOfDay(now);
  const yesterday = startOfDay(new Date(today - 24 * 60 * 60 * 1000));
  const buckets: Record<HistoryGroupId, ConversationMeta[]> = {
    pinned: [],
    today: [],
    yesterday: [],
    earlier: [],
  };
  for (const meta of sorted) {
    if (meta.pinned) {
      buckets.pinned.push(meta);
      continue;
    }
    const day = dayOf(meta.updatedAt);
    if (day === null) buckets.earlier.push(meta);
    else if (day === today) buckets.today.push(meta);
    else if (day === yesterday) buckets.yesterday.push(meta);
    else buckets.earlier.push(meta);
  }
  const order: HistoryGroupId[] = ["pinned", "today", "yesterday", "earlier"];
  return order
    .filter((id) => buckets[id].length > 0)
    .map((id) => ({ id, label: GROUP_LABEL[id], items: buckets[id] }));
}

/** Row metadata: the last participating agent and a compact time. */
export function rowMeta(meta: ConversationMeta, now: Date): { agent: string | null; time: string } {
  const agent = meta.agents.length > 0 ? (meta.agents[meta.agents.length - 1] ?? null) : null;
  const date = new Date(meta.updatedAt);
  if (Number.isNaN(date.getTime())) return { agent, time: "" };
  const pad = (value: number): string => String(value).padStart(2, "0");
  const day = startOfDay(date);
  const today = startOfDay(now);
  if (day === today || day === startOfDay(new Date(today - 24 * 60 * 60 * 1000))) {
    return { agent, time: `${pad(date.getHours())}:${pad(date.getMinutes())}` };
  }
  return { agent, time: `${pad(date.getMonth() + 1)}-${pad(date.getDate())}` };
}
