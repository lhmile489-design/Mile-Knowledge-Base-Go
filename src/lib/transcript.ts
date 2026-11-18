// Streaming transcript coalescing (ported from transcript.ts).
//
// Chunks of the same role merge into the last line; a role switch or any other
// event opens a new line. Tool calls update one line in place per toolCallId,
// and every tool call seals the transcript so text on both sides never glues
// together.
import { toolLineText, type ToolCallStatus, type ToolLineInput } from "./toolLine";

export type TranscriptRole = "user" | "agent" | "thought" | "system";

export interface TranscriptLine {
  readonly id: string;
  readonly role: TranscriptRole;
  readonly text: string;
  readonly toolCallId?: string;
  readonly toolStatus?: ToolCallStatus;
  readonly at?: string;
  readonly note?: "error" | "cancelled";
}

export interface TranscriptState {
  readonly lines: readonly TranscriptLine[];
  readonly sealed: boolean;
}

export function emptyTranscript(): TranscriptState {
  return { lines: [], sealed: false };
}

export function appendChunk(
  transcript: readonly TranscriptLine[],
  role: TranscriptRole,
  chunk: string,
  nextId: () => string,
  at?: string,
): TranscriptLine[] {
  const last = transcript[transcript.length - 1];
  if (last && last.role === role) {
    const merged: TranscriptLine = { ...last, text: last.text + chunk };
    return [...transcript.slice(0, -1), merged];
  }
  return [...transcript, { id: nextId(), role, text: chunk, ...(at ? { at } : {}) }];
}

export function pushChunk(
  state: TranscriptState,
  role: TranscriptRole,
  chunk: string,
  nextId: () => string,
  at?: string,
): TranscriptState {
  if (state.sealed) {
    return {
      lines: [...state.lines, { id: nextId(), role, text: chunk, ...(at ? { at } : {}) }],
      sealed: false,
    };
  }
  return { lines: appendChunk(state.lines, role, chunk, nextId, at), sealed: false };
}

/** Whole-line append (user message, system note); never merges. */
export function pushLine(
  state: TranscriptState,
  role: TranscriptRole,
  text: string,
  nextId: () => string,
  at?: string,
  note?: "error" | "cancelled",
): TranscriptState {
  return {
    lines: [
      ...state.lines,
      { id: nextId(), role, text, ...(at ? { at } : {}), ...(note ? { note } : {}) },
    ],
    sealed: true,
  };
}

export function seal(state: TranscriptState): TranscriptState {
  return state.sealed ? state : { lines: state.lines, sealed: true };
}

/** One tool call updates its line in place; the call seals the transcript. */
export function pushToolCall(
  state: TranscriptState,
  call: ToolLineInput & { readonly id: string },
  nextId: () => string,
  at?: string,
): TranscriptState {
  const text = toolLineText(call);
  if (text === null) return seal(state);
  for (let i = state.lines.length - 1; i >= 0; i -= 1) {
    const line = state.lines[i];
    if (!line || line.toolCallId !== call.id) continue;
    if (line.text === text && line.toolStatus === call.status) return seal(state);
    const lines = state.lines.slice();
    lines[i] = { ...line, text, toolStatus: call.status };
    return { lines, sealed: true };
  }
  return {
    lines: [
      ...state.lines,
      {
        id: nextId(),
        role: "system",
        text,
        toolCallId: call.id,
        toolStatus: call.status,
        ...(at ? { at } : {}),
      },
    ],
    sealed: true,
  };
}
