// Message time labels. A missing or invalid timestamp renders as an empty
// string — never a fabricated "now".
const pad = (value: number): string => String(value).padStart(2, "0");

export function lineTimeLabel(at: string | undefined, now: Date = new Date()): string {
  if (!at) return "";
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return "";
  const time = `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  if (date.toDateString() === now.toDateString()) return time;
  return `${date.getMonth() + 1}/${date.getDate()} ${time}`;
}

/** Conversation title from the first user message (first non-empty line, ≤40). */
export function deriveConversationTitle(firstUserText: string | undefined): string {
  const line = (firstUserText ?? "")
    .split("\n")
    .map((item) => item.replace(/\s+/g, " ").trim())
    .find((item) => item !== "");
  if (!line) return "新对话";
  return line.length > 40 ? `${line.slice(0, 40)}…` : line;
}

/** "M月D日 HH:mm" for the restore note. Invalid timestamps add nothing. */
export function formatMonthDayTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${date.getMonth() + 1}月${date.getDate()}日 ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
