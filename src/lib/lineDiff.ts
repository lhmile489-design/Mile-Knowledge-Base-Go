/**
 * Minimal line diff for the plan-card preview (ported from the legacy
 * shared/lineDiff.ts). A hand-written LCS is enough here: the preview shows
 * which lines are removed and added, and the caller caps the size.
 *
 * Above MAX_LCS_CELLS the result degrades to "everything removed, then
 * everything added" instead of pretending to align.
 */

const MAX_LCS_CELLS = 4_000_000;

export type DiffLineKind = "same" | "added" | "removed";

export interface DiffLine {
  readonly kind: DiffLineKind;
  readonly text: string;
}

export const PREVIEW_MAX_LINES = 60;
export const PREVIEW_MAX_LINE_CHARS = 200;

export function lineDiff(mine: string, disk: string): DiffLine[] {
  if (mine === disk) return mine === "" ? [] : [{ kind: "same", text: mine }];

  const a = mine.split("\n");
  const b = disk.split("\n");
  if (a.length * b.length > MAX_LCS_CELLS) {
    return [
      ...a.map((text) => ({ kind: "removed" as const, text })),
      ...b.map((text) => ({ kind: "added" as const, text })),
    ];
  }

  const rows = a.length + 1;
  const cols = b.length + 1;
  const dp = new Uint32Array(rows * cols);
  for (let i = a.length - 1; i >= 0; i -= 1) {
    for (let j = b.length - 1; j >= 0; j -= 1) {
      dp[i * cols + j] =
        a[i] === b[j]
          ? dp[(i + 1) * cols + j + 1]! + 1
          : Math.max(dp[(i + 1) * cols + j]!, dp[i * cols + j + 1]!);
    }
  }

  const out: DiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      out.push({ kind: "same", text: a[i]! });
      i += 1;
      j += 1;
    } else if (dp[(i + 1) * cols + j]! >= dp[i * cols + j + 1]!) {
      out.push({ kind: "removed", text: a[i]! });
      i += 1;
    } else {
      out.push({ kind: "added", text: b[j]! });
      j += 1;
    }
  }
  while (i < a.length) {
    out.push({ kind: "removed", text: a[i]! });
    i += 1;
  }
  while (j < b.length) {
    out.push({ kind: "added", text: b[j]! });
    j += 1;
  }
  return out;
}

/** Clips a single line and caps the diff; null means "show a summary instead". */
export function previewDiff(mine: string, disk: string): DiffLine[] | null {
  const lines = lineDiff(mine, disk);
  if (lines.length > PREVIEW_MAX_LINES) return null;
  return lines.map((line) => ({
    kind: line.kind,
    text:
      line.text.length > PREVIEW_MAX_LINE_CHARS
        ? line.text.slice(0, PREVIEW_MAX_LINE_CHARS) + "…"
        : line.text,
  }));
}
