import { describe, expect, it } from "vitest";
import {
  emptyTranscript,
  pushChunk,
  pushLine,
  pushToolCall,
  type TranscriptState,
} from "./transcript";

function ids(): () => string {
  let counter = 0;
  return () => `id-${(counter += 1)}`;
}

describe("transcript coalescing", () => {
  it("merges consecutive chunks of the same role into one line", () => {
    const next = ids();
    let state = emptyTranscript();
    state = pushChunk(state, "agent", "你", next, "2026-11-19T10:00:00Z");
    state = pushChunk(state, "agent", "好", next);
    expect(state.lines).toHaveLength(1);
    expect(state.lines[0]!.text).toBe("你好");
    expect(state.lines[0]!.at).toBe("2026-11-19T10:00:00Z");
  });

  it("opens a new line when the role switches", () => {
    const next = ids();
    let state = emptyTranscript();
    state = pushChunk(state, "thought", "想", next);
    state = pushChunk(state, "agent", "答", next);
    expect(state.lines.map((line) => line.role)).toEqual(["thought", "agent"]);
  });

  it("seals after a tool call so text never glues across the boundary", () => {
    const next = ids();
    let state = emptyTranscript();
    state = pushChunk(state, "agent", "前", next);
    state = sealForTest(state);
    state = pushChunk(state, "agent", "后", next);
    expect(state.lines.map((line) => line.text)).toEqual(["前", "后"]);
  });
});

function sealForTest(state: TranscriptState): TranscriptState {
  return { lines: state.lines, sealed: true };
}

describe("tool lines", () => {
  it("a pending call without a detail produces no line but seals", () => {
    const next = ids();
    let state = pushChunk(emptyTranscript(), "agent", "前", next);
    state = pushToolCall(state, { id: "c1", title: "write_note", status: "pending" }, next);
    expect(state.lines).toHaveLength(1);
    expect(state.sealed).toBe(true);
  });

  it("updates the same line in place as the call progresses", () => {
    const next = ids();
    let state = emptyTranscript();
    state = pushToolCall(
      state,
      { id: "c1", title: "read_note", status: "in-progress", detail: "读取 a.md" },
      next,
      "2026-11-19T10:00:00Z",
    );
    state = pushToolCall(
      state,
      { id: "c1", title: "read_note", status: "completed", detail: "读取 a.md" },
      next,
    );
    expect(state.lines).toHaveLength(1);
    expect(state.lines[0]!.toolStatus).toBe("completed");
    expect(state.lines[0]!.text).toBe("读取 a.md");
  });
});

describe("whole-line appends", () => {
  it("user lines never merge and carry their note", () => {
    const next = ids();
    let state = pushLine(emptyTranscript(), "user", "问", next, "2026-11-19T10:00:00Z");
    state = pushLine(state, "system", "坏了", next, undefined, "error");
    expect(state.lines).toHaveLength(2);
    expect(state.lines[1]!.note).toBe("error");
  });
});
