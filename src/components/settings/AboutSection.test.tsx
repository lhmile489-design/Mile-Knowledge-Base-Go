import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const aboutInfo = vi.fn();
const listTargets = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    settingsApi: {
      aboutInfo: () => aboutInfo(),
      copyStoragePath: vi.fn(),
    },
    skillsApi: {
      listTargets: () => listTargets(),
      backups: () => Promise.resolve({ items: [] }),
      cleanBackups: vi.fn(),
    },
    windowApi: {
      openExternal: vi.fn(),
    },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { AboutSection } = await import("./AboutSection");
const { useAppStore } = await import("../../stores/appStore");

afterEach(cleanup);

describe("AboutSection M6 row", () => {
  it("shows the registered target counts and opens the manager", async () => {
    aboutInfo.mockResolvedValue({ version: "1.4.0", techStack: "Go + React", engine: {}, capabilities: {} });
    listTargets.mockResolvedValue({
      items: [
        { id: "t-111111111111", kind: "custom", name: "Droid", path: "<home>/droid", exists: true, skillCount: 1 },
        { id: "t-222222222222", kind: "project", name: "site", path: "D:\\…\\site", exists: true, skillCount: 2 },
        { id: "t-333333333333", kind: "project", name: "site2", path: "D:\\…\\site2", exists: true, skillCount: 0 },
      ],
    });
    useAppStore.setState({ settingsOpen: false, page: "work", targetsRequest: 0 });
    render(<AboutSection onClose={vi.fn()} />);
    await waitFor(() => expect(screen.getByTestId("about-targets").textContent).toContain("自定义 1 个 · 项目 2 个"));
    fireEvent.click(screen.getByTestId("about-manage-targets"));
    expect(useAppStore.getState().targetsRequest).toBe(1);
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().skillsPanelOpen).toBe(false);
  });
});
