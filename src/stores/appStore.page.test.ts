import { beforeEach, describe, expect, it, vi } from "vitest";

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

const { useAppStore } = await import("./appStore");

beforeEach(() => {
  useAppStore.setState({ page: "work", pendingPage: null, settingsOpen: false });
});

describe("appStore page (M8 T01)", () => {
  it("starts on the workbench and switches directly when settings is closed", () => {
    expect(useAppStore.getState().page).toBe("work");
    useAppStore.getState().showPage("skills");
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().pendingPage).toBeNull();
    useAppStore.getState().showPage("work");
    expect(useAppStore.getState().page).toBe("work");
  });

  it("parks the skills request while settings is open, then commits it", () => {
    useAppStore.setState({ settingsOpen: true });
    useAppStore.getState().showPage("skills");
    expect(useAppStore.getState().page).toBe("work");
    expect(useAppStore.getState().pendingPage).toBe("skills");

    useAppStore.getState().commitPendingPage();
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().pendingPage).toBeNull();
  });

  it("routes the settings targets entry to the skills page and honours the leave guard", () => {
    useAppStore.getState().openTargetsManager();
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().targetsRequest).toBe(1);

    useAppStore.setState({ page: "work", settingsOpen: true, targetsRequest: 0 });
    useAppStore.getState().openTargetsManager();
    expect(useAppStore.getState().page).toBe("work");
    expect(useAppStore.getState().pendingPage).toBe("skills");
    useAppStore.getState().cancelPendingPage();
    expect(useAppStore.getState().pendingPage).toBeNull();
    expect(useAppStore.getState().targetsRequest).toBe(0);
    useAppStore.getState().consumeTargetsRequest();
    expect(useAppStore.getState().targetsRequest).toBe(0);
  });

  it("cancelling the request keeps the workbench", () => {
    useAppStore.setState({ settingsOpen: true });
    useAppStore.getState().showPage("skills");
    useAppStore.getState().cancelPendingPage();
    expect(useAppStore.getState().pendingPage).toBeNull();
    expect(useAppStore.getState().page).toBe("work");
  });
});
