// Group transcript lines for rendering (ported from transcriptGroups.ts).
// Consecutive tool steps merge into one pipeline segment; every other line
// stays its own segment.
import type { TranscriptLine } from "./transcript";

export type TranscriptSegment =
  | { readonly kind: "line"; readonly line: TranscriptLine }
  | { readonly kind: "pipeline"; readonly steps: readonly TranscriptLine[] };

function isToolStep(line: TranscriptLine): boolean {
  return line.role === "system" && line.toolCallId !== undefined;
}

export function groupTranscript(lines: readonly TranscriptLine[]): TranscriptSegment[] {
  const segments: TranscriptSegment[] = [];
  let pending: TranscriptLine[] = [];
  const flush = (): void => {
    if (pending.length === 0) return;
    segments.push({ kind: "pipeline", steps: pending });
    pending = [];
  };
  for (const line of lines) {
    if (isToolStep(line)) {
      pending.push(line);
    } else {
      flush();
      segments.push({ kind: "line", line });
    }
  }
  flush();
  return segments;
}

export type StepState = "done" | "running" | "failed" | "neutral";

export function stepState(line: TranscriptLine): StepState {
  switch (line.toolStatus) {
    case "completed":
      return "done";
    case "failed":
      return "failed";
    case "pending":
    case "in-progress":
      return "running";
    default:
      return "neutral";
  }
}
