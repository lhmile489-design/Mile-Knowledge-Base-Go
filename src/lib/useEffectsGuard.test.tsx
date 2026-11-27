import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import type { AppSettings } from "./transport";

vi.mock("./transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    settingsApi: { patch: vi.fn(), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
    },
  };
});

type FrameCallback = (time: number) => void;

let queue: Map<number, FrameCallback>;
let nextId: number;
let clock: number;

function installFrames(): void {
  queue = new Map();
  nextId = 0;
  clock = 1000;
  vi.stubGlobal("requestAnimationFrame", (callback: FrameCallback) => {
    nextId += 1;
    queue.set(nextId, callback);
    return nextId;
  });
  vi.stubGlobal("cancelAnimationFrame", (id: number) => {
    queue.delete(id);
  });
}

/** Runs the callbacks scheduled so far with a timestamp `gap` ms after the last frame. */
function step(gap: number): void {
  clock += gap;
  const due = [...queue.values()];
  queue.clear();
  for (const callback of due) callback(clock);
}

/** The first frame only records a start time; N gaps need N + 1 frames. */
function frames(gaps: readonly number[]): void {
  step(gaps[0] ?? 5);
  for (const gap of gaps) step(gap);
}

function settingsWith(blur: string): AppSettings {
  return {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent: { preset: "orange" },
    effects: { motion: "std", blur },
    layout: {
      sidebarWidth: 232,
      docWidth: 560,
      docCollapsed: false,
      sidebarCollapsed: false,
      agnesControlsCollapsed: false,
      agnesHistoryOpen: true,
      agnesComposerHeight: 96,
      skillsKbCollapsed: false,
      skillsToolsCollapsed: false,
    },
    recentVaults: [],
  } as AppSettings;
}

function setVisibility(state: "visible" | "hidden"): void {
  Object.defineProperty(document, "visibilityState", { configurable: true, value: state });
  document.dispatchEvent(new Event("visibilitychange"));
}

async function mount(blur: string) {
  vi.resetModules();
  const { useAppStore } = await import("../stores/appStore");
  const { useEffectsGuard } = await import("./useEffectsGuard");
  useAppStore.setState({ settings: settingsWith(blur), effectsDegraded: false });
  const view = renderHook(() => useEffectsGuard());
  return { useAppStore, view };
}

const IDLE = Array.from({ length: 30 }, () => 5);
const scroll = (): void => {
  document.body.dispatchEvent(new Event("scroll"));
};
const jank = (slow: number, slowGap: number, base: number): number[] =>
  Array.from({ length: 120 }, (_, index) => (index < slow ? slowGap : base));

