import { beforeEach, describe, expect, it, vi } from "vitest";
import type { AppSettings } from "../lib/transport";

const patch = vi.fn();

vi.mock("../lib/transport", () => {
  class TransportError extends Error {}
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

function settingsWith(accent: AppSettings["accent"]): AppSettings {
  return {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent,
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
  document.documentElement.dataset.accent = "orange";
  useAppStore.setState({ settings: settingsWith({ preset: "orange" }), notices: [] });
});

describe("setAccent", () => {
  it("applies the color first and keeps the new settings on success", async () => {
    patch.mockResolvedValue({ settings: settingsWith({ preset: "teal" }) });
    await useAppStore.getState().setAccent({ preset: "teal" });
    expect(patch).toHaveBeenCalledWith({ accent: { preset: "teal" } });
    expect(document.documentElement.dataset.accent).toBe("teal");
    expect(useAppStore.getState().settings?.accent).toEqual({ preset: "teal" });
  });

  it("derives and sends a cache for a custom color", async () => {
    patch.mockImplementation(async (body: { accent: AppSettings["accent"] }) => ({
      settings: settingsWith(body.accent),
    }));
    await useAppStore.getState().setAccent({ preset: "custom", hex: "#ffff00" });
    const sent = (patch.mock.calls.at(0)?.[0] ?? {}) as { accent: { preset: string; cache?: unknown } };
    expect(sent.accent.preset).toBe("custom");
    expect(sent.accent.cache).toBeTruthy();
    expect(document.documentElement.dataset.accent).toBe("custom");
  });

  it("reverts and warns when the host rejects the color", async () => {
    patch.mockResolvedValue({ failure: { code: "invalid-input", message: "主题色值不合法" } });
    await useAppStore.getState().setAccent({ preset: "violet" });
    expect(document.documentElement.dataset.accent).toBe("orange");
    const notices = useAppStore.getState().notices;
    expect(notices.at(-1)?.text).toBe("主题色值不合法");
    expect(notices.at(-1)?.level).toBe("fault");
  });

  it("reverts and warns when the call fails", async () => {
    patch.mockRejectedValue(new Error("socket closed"));
    await useAppStore.getState().setAccent({ preset: "green" });
    expect(document.documentElement.dataset.accent).toBe("orange");
    expect(useAppStore.getState().notices.at(-1)?.level).toBe("fault");
  });

  it("refuses an unusable custom hex without calling the host", async () => {
    await useAppStore.getState().setAccent({ preset: "custom", hex: "nope" });
    expect(patch).not.toHaveBeenCalled();
    expect(document.documentElement.dataset.accent).toBe("orange");
  });
});
