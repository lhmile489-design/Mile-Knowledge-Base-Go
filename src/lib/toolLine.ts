// Tool call → one compact conversation line (ported from toolLine.ts). The
// text comes from the detail summary, never from the raw tool name: a pending
// call without a detail produces no line at all.
export type ToolCallStatus = "pending" | "in-progress" | "completed" | "failed";

export interface ToolLineInput {
  readonly title: string;
  readonly status: ToolCallStatus;
  readonly detail?: string;
}

const TOOL_STATUS_LABEL: Record<ToolCallStatus, string> = {
  pending: "等待中",
  "in-progress": "进行中",
  completed: "已完成",
  failed: "失败",
};

const MAX_LINE_CHARS = 160;

function tidy(value: string | undefined): string {
  const flat = (value ?? "").replace(/\s+/g, " ").trim();
  return flat.length <= MAX_LINE_CHARS ? flat : `${flat.slice(0, MAX_LINE_CHARS)}…`;
}

/** The "what is happening now" text for the indicator; empty means nothing. */
export function toolActivityText(detail: string | undefined): string {
  return tidy(detail);
}

/**
 * The line a tool event shows; null means it produces none (pending without a
 * detail). Completed calls show the summary alone; running and failed calls
 * carry their status.
 */
export function toolLineText(call: ToolLineInput): string | null {
  const detail = tidy(call.detail);
  if (detail === "") {
    if (call.status === "pending") return null;
    return `工具：${tidy(call.title)}（${TOOL_STATUS_LABEL[call.status]}）`;
  }
  switch (call.status) {
    case "completed":
      return detail;
    case "failed":
      return detail.includes("失败") ? detail : `${detail} · 失败`;
    default:
      return `${detail} · ${TOOL_STATUS_LABEL[call.status]}`;
  }
}
