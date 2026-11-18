import { describe, expect, it } from "vitest";
import type { PermFrame, StreamFrame } from "../mygo";
import { applyFrame, applyStreamError, emptyBucket, startTurn } from "./chatStream";

function ids(): () => string {
  let counter = 0;
  return () => `id-${(counter += 1)}`;
}

function frame(partial: Partial<StreamFrame> & { kind: string }): StreamFrame {
  return { seq: 1, agent: "general", sessionId: "s1", ...partial } as StreamFrame;
}

function permission(status: string): PermFrame {
  return {
    requestId: "perm_1",
    toolName: "write_note",
    detail: "写入 raws/a.md",
    targetPath: "raws/a.md",
    approval: "ask",
    options: [
      { id: "allow_once", label: "允许" },
      { id: "reject_once", label: "拒绝" },
    ],
    status,
  };
}

const at = "2026-11-19T10:00:00Z";

describe("chat stream reducer", () => {
  it("records the question and marks the turn busy", () => {
    const next = ids();
    const bucket = startTurn(emptyBucket(), "帮我整理", at, next);
    expect(bucket.busy).toBe(true);
    expect(bucket.transcript.lines[0]).toMatchObject({ role: "user", text: "帮我整理", at });
  });

  it("merges answer deltas into one line", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    bucket = applyFrame(bucket, frame({ kind: "delta", text: "你" }), next, at);
    bucket = applyFrame(bucket, frame({ kind: "delta", text: "好" }), next, at);
    const agents = bucket.transcript.lines.filter((line) => line.role === "agent");
    expect(agents).toHaveLength(1);
    expect(agents[0]!.text).toBe("你好");
  });

  it("keeps one tool line per call id", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    bucket = applyFrame(
      bucket,
      frame({ kind: "tool", toolCall: { id: "c1", title: "write_note", status: "pending" } }),
      next,
      at,
    );
    bucket = applyFrame(
      bucket,
      frame({
        kind: "tool",
        toolCall: { id: "c1", title: "write_note", status: "completed", detail: "写入 raws/a.md" },
      }),
      next,
      at,
    );
    const tools = bucket.transcript.lines.filter((line) => line.toolCallId === "c1");
    expect(tools).toHaveLength(1);
    expect(tools[0]!.toolStatus).toBe("completed");
    expect(tools[0]!.text).toBe("写入 raws/a.md");
  });

  it("holds a pending card until Go confirms its lifecycle", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    bucket = applyFrame(bucket, frame({ kind: "permission", permission: permission("pending") }), next, at);
    expect(bucket.pendingPermissions).toHaveLength(1);
    expect(bucket.phase).toBe("waiting_permission");

    bucket = applyFrame(bucket, frame({ kind: "permission", permission: permission("approved") }), next, at);
    expect(bucket.pendingPermissions).toHaveLength(0);
  });

  it("records usage, status and the turn end", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    bucket = applyFrame(
      bucket,
      frame({
        kind: "usage",
        usage: {
          turn: { totalTokens: 10, promptTokens: 6, completionTokens: 4, reasoningTokens: 0, cacheHitTokens: 0, cacheMissTokens: 0 },
          cumulative: { totalTokens: 10, promptTokens: 6, completionTokens: 4, reasoningTokens: 0, cacheHitTokens: 0, cacheMissTokens: 0 },
        },
      }),
      next,
      at,
    );
    expect(bucket.usage?.turn.totalTokens).toBe(10);
    bucket = applyFrame(bucket, frame({ kind: "status", status: { state: "retrying", detail: "第 1/3 次重试" } }), next, at);
    expect(bucket.phase).toBe("retrying");
    expect(bucket.phaseDetail).toBe("第 1/3 次重试");
    bucket = applyFrame(bucket, frame({ kind: "turn-end", stopReason: "end_turn" }), next, at);
    expect(bucket.busy).toBe(false);
    expect(bucket.phase).toBe("completed");
    expect(bucket.stopReason).toBe("end_turn");
  });

  it("an error frame ends the turn without duplicating the message", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    const linesBefore = bucket.transcript.lines.length;
    bucket = applyFrame(bucket, frame({ kind: "error", message: "供应商请求失败" }), next, at);
    expect(bucket.busy).toBe(false);
    expect(bucket.error).toBe("供应商请求失败");
    expect(bucket.transcript.lines).toHaveLength(linesBefore);
  });

  it("a transport failure is recorded once", () => {
    const next = ids();
    let bucket = startTurn(emptyBucket(), "问", at, next);
    bucket = applyStreamError(bucket, "连接中断", at, next);
    const first = bucket;
    bucket = applySimulatedSecondError(first, "再次失败", next);
    expect(bucket.error).toBe("连接中断");
  });
});

function applySimulatedSecondError(
  bucket: ReturnType<typeof emptyBucket>,
  message: string,
  next: () => string,
): ReturnType<typeof emptyBucket> {
  return applyStreamError(bucket, message, at, next);
}
