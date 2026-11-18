// Pure reducer from StreamFrames to the chat bucket (T11).
//
// The store only orchestrates: every frame goes through applyFrame, which
// merges deltas into the active line, updates tool lines in place and keeps
// the approval cards until Go confirms their lifecycle. Being pure keeps the
// frame semantics exhaustively testable without the mygo runtime.
import type { PermFrame, StreamFrame, UsageFrame } from "../mygo";
import { emptyTranscript, pushChunk, pushLine, seal, type TranscriptState } from "./transcript";
import { pushToolCall } from "./transcript";
import type { ToolCallStatus } from "./toolLine";

export interface ChatBucket {
  readonly transcript: TranscriptState;
  readonly busy: boolean;
  readonly phase: string;
  readonly phaseDetail: string;
  readonly error: string | null;
  readonly pendingPermissions: readonly PermFrame[];
  readonly usage: UsageFrame | null;
  readonly stopReason: string | null;
}

export function emptyBucket(): ChatBucket {
  return {
    transcript: emptyTranscript(),
    busy: false,
    phase: "",
    phaseDetail: "",
    error: null,
    pendingPermissions: [],
    usage: null,
    stopReason: null,
  };
}

/** A new turn: reset the per-turn facts and record the question first. */
export function startTurn(
  bucket: ChatBucket,
  userText: string,
  at: string,
  nextId: () => string,
): ChatBucket {
  return {
    ...bucket,
    transcript: pushLine(seal(bucket.transcript), "user", userText, nextId, at),
    busy: true,
    phase: "working",
    phaseDetail: "",
    error: null,
    pendingPermissions: [],
    usage: null,
    stopReason: null,
  };
}

function upsertPermission(
  pending: readonly PermFrame[],
  frame: PermFrame,
): readonly PermFrame[] {
  const rest = pending.filter((item) => item.requestId !== frame.requestId);
  return frame.status === "pending" ? [...rest, frame] : rest;
}

function toolStatus(status: string): ToolCallStatus {
  switch (status) {
    case "pending":
    case "in-progress":
    case "completed":
    case "failed":
      return status;
    default:
      return "completed";
  }
}

/** Fold one frame into the bucket. Unknown kinds are ignored. */
export function applyFrame(
  bucket: ChatBucket,
  frame: StreamFrame,
  nextId: () => string,
  at: string,
): ChatBucket {
  switch (frame.kind) {
    case "delta":
      return {
        ...bucket,
        transcript: pushChunk(bucket.transcript, "agent", frame.text ?? "", nextId, at),
      };
    case "thought":
      return {
        ...bucket,
        transcript: pushChunk(bucket.transcript, "thought", frame.text ?? "", nextId, at),
      };
    case "tool": {
      const call = frame.toolCall;
      if (!call) return bucket;
      return {
        ...bucket,
        transcript: pushToolCall(
          bucket.transcript,
          {
            id: call.id,
            title: call.title,
            status: toolStatus(call.status),
            ...(call.detail ? { detail: call.detail } : {}),
          },
          nextId,
          at,
        ),
      };
    }
    case "permission": {
      if (!frame.permission) return bucket;
      return {
        ...bucket,
        transcript: seal(bucket.transcript),
        pendingPermissions: upsertPermission(bucket.pendingPermissions, frame.permission),
        phase: frame.permission.status === "pending" ? "waiting_permission" : bucket.phase,
      };
    }
    case "status":
      return {
        ...bucket,
        phase: frame.status?.state ?? bucket.phase,
        phaseDetail: frame.status?.detail ?? "",
      };
    case "usage":
      return { ...bucket, usage: frame.usage ?? bucket.usage };
    case "turn-end":
      return {
        ...bucket,
        busy: false,
        phase: frame.stopReason === "cancelled" ? "cancelled" : "completed",
        phaseDetail: "",
        stopReason: frame.stopReason ?? null,
        transcript: seal(bucket.transcript),
      };
    case "error":
      // The failure is shown once, as the status card under the transcript;
      // adding a system line as well would duplicate the same sentence.
      return {
        ...bucket,
        busy: false,
        phase: "error",
        phaseDetail: "",
        error: frame.message ?? "这一轮没有完成",
        transcript: seal(bucket.transcript),
      };
    default:
      return bucket;
  }
}

/** Transport-level failure outside the frames (call rejected or channel broke). */
export function applyStreamError(bucket: ChatBucket, message: string, at: string, nextId: () => string): ChatBucket {
  if (bucket.error) return bucket;
  return {
    ...bucket,
    busy: false,
    phase: "error",
    phaseDetail: "",
    error: message,
    transcript: pushLine(seal(bucket.transcript), "system", message, nextId, at, "error"),
  };
}
