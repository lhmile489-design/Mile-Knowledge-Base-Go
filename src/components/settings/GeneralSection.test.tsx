import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { AppSettings } from "../../lib/transport";

const patch = vi.fn();

vi.mock("../../lib/transport", () => {
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

const { GeneralSection } = await import("./GeneralSection");
const { AccentPicker } = await import("../AccentPicker");
const { useAppStore } = await import("../../stores/appStore");

function settingsWith(accent: AppSettings["accent"]): AppSettings {
  return {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent,
    effects: { motion: "std", blur: "std" },
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

const noop = () => {};

beforeEach(() => {
  patch.mockReset();
  patch.mockImplementation(async (body: { accent: AppSettings["accent"] }) => ({
    settings: settingsWith(body.accent),
  }));
  useAppStore.setState({ settings: settingsWith({ preset: "blue" }), notices: [] });
});

afterEach(cleanup);

describe("GeneralSection accent row", () => {
  it("adds a 主题色 row with the six presets and the current one checked", () => {
    render(<GeneralSection onClose={noop} />);
    const group = screen.getByRole("radiogroup", { name: "主题色" });
    expect(within(group).getAllByRole("radio")).toHaveLength(6);
    expect(screen.getByTestId("settings-accent-preset-blue").getAttribute("aria-checked")).toBe("true");
  });

  it("applies immediately and keeps the title-bar dot in sync", async () => {
    render(
      <>
        <AccentPicker />
        <GeneralSection onClose={noop} />
      </>,
    );
    expect(screen.getByTestId("accent-dot").getAttribute("aria-label")).toBe("主题色：蓝");
    fireEvent.click(screen.getByTestId("settings-accent-preset-green"));
    expect(patch).toHaveBeenCalledWith({ accent: { preset: "green" } });
    await waitFor(() => {
      expect(screen.getByTestId("accent-dot").getAttribute("aria-label")).toBe("主题色：绿");
    });
  });

  it("sends a custom color with its cache from the settings row", () => {
    render(<GeneralSection onClose={noop} />);
    const input = screen.getByTestId("settings-accent-hex");
    fireEvent.change(input, { target: { value: "#ffff00" } });
    expect(screen.getByTestId("settings-accent-hint").textContent).toContain("实际使用");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(patch).toHaveBeenCalledTimes(1);
    const sent = (patch.mock.calls.at(0)?.[0] ?? {}) as { accent: { preset: string; cache?: unknown } };
    expect(sent.accent.preset).toBe("custom");
    expect(sent.accent.cache).toBeTruthy();
  });
});
