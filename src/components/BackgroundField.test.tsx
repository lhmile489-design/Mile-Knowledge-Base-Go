import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { AppSettings } from "../lib/transport";

vi.mock("../lib/transport", () => {
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

const { BackgroundField } = await import("./BackgroundField");
const { useAppStore } = await import("../stores/appStore");

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

beforeEach(() => {
  setVisibility("visible");
  useAppStore.setState({ settings: settingsWith("std"), effectsDegraded: false });
});

afterEach(cleanup);

describe("BackgroundField", () => {
  it("draws the three colour blobs behind the app by default", () => {
    render(<BackgroundField />);
    const field = screen.getByTestId("background-field");
    expect(field.getAttribute("aria-hidden")).toBe("true");
    expect(field.querySelectorAll("span")).toHaveLength(3);
  });

  it("is not rendered at blur level off", () => {
    useAppStore.setState({ settings: settingsWith("off") });
    render(<BackgroundField />);
    expect(screen.queryByTestId("background-field")).toBeNull();
  });

  it("is not rendered after the automatic degrade", () => {
    useAppStore.setState({ effectsDegraded: true });
    render(<BackgroundField />);
    expect(screen.queryByTestId("background-field")).toBeNull();
  });

  it("follows the blur level and the degrade flag while mounted", () => {
    render(<BackgroundField />);
    expect(screen.queryByTestId("background-field")).not.toBeNull();
    act(() => useAppStore.setState({ settings: settingsWith("off") }));
    expect(screen.queryByTestId("background-field")).toBeNull();
    act(() => useAppStore.setState({ settings: settingsWith("high") }));
    expect(screen.queryByTestId("background-field")).not.toBeNull();
    act(() => useAppStore.setState({ effectsDegraded: true }));
    expect(screen.queryByTestId("background-field")).toBeNull();
  });

  it("pauses the drift while the page is hidden and resumes when it returns", () => {
    render(<BackgroundField />);
    const root = document.documentElement;
    expect(root.hasAttribute("data-field-paused")).toBe(false);
    act(() => setVisibility("hidden"));
    expect(root.hasAttribute("data-field-paused")).toBe(true);
    act(() => setVisibility("visible"));
    expect(root.hasAttribute("data-field-paused")).toBe(false);
  });

  it("starts paused when mounted on a hidden page and cleans up on unmount", () => {
    setVisibility("hidden");
    const view = render(<BackgroundField />);
    expect(document.documentElement.hasAttribute("data-field-paused")).toBe(true);
    view.unmount();
    expect(document.documentElement.hasAttribute("data-field-paused")).toBe(false);
  });
});
