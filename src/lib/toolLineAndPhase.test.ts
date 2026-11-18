import { describe, expect, it } from "vitest";
import { toolLineText } from "./toolLine";
import { describePhase } from "./agentPhase";
import { deriveConversationTitle, lineTimeLabel } from "./conversationTime";

describe("tool lines", () => {
  it("completed calls show the summary alone", () => {
    expect(toolLineText({ title: "read_note", status: "completed", detail: "读取 a.md" })).toBe("读取 a.md");
  });

  it("failed calls carry the failure without repeating it", () => {
    expect(toolLineText({ title: "read_note", status: "failed", detail: "读取 a.md 失败" })).toBe("读取 a.md 失败");
    expect(toolLineText({ title: "read_note", status: "failed", detail: "读取 a.md" })).toBe("读取 a.md · 失败");
  });

  it("pending without a detail produces no line", () => {
    expect(toolLineText({ title: "write_note", status: "pending" })).toBeNull();
    expect(toolLineText({ title: "write_note", status: "pending", detail: "写入 a.md" })).toBe(
      "写入 a.md · 等待中",
    );
  });
});

describe("agent phase presentation", () => {
  it("waiting for permission never animates", () => {
    const phase = describePhase("waiting_permission");
    expect(phase.animate).toBe(false);
    expect(phase.tone).toBe("waiting");
  });

  it("retrying keeps animating with a concrete label", () => {
    const phase = describePhase("retrying");
    expect(phase.animate).toBe(true);
    expect(phase.label).toContain("重试");
  });

  it("unknown and empty phases stay honest", () => {
    expect(describePhase("implementing_v2").label).toBe("处理中");
    expect(describePhase("").label).toBe("正在思考");
  });
});

describe("conversation time and title", () => {
  it("labels use the message's own timestamp", () => {
    const now = new Date("2026-11-19T12:00:00");
    expect(lineTimeLabel("2026-11-19T09:05:00", now)).toBe("09:05");
    expect(lineTimeLabel(undefined, now)).toBe("");
    expect(lineTimeLabel("not a date", now)).toBe("");
  });

  it("the title comes from the first non-empty line, capped at 40", () => {
    expect(deriveConversationTitle("\n  \n第一行内容\n第二行")).toBe("第一行内容");
    expect(deriveConversationTitle("")).toBe("新对话");
    expect(deriveConversationTitle("字".repeat(50))).toBe(`${"字".repeat(40)}…`);
  });
});