beforeEach(() => {
  installFrames();
  delete document.documentElement.dataset.blurAuto;
  setVisibility("visible");
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("useEffectsGuard", () => {
  it("does not sample at all while the blur level is off", async () => {
    await mount("off");
    expect(queue.size).toBe(0);
    scroll();
    expect(queue.size).toBe(0);
    expect(document.documentElement.dataset.blurAuto).toBeUndefined();
  });

  it("turns the blur off for the run when scrolling drops frames", async () => {
    const { useAppStore } = await mount("std");
    frames(IDLE);
    scroll();
    frames(jank(12, 15, 5.1));
    expect(document.documentElement.dataset.blurAuto).toBe("off");
    expect(useAppStore.getState().effectsDegraded).toBe(true);
  });

  it("keeps the blur and judges only once when scrolling stays smooth", async () => {
    const { useAppStore } = await mount("std");
    frames(IDLE);
    scroll();
    frames(Array.from({ length: 120 }, () => 5.1));
    expect(document.documentElement.dataset.blurAuto).toBeUndefined();
    expect(useAppStore.getState().effectsDegraded).toBe(false);
    // The trial is consumed: a later scroll starts no new sampling.
    scroll();
    expect(queue.size).toBe(0);
  });

  it("is not fooled by a throttled idle sample", async () => {
    const { useAppStore } = await mount("std");
    frames(Array.from({ length: 30 }, () => 25));
    scroll();
    frames(jank(12, 30, 8.3));
    expect(useAppStore.getState().effectsDegraded).toBe(true);
  });

  it("catches uniformly slow frames", async () => {
    const { useAppStore } = await mount("std");
    frames(Array.from({ length: 30 }, () => 16.7));
    scroll();
    frames(Array.from({ length: 120 }, () => 40));
    expect(useAppStore.getState().effectsDegraded).toBe(true);
    expect(document.documentElement.dataset.blurAuto).toBe("off");
  });

  it("waits for the first scroll anywhere in the document", async () => {
    await mount("std");
    frames(IDLE);
    expect(queue.size).toBe(0);
    const inner = document.createElement("div");
    document.body.appendChild(inner);
    inner.dispatchEvent(new Event("scroll"));
    expect(queue.size).toBe(1);
  });

  it("drops a half-finished sample when the page is hidden and starts over when it returns", async () => {
    const { useAppStore } = await mount("std");
    frames(IDLE);
    scroll();
    frames(Array.from({ length: 40 }, () => 100));
    setVisibility("hidden");
    expect(queue.size).toBe(0);
    setVisibility("visible");
    // Back to idle sampling; the earlier slow frames were discarded and the trial is still open.
    expect(queue.size).toBe(1);
    expect(useAppStore.getState().effectsDegraded).toBe(false);
    frames(IDLE);
    scroll();
    frames(Array.from({ length: 120 }, () => 5.1));
    expect(useAppStore.getState().effectsDegraded).toBe(false);
  });

  it("starts judging once the level moves from off to a blur level", async () => {
    const { useAppStore } = await mount("off");
    expect(queue.size).toBe(0);
    act(() => {
      useAppStore.setState({ settings: settingsWith("high") });
    });
    expect(queue.size).toBe(1);
  });

  it("stops sampling when the level goes back to off", async () => {
    const { useAppStore } = await mount("std");
    expect(queue.size).toBe(1);
    act(() => {
      useAppStore.setState({ settings: settingsWith("off") });
    });
    expect(queue.size).toBe(0);
  });

  it("removes its listeners on unmount", async () => {
    const { view } = await mount("std");
    frames(IDLE);
    view.unmount();
    scroll();
    expect(queue.size).toBe(0);
  });
});

describe("useSystemReducedMotion", () => {
  function fakeMedia(initial: boolean) {
    const listeners = new Set<() => void>();
    const media = {
      matches: initial,
      addEventListener: (_type: string, callback: () => void) => listeners.add(callback),
      removeEventListener: (_type: string, callback: () => void) => listeners.delete(callback),
    };
    vi.stubGlobal("matchMedia", () => media);
    return {
      flip(value: boolean) {
        media.matches = value;
        for (const callback of listeners) callback();
      },
      count: () => listeners.size,
    };
  }

  it("mirrors the system preference into the store and follows changes", async () => {
    const media = fakeMedia(true);
    vi.resetModules();
    const { useAppStore } = await import("../stores/appStore");
    const { useSystemReducedMotion } = await import("./useEffectsGuard");
    const view = renderHook(() => useSystemReducedMotion());
    expect(useAppStore.getState().systemReducedMotion).toBe(true);
    act(() => media.flip(false));
    expect(useAppStore.getState().systemReducedMotion).toBe(false);
    act(() => media.flip(true));
    expect(useAppStore.getState().systemReducedMotion).toBe(true);
    view.unmount();
    expect(media.count()).toBe(0);
  });

  it("stays false without matchMedia", async () => {
    vi.stubGlobal("matchMedia", undefined);
    vi.resetModules();
    const { useAppStore } = await import("../stores/appStore");
    const { useSystemReducedMotion } = await import("./useEffectsGuard");
    renderHook(() => useSystemReducedMotion());
    expect(useAppStore.getState().systemReducedMotion).toBe(false);
  });
});
