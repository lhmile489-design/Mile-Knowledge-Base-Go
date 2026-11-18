import { describe, expect, it } from "vitest";
import type { ConversationEvent, ConversationIndex, ConversationMeta } from "../mygo";
import { groupHistory, rowMeta } from "./conversationGroups";
import { eventsToLines, linesToTranscript } from "./conversationView";
import { autoRestoreId, restoreTargetAgent } from "./conversationRestore";

const now = new Date("2026-11-19T12:00:00");

function meta(partial: Partial<ConversationMeta>): ConversationMeta {
  return {
    id: "c1",
    title: "标题",
    createdAt: "2026-11-19T08:00:00",
    updatedAt: "2026-11-19T08:00:00",
    turnCount: 1,
    agents: ["curator"],
    pinned: false,
    ...partial,
  };
}

describe("history groups", () => {
  it("splits pinned from calendar days, newest first", () => {
    const groups = groupHistory(
      [
        meta({ id: "today1", updatedAt: "2026-11-19T09:00:00" }),
        meta({ id: "today2", updatedAt: "2026-11-19T10:00:00" }),
        meta({ id: "yesterday", updatedAt: "2026-11-18T23:00:00" }),
        meta({ id: "earlier", updatedAt: "2026-10-01T09:00:00" }),
        meta({ id: "pin", pinned: true, updatedAt: "2026-10-02T09:00:00" }),
      ],
      now,
    );
    expect(groups.map((group) => group.id)).toEqual(["pinned", "today", "yesterday", "earlier"]);
    expect(groups[1]?.items.map((item) => item.id)).toEqual(["today2", "today1"]);
    expect(groups[0]?.items.map((item) => item.id)).toEqual(["pin"]);
  });

  it("an unparsable date falls into earlier instead of crashing", () => {
    const groups = groupHistory([meta({ id: "bad", updatedAt: "nope" })], now);
    expect(groups[0]).toMatchObject({ id: "earlier" });
  });

  it("rows show the last agent and a compact time", () => {
    expect(rowMeta(meta({ agents: ["curator", "scout"], updatedAt: "2026-11-19T09:05:00" }), now)).toEqual({
      agent: "scout",
      time: "09:05",
    });
    expect(rowMeta(meta({ updatedAt: "2026-11-18T22:30:00" }), now).time).toBe("22:30");
    expect(rowMeta(meta({ updatedAt: "2026-10-01T09:05:00" }), now).time).toBe("10-01");
    expect(rowMeta(meta({ agents: [], updatedAt: "bad" }), now)).toEqual({ agent: null, time: "" });
  });
});

describe("conversation view", () => {
  const events: ConversationEvent[] = [
    { t: "user", at: "t1", agent: "curator", text: "问" },
    { t: "thought", at: "t2", agent: "curator", text: "想" },
    { t: "tool", at: "t3", agent: "curator", id: "c1", title: "read_note", status: "completed", detail: "读取 a.md" },
    { t: "message", at: "t4", agent: "curator", text: "答" },
    { t: "turn-end", at: "t5", agent: "curator", stopReason: "cancelled" },
    { t: "error", at: "t6", agent: "curator", message: "断线" },
  ];

  it("maps events to lines and keeps only informative turn ends", () => {
    const lines = eventsToLines(events);
    expect(lines.map((line) => [line.role, line.text])).toEqual([
      ["user", "问"],
      ["thought", "想"],
      ["system", "读取 a.md"],
      ["agent", "答"],
      ["system", "这一轮被取消"],
      ["system", "出错：断线"],
    ]);
    expect(lines[4]?.note).toBe("cancelled");
    expect(lines[5]?.note).toBe("error");
    expect(lines[2]?.toolCallId).toBe("c1");
  });

  it("normal turn ends add no line", () => {
    const lines = eventsToLines([{ t: "turn-end", at: "t", agent: "curator", stopReason: "end_turn" }]);
    expect(lines).toHaveLength(0);
  });

  it("loads lines into a sealed transcript for the viewer", () => {
    const state = linesToTranscript(eventsToLines(events));
    expect(state.sealed).toBe(true);
    expect(state.lines[2]).toMatchObject({ role: "system", toolStatus: "completed" });
  });
});

describe("startup restore", () => {
  const index: ConversationIndex = {
    activeId: "c1",
    conversations: [meta({ id: "c1", turnCount: 2 }), meta({ id: "c2", turnCount: 0 })],
  };

  it("restores only a non-empty active conversation", () => {
    expect(autoRestoreId(index)).toBe("c1");
    expect(autoRestoreId({ ...index, activeId: "c2" })).toBeNull();
    expect(autoRestoreId({ ...index, activeId: null })).toBeNull();
    expect(autoRestoreId(null)).toBeNull();
  });

  it("the target agent is whoever got the last question", () => {
    const events: ConversationEvent[] = [
      { t: "user", at: "t", agent: "curator", text: "一问" },
      { t: "message", at: "t", agent: "scout", text: "答" },
      { t: "user", at: "t", agent: "distiller", text: "二问" },
      { t: "turn-end", at: "t", agent: "scout", stopReason: "end_turn" },
    ];
    expect(restoreTargetAgent(events)).toBe("distiller");
    expect(restoreTargetAgent([{ t: "message", at: "t", agent: "scout", text: "答" }])).toBeNull();
  });
});
