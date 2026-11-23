import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    settingsApi: {
      providers: vi.fn(async () => ({ providers: [], credentials: [] })),
      appConfigGet: vi.fn(async () => ({ agents: {}, categories: [] })),
      skillCatalog: vi.fn(async () => ({ skills: [] })),
      appConfigPatchAgent: vi.fn(),
      appConfigSetCategories: vi.fn(),
      patch: vi.fn(),
      get: vi.fn(),
    },
    vaultApi: { list: vi.fn(), open: vi.fn() },
    docApi: { invalidate: vi.fn(async () => undefined) },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
    },
  };
});

// The leave guard lives in the page shell; section bodies are out of scope here.
vi.mock("./GeneralSection", () => ({ GeneralSection: () => <div data-testid="general-stub" /> }));
vi.mock("./ModelServiceSection", () => ({ ModelServiceSection: () => <div data-testid="model-stub" /> }));
vi.mock("./AgentsSection", () => ({ AgentsSection: () => <div data-testid="agents-stub" /> }));
vi.mock("./CategoriesSection", () => ({ CategoriesSection: () => <div data-testid="categories-stub" /> }));
vi.mock("./AboutSection", () => ({ AboutSection: () => <div data-testid="about-stub" /> }));
vi.mock("./SettingsSaveBar", () => ({ SettingsSaveBar: () => null }));

const { SettingsPage } = await import("./SettingsPage");
const { useAppStore } = await import("../../stores/appStore");
const { useSettingsStore } = await import("../../stores/settingsStore");

beforeEach(() => {
  useAppStore.setState({
    activeVaultId: "v1",
    settingsOpen: true,
    page: "work",
    pendingPage: null,
  });
  useSettingsStore.setState({ config: null, drafts: {}, leaveTarget: null, error: null, notice: null });
});

afterEach(cleanup);

describe("settings page page-switch guard (M8 T01)", () => {
  it("closes settings and opens the skills page when nothing is dirty", async () => {
    render(<SettingsPage />);
    await act(async () => {
      useAppStore.getState().showPage("skills");
    });
    expect(useAppStore.getState().settingsOpen).toBe(false);
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().pendingPage).toBeNull();
  });

  it("discarding the drafts lets the page switch through", async () => {
    render(<SettingsPage />);
    await act(async () => {
      useSettingsStore.getState().setDraft("curator", "改过的 steering");
      useAppStore.getState().showPage("skills");
    });
    expect(screen.getByTestId("settings-leave-dialog")).toBeTruthy();
    expect(useAppStore.getState().settingsOpen).toBe(true);
    expect(useAppStore.getState().page).toBe("work");

    fireEvent.click(screen.getByTestId("settings-leave-discard"));
    expect(useAppStore.getState().settingsOpen).toBe(false);
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().pendingPage).toBeNull();
  });

  it("staying keeps the settings page and the drafts", async () => {
    render(<SettingsPage />);
    await act(async () => {
      useSettingsStore.getState().setDraft("curator", "改过的 steering");
      useAppStore.getState().showPage("skills");
    });
    fireEvent.click(screen.getByTestId("settings-leave-stay"));
    expect(useAppStore.getState().settingsOpen).toBe(true);
    expect(useAppStore.getState().page).toBe("work");
    expect(useAppStore.getState().pendingPage).toBeNull();
    expect(useSettingsStore.getState().drafts["steering:curator"]).toBe("改过的 steering");
  });
});
