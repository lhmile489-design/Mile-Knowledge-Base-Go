import { useEffect } from "react";
import { useAppStore } from "../stores/appStore";
import { IDLE_FRAMES, SCROLL_FRAMES, median, shouldDegradeGlobal } from "./effectsGuard";

/** One judgement per app run (PRD Q-MAT-2): after it the window keeps whichever
 * mode it picked, and a degrade is never undone until the next launch. */
let trialConsumed = false;
let degraded = false;

function degrade(): void {
  degraded = true;
  document.documentElement.dataset.blurAuto = "off";
  useAppStore.getState().setEffectsDegraded(true);
  // Frontend log only; no paths (tech design 21.6).
  console.info("[effects] blur off: frame p95 over budget");
}

/**
 * Samples the idle refresh interval, then the first scroll anywhere in the
 * document, and switches the blur off for the run when frames blow past the
 * budget. Nothing is sampled while the user's blur level is `off`, and a hidden
 * page throws the half-finished sample away (rAF pauses, so the next gap would
 * span the whole pause).
 */
export function useEffectsGuard(): void {
  const blur = useAppStore((state) => state.settings?.effects.blur);

  useEffect(() => {
    if (blur === undefined || blur === "off") return;
    if (trialConsumed || degraded) return;
    if (typeof requestAnimationFrame === "undefined") return;

    let frame = 0;
    let stopped = false;
    let detachScroll: () => void = () => undefined;

    const cancelWork = (): void => {
      cancelAnimationFrame(frame);
      detachScroll();
      detachScroll = () => undefined;
    };

    const sampleScroll = (idleMedian: number): void => {
      const gaps: number[] = [];
      let previous = 0;
      const tick = (time: number): void => {
        if (stopped) return;
        if (previous > 0) gaps.push(time - previous);
        previous = time;
        if (gaps.length < SCROLL_FRAMES) {
          frame = requestAnimationFrame(tick);
          return;
        }
        trialConsumed = true;
        if (shouldDegradeGlobal(gaps, idleMedian)) degrade();
      };
      frame = requestAnimationFrame(tick);
    };

    const armScroll = (idleMedian: number): void => {
      const onScroll = (): void => {
        detachScroll();
        detachScroll = () => undefined;
        sampleScroll(idleMedian);
      };
      document.addEventListener("scroll", onScroll, { capture: true, passive: true });
      detachScroll = () => document.removeEventListener("scroll", onScroll, true);
    };

    const sampleIdle = (): void => {
      const gaps: number[] = [];
      let previous = 0;
      const tick = (time: number): void => {
        if (stopped) return;
        if (previous > 0) gaps.push(time - previous);
        previous = time;
        if (gaps.length < IDLE_FRAMES) {
          frame = requestAnimationFrame(tick);
          return;
        }
        armScroll(median(gaps));
      };
      frame = requestAnimationFrame(tick);
    };

    const onVisibility = (): void => {
      cancelWork();
      if (document.visibilityState !== "hidden") sampleIdle();
    };

    document.addEventListener("visibilitychange", onVisibility);
    if (document.visibilityState !== "hidden") sampleIdle();

    return () => {
      stopped = true;
      cancelWork();
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [blur]);
}

/** Mirrors the system "animation effects" preference into the store so the
 * settings page can say why the animation level is being overridden (PRD
 * 19.2, tech design 21.7). The user's own level is never changed. */
export function useSystemReducedMotion(): void {
  useEffect(() => {
    if (typeof window.matchMedia !== "function") return;
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = (): void => useAppStore.getState().setSystemReducedMotion(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
}
