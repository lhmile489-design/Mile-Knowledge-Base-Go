import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

const { AccentPicker } = await import("./AccentPicker");
const { useAppStore } = await import("../stores/appStore");

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

beforeEach(() => {
  patch.mockReset();
  patch.mockImplementation(async (body: { accent: AppSettings["accent"] }) => ({
    settings: settingsWith(body.accent),
  }));
  useAppStore.setState({ settings: settingsWith({ preset: "blue" }), notices: [] });
});

afterEach(cleanup);

describe("AccentPicker", () => {
  it("names the dot and opens a six-preset radiogroup", () => {
    render(<AccentPicker />);
    const dot = screen.getByTestId("accent-dot");
    expect(dot.getAttribute("aria-label")).toBe("主题色：蓝");
    expect(dot.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(dot);
    expect(dot.getAttribute("aria-expanded")).toBe("true");
    const radios = screen.getAllByRole("radio");
    expect(radios).toHaveLength(6);
    expect(screen.getByTestId("accent-preset-blue").getAttribute("aria-checked")).toBe("true");
  });

  it("applies a preset through the store", () => {
    render(<AccentPicker />);
    fireEvent.click(screen.getByTestId("accent-dot"));
    fireEvent.click(screen.getByTestId("accent-preset-green"));
    expect(patch).toHaveBeenCalledWith({ accent: { preset: "green" } });
  });

  it("closes on Escape and returns focus to the dot", () => {
    render(<AccentPicker />);
    const dot = screen.getByTestId("accent-dot");
    fireEvent.click(dot);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByTestId("accent-popover")).toBeNull();
    expect(document.activeElement).toBe(dot);
  });

  it("warns when the custom color is close to gray", () => {
    render(<AccentPicker />);
    fireEvent.click(screen.getByTestId("accent-dot"));
    const input = screen.getByTestId("accent-hex");
    fireEvent.change(input, { target: { value: "#8a8b8c" } });
    expect(screen.getByTestId("accent-hint").textContent).toContain("颜色太接近灰色");
  });

  it("rejects a malformed hex without calling the host", () => {
    render(<AccentPicker />);
    fireEvent.click(screen.getByTestId("accent-dot"));
    const input = screen.getByTestId("accent-hex");
    fireEvent.change(input, { target: { value: "#fff" } });
    expect(screen.getByTestId("accent-hint").textContent).toContain("请输入 #RRGGBB");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(patch).not.toHaveBeenCalled();
  });

  it("shows the adjusted color and applies a custom hex on Enter", () => {
    render(<AccentPicker />);
    fireEvent.click(screen.getByTestId("accent-dot"));
    const input = screen.getByTestId("accent-hex");
    fireEvent.change(input, { target: { value: "#ffff00" } });
    expect(screen.getByTestId("accent-hint").textContent).toContain("实际使用");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(patch).toHaveBeenCalledTimes(1);
    const sent = (patch.mock.calls.at(0)?.[0] ?? {}) as { accent: { preset: string; cache?: unknown } };
    expect(sent.accent.preset).toBe("custom");
    expect(sent.accent.cache).toBeTruthy();
  });
});
