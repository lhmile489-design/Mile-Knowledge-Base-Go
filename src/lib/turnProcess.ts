// One turn's process summary (ported from turnProcess.ts): thoughts and tool
// calls on one timeline, "N steps done · thought x.xs", current step while
// live. Facts only: a missing timestamp yields null instead of 0.0s.
import type { TranscriptLine } from "./transcript";
import { stepState, type StepState, type TranscriptSegment } from "./transcriptGroups";

export interface ProcessStep {
  readonly id: string;
  readonly kind: "thought" | "tool";
  readonly label: string;
  readonly detail: string | null;
  readonly state: StepState;
}

export interface ProcessSummary {
  readonly steps: readonly ProcessStep[];
  readonly doneCount: number;
  readonly thoughtMs: number | null;
  readonly current: string | null;
}

const MAX_STEP_MS = 10 * 60 * 1000;

function isProcessLine(line: TranscriptLine): boolean {
  if (line.role === "thought") return true;
  return line.role === "system" && line.toolCallId !== undefined;
}

export function flattenSegments(segments: readonly TranscriptSegment[]): TranscriptLine[] {
  const lines: TranscriptLine[] = [];
  for (const segment of segments) {
    if (segment.kind === "line") lines.push(segment.line);
    else lines.push(...segment.steps);
  }
  return lines;
}

function findNextTimestamp(lines: readonly TranscriptLine[], from: number): number | null {
  for (let i = from + 1; i < lines.length; i += 1) {
    const line = lines[i];
    if (!line?.at) continue;
    const at = Date.parse(line.at);
    if (!Number.isNaN(at)) return at;
  }
  return null;
}

export function buildProcess(
  segments: readonly TranscriptSegment[],
  live: boolean,
): ProcessSummary | null {
  const lines = flattenSegments(segments);
  const steps: ProcessStep[] = [];
  let thoughtMs = 0;
  let timedThoughts = 0;

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    if (!line || !isProcessLine(line)) continue;
    if (line.role === "thought") {
      steps.push({ id: line.id, kind: "thought", label: "思考", detail: line.text, state: "done" });
      const own = line.at ? Date.parse(line.at) : Number.NaN;
      const next = findNextTimestamp(lines, i);
      if (!Number.isNaN(own) && next !== null && next >= own) {
        const delta = next - own;
        if (delta <= MAX_STEP_MS) {
          thoughtMs += delta;
          timedThoughts += 1;
        }
      }
      continue;
    }
    steps.push({ id: line.id, kind: "tool", label: line.text, detail: null, state: stepState(line) });
  }

  if (steps.length === 0) return null;
  const doneCount = steps.filter((step) => step.state === "done").length;
  const last = steps[steps.length - 1];
  const current = live && last && last.state === "running" ? last.label : null;
  const effectiveDone = current !== null ? Math.min(doneCount, steps.length - 1) : doneCount;
  return {
    steps,
    doneCount: effectiveDone,
    thoughtMs: timedThoughts > 0 ? thoughtMs : null,
    current,
  };
}

export function formatThoughtMs(ms: number): string {
  const seconds = ms / 1000;
  if (seconds < 60) return `${(Math.round(seconds * 10) / 10).toFixed(1)}s`;
  const whole = Math.round(seconds);
  return `${Math.floor(whole / 60)}m ${String(whole % 60).padStart(2, "0")}s`;
}
