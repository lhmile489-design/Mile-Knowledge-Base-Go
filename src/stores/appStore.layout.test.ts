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

const base = {
  version: 1,
  theme: "light",
  closeBehavior: "ask",
  accent: { preset: "orange" },
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
} as unknown as AppSettings;

beforeEach(() => {
  patch.mockReset();
  // The fake backend applies the layout it is given, like M61 does.
  patch.mockImplementation(async (body: { layout: AppSettings["layout"] }) => ({
    settings: { ...base, layout: body.layout },
  }));
  useAppStore.setState({ settings: base });
});

describe("setLayout (M9 T05)", () => {
  it("keeps both fields when two patches are fired back to back", async () => {
    const first = useAppStore.getState().setLayout({ skillsKbCollapsed: true });
    const second = useAppStore.getState().setLayout({ skillsToolsCollapsed: true });
    await Promise.all([first, second]);
    const layout = useAppStore.getState().settings?.layout;
    expect(layout?.skillsKbCollapsed).toBe(true);
    expect(layout?.skillsToolsCollapsed).toBe(true);
    expect(patch).toHaveBeenCalledTimes(2);
    // The second request already carries the first change.
    expect((patch.mock.calls[1]?.[0] as { layout: AppSettings["layout"] }).layout.skillsKbCollapsed).toBe(true);
  });

  it("still sends the next patch after one failed", async () => {
    patch.mockRejectedValueOnce(new Error("boom"));
    const failed = useAppStore.getState().setLayout({ skillsKbCollapsed: true });
    const ok = useAppStore.getState().setLayout({ skillsToolsCollapsed: true });
    await expect(failed).rejects.toThrow("boom");
    await ok;
    expect(useAppStore.getState().settings?.layout.skillsToolsCollapsed).toBe(true);
  });
});
