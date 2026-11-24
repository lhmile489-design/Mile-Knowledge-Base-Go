/** Frame-gap maths behind the skills-page glass auto-degrade (DESIGN 13.3 /
 * tech design 19.4). T00 calibrated the thresholds to the display refresh
 * interval R: p95 above 2R plus more than 5% of frames above 2R means the
 * glass costs too much, so the page falls back to opaque cards for this run. */

export function percentile(values: readonly number[], fraction: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.max(0, Math.ceil((fraction / 100) * sorted.length) - 1));
  return sorted[index] ?? 0;
}

export function median(values: readonly number[]): number {
  return percentile(values, 50);
}

export function shouldDegrade(frameGaps: readonly number[], refreshMs: number): boolean {
  if (frameGaps.length === 0 || refreshMs <= 0) return false;
  const limit = refreshMs * 2;
  const slow = frameGaps.filter((gap) => gap > limit).length / frameGaps.length;
  return percentile(frameGaps, 95) > limit && slow > 0.05;
}
