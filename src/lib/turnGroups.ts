// Turn grouping (ported from turnGroups.ts): a user line starts a turn; all
// later segments belong to it until the next user line.
import type { TranscriptLine } from "./transcript";
import type { TranscriptSegment } from "./transcriptGroups";

export type TurnGroup =
  | { readonly kind: "segment"; readonly segment: TranscriptSegment }
  | { readonly kind: "turn"; readonly segments: readonly TranscriptSegment[] };

export function groupTurns(segments: readonly TranscriptSegment[]): TurnGroup[] {
  const groups: TurnGroup[] = [];
  let pending: TranscriptSegment[] = [];
  let inTurn = false;
  const flush = (): void => {
    if (pending.length === 0) return;
    groups.push({ kind: "turn", segments: pending });
    pending = [];
  };
  for (const segment of segments) {
    if (segment.kind === "line" && segment.line.role === "user") {
      flush();
      pending.push(segment);
      inTurn = true;
    } else if (inTurn) {
      pending.push(segment);
    } else {
      groups.push({ kind: "segment", segment });
    }
  }
  flush();
  return groups;
}

export interface TurnParts {
  readonly user: TranscriptLine | null;
  readonly rest: readonly TranscriptSegment[];
}

export function splitTurn(segments: readonly TranscriptSegment[]): TurnParts {
  const first = segments[0];
  if (first && first.kind === "line" && first.line.role === "user") {
    return { user: first.line, rest: segments.slice(1) };
  }
  return { user: null, rest: segments };
}

export type TurnStatus = "running" | "waiting" | "done" | "failed" | "cancelled";

export function turnStatus(
  segments: readonly TranscriptSegment[],
  live: {
    readonly isCurrent: boolean;
    readonly busy: boolean;
    readonly waiting: boolean;
    readonly failed: boolean;
  },
): TurnStatus {
  for (const segment of segments) {
    if (segment.kind !== "line") continue;
    if (segment.line.note === "error") return "failed";
    if (segment.line.note === "cancelled") return "cancelled";
  }
  if (live.isCurrent) {
    if (live.waiting) return "waiting";
    if (live.busy) return "running";
    if (live.failed) return "failed";
  }
  return "done";
}

/** The first timestamped line of a turn; the user's time is the fallback. */
export function turnStartedAt(
  segments: readonly TranscriptSegment[],
  fallback?: string,
): string | undefined {
  for (const segment of segments) {
    if (segment.kind === "line") {
      if (segment.line.at) return segment.line.at;
      continue;
    }
    for (const step of segment.steps) {
      if (step.at) return step.at;
    }
  }
  return fallback;
}
