// Persisted events → renderable lines (ported from conversationView.ts). The
// viewer then reuses the live transcript grouping and message components.
import type { ConversationEvent } from "../mygo";
import { toolLineText } from "./toolLine";
import type { TranscriptRole, TranscriptState } from "./transcript";
import type { ToolCallStatus } from "./toolLine";

export interface ViewLine {
  readonly key: string;
  readonly role: TranscriptRole;
  readonly text: string;
  readonly at: string;
  readonly note?: "error" | "cancelled";
  readonly toolCallId?: string;
  readonly toolStatus?: ToolCallStatus;
}

const STOP_LABEL: Record<string, string> = {
  cancelled: "这一轮被取消",
  max_tokens: "这一轮因长度上限被截断",
  max_turn_requests: "这一轮因请求次数上限而停止",
  refusal: "模型拒绝了这一轮",
};

function statusOf(value: string | undefined): ToolCallStatus {
  switch (value) {
    case "pending":
    case "in-progress":
    case "completed":
    case "failed":
      return value;
    default:
      return "completed";
  }
}

/** Normal-ending turns add nothing; cancelled/truncated turns stay visible. */
export function eventsToLines(events: readonly ConversationEvent[]): ViewLine[] {
  const lines: ViewLine[] = [];
  events.forEach((event, index) => {
    const key = `h-${index}`;
    switch (event.t) {
      case "user":
        lines.push({ key, role: "user", text: event.text ?? "", at: event.at });
        return;
      case "message":
        lines.push({ key, role: "agent", text: event.text ?? "", at: event.at });
        return;
      case "thought":
        lines.push({ key, role: "thought", text: event.text ?? "", at: event.at });
        return;
      case "tool": {
        const text = toolLineText({
          title: event.title ?? "",
          status: statusOf(event.status),
          detail: event.detail,
        });
        if (text !== null) {
          lines.push({
            key,
            role: "system",
            text,
            at: event.at,
            toolCallId: event.id ?? key,
            toolStatus: statusOf(event.status),
          });
        }
        return;
      }
      case "error":
        lines.push({ key, role: "system", text: `出错：${event.message ?? ""}`, at: event.at, note: "error" });
        return;
      case "turn-end": {
        const label = STOP_LABEL[event.stopReason ?? ""];
        if (label) {
          lines.push({
            key,
            role: "system",
            text: label,
            at: event.at,
            ...(event.stopReason === "cancelled" ? { note: "cancelled" as const } : {}),
          });
        }
        return;
      }
    }
  });
  return lines;
}

/** Load a historical conversation into a live-looking transcript state. */
export function linesToTranscript(lines: readonly ViewLine[]): TranscriptState {
  return {
    lines: lines.map((line) => ({
      id: line.key,
      role: line.role,
      text: line.text,
      at: line.at,
      ...(line.note ? { note: line.note } : {}),
      ...(line.toolCallId ? { toolCallId: line.toolCallId } : {}),
      ...(line.toolStatus ? { toolStatus: line.toolStatus } : {}),
    })),
    sealed: true,
  };
}
