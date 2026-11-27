import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AppSettings } from "../lib/transport";

const patch = vi.fn();

vi.mock("../lib/transport", () => {
  class TransportError extends Error {
    constructor(
      readonly code: string,
      message: string,
    ) {
      super(message);
    }
  }
  return {
    TransportError,
    settingsApi: { patch: (...args: unknown[]) => patch(...args), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
    },
  };
});

const { useAppStore } = await import("./appStore");

function settingsWith(effects: AppSettings["effects"]): AppSettings {
  return {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent: { preset: "orange" },
    effects,
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

beforeEach(() => {
  patch.mockReset();
  delete document.documentElement.dataset.motion;
  delete document.documentElement.dataset.blur;
  useAppStore.setState({ settings: settingsWith({ motion: "std", blur: "std" }), notices: [] });
});

describe("setEffects", () => {
  it("paints the merged pair first and persists both fields", async () => {
    patch.mockResolvedValue({ settings: settingsWith({ motion: "high", blur: "std" }) });
    await useAppStore.getState().setEffects({ motion: "high" });
    expect(patch).toHaveBeenCalledWith({ effects: { motion: "high", blur: "std" } });
    expect(document.documentElement.dataset.motion).toBe("high");
    expect(document.documentElement.dataset.blur).toBe("std");
    expect(useAppStore.getState().settings?.effects).toEqual({ motion: "high", blur: "std" });
  });

  it("reverts and reports when the patch fails", async () => {
    patch.mockResolvedValue({ failure: { code: "invalid-input", message: "外观档位值不合法" } });
    await useAppStore.getState().setEffects({ blur: "off" });
    expect(document.documentElement.dataset.motion).toBe("std");
    expect(document.documentElement.dataset.blur).toBe("std");
    expect(useAppStore.getState().settings?.effects).toEqual({ motion: "std", blur: "std" });
    expect(useAppStore.getState().notices.at(-1)?.text).toBe("外观档位值不合法");
  });

  it("reverts and reports instead of rejecting when the transport throws", async () => {
    const { TransportError } = await import("../lib/transport");
    patch.mockRejectedValue(new TransportError("unavailable", "连接断开"));
    await expect(useAppStore.getState().setEffects({ motion: "off" })).resolves.toBeUndefined();
    expect(document.documentElement.dataset.motion).toBe("std");
    expect(useAppStore.getState().settings?.effects).toEqual({ motion: "std", blur: "std" });
    expect(useAppStore.getState().notices.at(-1)?.text).toBe("连接断开");
  });

  it("serialises two quick changes so the second merges the first", async () => {
    patch
      .mockResolvedValueOnce({ settings: settingsWith({ motion: "low", blur: "std" }) })
      .mockResolvedValueOnce({ settings: settingsWith({ motion: "low", blur: "high" }) });
    const first = useAppStore.getState().setEffects({ motion: "low" });
    const second = useAppStore.getState().setEffects({ blur: "high" });
    await Promise.all([first, second]);
    expect(patch.mock.calls[0]?.[0]).toEqual({ effects: { motion: "low", blur: "std" } });
    expect(patch.mock.calls[1]?.[0]).toEqual({ effects: { motion: "low", blur: "high" } });
  });
});
