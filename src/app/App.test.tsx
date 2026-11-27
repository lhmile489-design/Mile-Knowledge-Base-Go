import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import type { AppSettings } from "../lib/transport";

const vaultsResult: { vaults: unknown[]; unavailable: unknown[]; activeVaultId: string } = {
  vaults: [],
  unavailable: [],
  activeVaultId: "",
};

vi.mock("../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    windowApi: { ready: vi.fn(async () => undefined) },
    settingsApi: {
      get: vi.fn(async () => ({
        settings: {
          version: 1,
          theme: "light",
          closeBehavior: "ask",
          accent: { preset: "orange" },
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
        } as AppSettings,
      })),
    },
    vaultApi: { list: vi.fn(async () => vaultsResult) },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

vi.mock("../components/TitleBar", () => ({ TitleBar: () => <div data-testid="titlebar-stub" /> }));
vi.mock("../components/Workbench", () => ({ Workbench: () => <div data-testid="workbench-stub" /> }));
vi.mock("../components/settings/SettingsPage", () => ({
  SettingsPage: () => <div data-testid="settings-stub" />,
}));
vi.mock("../components/WelcomeView", () => ({ WelcomeView: () => <div data-testid="welcome-stub" /> }));
vi.mock("../components/NoticeStack", () => ({ NoticeStack: () => null }));
vi.mock("../components/SkillImportWizard", () => ({ SkillImportWizard: () => null }));
vi.mock("../components/skills/SkillsPage", () => ({ SkillsPage: () => <div data-testid="skills-stub" /> }));

const { App } = await import("./App");
const { useAppStore } = await import("../stores/appStore");

beforeEach(() => {
  vi.stubGlobal("matchMedia", (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
  vaultsResult.vaults = [{ id: "v1", root: "C:/vaults/one", displayName: "one" }];
  vaultsResult.unavailable = [];
  vaultsResult.activeVaultId = "v1";
  useAppStore.setState({
    booted: false,
    bootError: null,
    settings: null,
    vaults: [],
    unavailable: [],
    activeVaultId: "",
    page: "work",
    pendingPage: null,
    settingsOpen: false,
    notices: [],
  });
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("App shell (M8 T01)", () => {
  it("renders one title bar above a mounted workbench", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    expect(screen.getByTestId("titlebar-stub")).toBeTruthy();
    const host = screen.getByTestId("workbench-host");
    expect(host.hasAttribute("hidden")).toBe(false);
    expect(host.hasAttribute("inert")).toBe(false);
  });

  it("hides and inerts the workbench on the skills page instead of unmounting it", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    act(() => useAppStore.getState().showPage("skills"));
    const host = screen.getByTestId("workbench-host");
    expect(host.hasAttribute("hidden")).toBe(true);
    expect(host.hasAttribute("inert")).toBe(true);
    expect(screen.getByTestId("workbench-stub")).toBeTruthy();
  });

  it("mounts the skills page while the workbench stays hidden (M8 T03)", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    act(() => useAppStore.getState().showPage("skills"));
    expect(screen.getByTestId("skills-stub")).toBeTruthy();
    expect(screen.getByTestId("skills-host")).toBeTruthy();
    expect(screen.getByTestId("workbench-host").hasAttribute("hidden")).toBe(true);
    expect(screen.getByTestId("workbench-host").hasAttribute("inert")).toBe(true);
  });

  it("keeps the settings page as a replacement, not a hidden layer", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    act(() => useAppStore.setState({ settingsOpen: true }));
    expect(screen.getByTestId("settings-stub")).toBeTruthy();
    expect(screen.queryByTestId("workbench-stub")).toBeNull();
  });

  it("shows the welcome page without an active vault", async () => {
    vaultsResult.vaults = [];
    vaultsResult.activeVaultId = "";
    render(<App />);
    await screen.findByTestId("welcome-stub");
    expect(screen.getByTestId("titlebar-stub")).toBeTruthy();
  });
});

describe("Global colour field (M10 T03)", () => {
  it("mounts one field inside the app shell, behind every page", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    const field = screen.getByTestId("background-field");
    expect(field.parentElement?.classList.contains("app-shell")).toBe(true);
    expect(document.querySelectorAll("[data-testid='background-field']")).toHaveLength(1);
  });

  it("keeps the same single field when the skills page takes over", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    act(() => useAppStore.getState().showPage("skills"));
    expect(document.querySelectorAll("[data-testid='background-field']")).toHaveLength(1);
  });

  it("draws no field once the user turns the blur off", async () => {
    render(<App />);
    await screen.findByTestId("workbench-stub");
    act(() => {
      const current = useAppStore.getState().settings;
      if (current) useAppStore.setState({ settings: { ...current, effects: { motion: "std", blur: "off" } } });
    });
    expect(screen.queryByTestId("background-field")).toBeNull();
  });

  it("mirrors the system animation preference into the store", async () => {
    vi.stubGlobal("matchMedia", (query: string) => ({
      matches: query.includes("prefers-reduced-motion"),
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    }));
    render(<App />);
    await screen.findByTestId("workbench-stub");
    expect(useAppStore.getState().systemReducedMotion).toBe(true);
  });
});
