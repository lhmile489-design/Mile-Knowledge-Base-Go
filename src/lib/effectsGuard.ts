/** Frame-gap maths behind the global blur auto-degrade (PRD Q-MAT-2, tech
 * design 21.6). It generalises the skills-page rule from M8: a window that
 * cannot hold the refresh interval while scrolling falls back to opaque
 * panels for the rest of the run.
 *
 * The refresh interval R is the smaller of the idle sample median and the
 * median of the scroll sample itself. A throttled idle sample reads 16.7 or
 * 25 ms on a 120 Hz screen (M9 T07), which made the old R too large and hid
 * real jank; the scroll median cannot be larger than what the user sees.
 * An absolute p95 cap catches a window that is uniformly slow, where the
 * ratio to its own median stays near 1. */

export const IDLE_FRAMES = 30;
export const SCROLL_FRAMES = 120;
/** About 30 fps; p95 above this is degraded no matter the display rate. */
export const ABSOLUTE_P95_MS = 33.4;
/** Share of frames above 2R that makes a high p95 count. */
const SLOW_SHARE = 0.05;

export function percentile(values: readonly number[], fraction: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.min(sorted.length - 1, Math.max(0, Math.ceil((fraction / 100) * sorted.length) - 1));
  return sorted[index] ?? 0;
}

export function median(values: readonly number[]): number {
  return percentile(values, 50);
}

export function shouldDegradeGlobal(scrollGaps: readonly number[], idleMedian: number): boolean {
  if (scrollGaps.length === 0) return false;
  const p95 = percentile(scrollGaps, 95);
  if (p95 > ABSOLUTE_P95_MS) return true;

  const scrollMedian = median(scrollGaps);
  const refresh = idleMedian > 0 ? Math.min(idleMedian, scrollMedian) : scrollMedian;
  if (refresh <= 0) return false;
  const limit = refresh * 2;
  const slow = scrollGaps.filter((gap) => gap > limit).length / scrollGaps.length;
  return p95 > limit && slow > SLOW_SHARE;
}
