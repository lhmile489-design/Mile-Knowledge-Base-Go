import { describe, expect, it } from "vitest";
import type { TranscriptLine } from "./transcript";
import { groupTranscript } from "./transcriptGroups";
import { groupTurns, splitTurn, turnStartedAt, turnStatus } from "./turnGroups";

function line(
  partial: Partial<TranscriptLine> & { role: TranscriptLine["role"]; text: string },
): TranscriptLine {
  return { id: partial.text, ...partial };
}

const lines: TranscriptLine[] = [
  line({ role: "user", text: "问一", at: "2026-11-19T10:00:00Z" }),
  line({ role: "agent", text: "答一", at: "2026-11-19T10:00:05Z" }),
  line({ role: "user", text: "问二", at: "2026-11-19T10:01:00Z" }),
];

describe("turn grouping", () => {
  it("starts a turn at every user line", () => {
    const groups = groupTurns(groupTranscript(lines));
    expect(groups).toHaveLength(2);
    const first = groups[0];
    if (!first || first.kind !== "turn") throw new Error("expected a turn");
    expect(first.segments).toHaveLength(2);
  });

  it("splits the turn into question and the rest", () => {
    const groups = groupTurns(groupTranscript(lines));
    const first = groups[0];
    if (!first || first.kind !== "turn") throw new Error("expected a turn");
    const parts = splitTurn(first.segments);
    expect(parts.user?.text).toBe("问一");
    expect(parts.rest).toHaveLength(1);
  });

  it("turn status follows structured notes, then the live facts", () => {
    const failed = groupTranscript([
      line({ role: "user", text: "问" }),
      line({ role: "system", text: "坏了", note: "error" }),
    ]);
    expect(turnStatus(failed, { isCurrent: true, busy: true, waiting: false, failed: false })).toBe("failed");

    const live = groupTranscript([line({ role: "user", text: "问" })]);
    expect(turnStatus(live, { isCurrent: true, busy: true, waiting: true, failed: false })).toBe("waiting");
    expect(turnStatus(live, { isCurrent: true, busy: true, waiting: false, failed: false })).toBe("running");
    expect(turnStatus(live, { isCurrent: false, busy: true, waiting: true, failed: false })).toBe("done");
  });

  it("turn start comes from the first timestamped line, user time as fallback", () => {
    const groups = groupTurns(groupTranscript(lines));
    const first = groups[0];
    if (!first || first.kind !== "turn") throw new Error("expected a turn");
    const parts = splitTurn(first.segments);
    expect(turnStartedAt(parts.rest, "fallback")).toBe("2026-11-19T10:00:05Z");
    expect(turnStartedAt([], "fallback")).toBe("fallback");
  });
});
