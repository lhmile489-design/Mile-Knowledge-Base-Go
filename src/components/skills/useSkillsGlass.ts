import { useEffect, useState, type RefObject } from "react";
import { median, shouldDegrade } from "../../lib/skillsGlass";

/** Once per app run: after a trial the page keeps whichever mode it picked
 * (tech design 19.4 / Q-PAGE-5). */
let trialConsumed = false;

const IDLE_FRAMES = 30;
const SCROLL_FRAMES = 120;

/** Samples the refresh interval while idle, then the first scroll of the page;
 * switches the glass off for this run when frames blow past the budget. */
export function useSkillsGlass(rootRef: RefObject<HTMLElement | null>): boolean {
  const [glass, setGlass] = useState(true);

  useEffect(() => {
    if (trialConsumed) return;
    const root = rootRef.current;
    if (!root || typeof requestAnimationFrame === "undefined") return;

    let cancelled = false;
    let frame = 0;
    let detach: () => void = () => undefined;

    const idle: number[] = [];
    let last = 0;
    const collectIdle = (time: number) => {
      if (cancelled) return;
      if (last > 0) idle.push(time - last);
      last = time;
      if (idle.length < IDLE_FRAMES) {
        frame = requestAnimationFrame(collectIdle);
        return;
      }
      startScrollWatch();
    };

    const startScrollWatch = () => {
      const refresh = median(idle);
      if (refresh <= 0) return;
      const onScroll = () => {
        detach();
        const gaps: number[] = [];
        let previous = 0;
        const sample = (time: number) => {
          if (cancelled) return;
          if (previous > 0) gaps.push(time - previous);
          previous = time;
          if (gaps.length < SCROLL_FRAMES) {
            frame = requestAnimationFrame(sample);
            return;
          }
          trialConsumed = true;
          if (shouldDegrade(gaps, refresh)) {
            setGlass(false);
            // Frontend log only; no paths (tech design 19.4).
            console.info("[skills-page] glass off: frame p95 over budget");
          }
        };
        frame = requestAnimationFrame(sample);
      };
      root.addEventListener("scroll", onScroll, { capture: true, passive: true });
      detach = () => root.removeEventListener("scroll", onScroll, true);
    };

    frame = requestAnimationFrame(collectIdle);
    return () => {
      cancelled = true;
      cancelAnimationFrame(frame);
      detach();
    };
    // The trial runs once for the page's first mount in this app run.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return glass;
}
