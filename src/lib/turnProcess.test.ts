import { describe, expect, it } from "vitest";
import type { TranscriptLine } from "./transcript";
import { groupTranscript } from "./transcriptGroups";
import { buildProcess, formatThoughtMs } from "./turnProcess";

function thought(id: string, text: string, at?: string): TranscriptLine {
  return { id, role: "thought", text, ...(at ? { at } : {}) };
}

function tool(id: string, text: string, status: "completed" | "in-progress" | "failed", at?: string): TranscriptLine {
  return { id, role: "system", text, toolCallId: id, toolStatus: status, ...(at ? { at } : {}) };
}

describe("turn process summary", () => {
  it("returns null without thoughts or tools", () => {
    expect(buildProcess(groupTranscript([{ id: "u", role: "user", text: "问" }]), false)).toBeNull();
  });

  it("counts done steps and accumulates thought durations between timestamps", () => {
    const segments = groupTranscript([
      thought("t1", "想法", "2026-11-19T10:00:00Z"),
      tool("c1", "读取 a.md", "completed", "2026-11-19T10:00:02Z"),
      thought("t2", "再想", "2026-11-19T10:00:03Z"),
      tool("c2", "写入 b.md", "completed", "2026-11-19T10:00:04Z"),
    ]);
    const process = buildProcess(segments, false);
    expect(process).not.toBeNull();
    if (!process) return;
    expect(process.steps.map((step) => step.kind)).toEqual(["thought", "tool", "thought", "tool"]);
    expect(process.doneCount).toBe(4);
    expect(process.thoughtMs).toBe(3000);
    expect(process.current).toBeNull();
  });

  it("shows the running step only while live; the in-flight step is not done", () => {
    const segments = groupTranscript([
      tool("c1", "读取 a.md", "completed"),
      tool("c2", "写入 b.md", "in-progress"),
    ]);
    const live = buildProcess(segments, true);
    expect(live?.current).toBe("写入 b.md");
    expect(live?.doneCount).toBe(1);
    const settled = buildProcess(segments, false);
    expect(settled?.current).toBeNull();
  });

  it("keeps thoughtMs null when timestamps are missing instead of claiming 0s", () => {
    const process = buildProcess(groupTranscript([thought("t1", "想法")]), false);
    expect(process?.thoughtMs).toBeNull();
  });

  it("formats thought durations", () => {
    expect(formatThoughtMs(3200)).toBe("3.2s");
    expect(formatThoughtMs(0)).toBe("0.0s");
    expect(formatThoughtMs(65_000)).toBe("1m 05s");
  });
});
