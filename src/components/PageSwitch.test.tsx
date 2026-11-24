import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const registry = vi.fn();
const scan = vi.fn();

vi.mock("../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      registry: (...args: unknown[]) => registry(...args),
      scan: (...args: unknown[]) => scan(...args),
    },
    settingsApi: { patch: vi.fn(), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { PageSwitch } = await import("./PageSwitch");
const { useAppStore } = await import("../stores/appStore");
const { useSkillsStore } = await import("../stores/skillsStore");

const emptyScan = { cachedAt: "2026-11-24T09:00:00Z", scanning: false, kb: [], tools: [] };

beforeEach(() => {
  registry.mockReset().mockResolvedValue({ tools: [] });
  scan.mockReset().mockResolvedValue({ scan: emptyScan });
  useAppStore.setState({
    page: "work",
    pendingPage: null,
    settingsOpen: false,
    skillsPanelOpen: false,
    activeVaultId: "",
    vaults: [],
    unavailable: [],
  });
  useSkillsStore.setState({ tools: [], scan: null, loading: false, error: null });
});

afterEach(cleanup);

describe("page switch pill (M8 T02)", () => {
  it("marks the current page and moves it when the other segment is clicked", () => {
    render(<PageSwitch />);
    const work = screen.getByRole("button", { name: "工作台" });
    const skills = screen.getByRole("button", { name: "技能" });
    expect(work.getAttribute("aria-current")).toBe("page");
    expect(skills.getAttribute("aria-current")).toBeNull();

    fireEvent.click(skills);
    expect(useAppStore.getState().page).toBe("skills");
    expect(useAppStore.getState().pendingPage).toBeNull();
    expect(screen.getByRole("button", { name: "技能" }).getAttribute("aria-current")).toBe("page");
    expect(screen.getByRole("button", { name: "工作台" }).getAttribute("aria-current")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "工作台" }));
    expect(useAppStore.getState().page).toBe("work");
  });

  it("keeps the attention badge rule and its spoken label", async () => {
    scan.mockResolvedValue({
      scan: {
        ...emptyScan,
        tools: [
          {
            toolId: "claude-code",
            skills: [
              { name: "broken", origin: "link-broken" },
              { name: "stale", origin: "copied", sync: "vault-updated" },
              { name: "fine", origin: "linked" },
            ],
          },
        ],
      },
    });
    render(<PageSwitch />);
    await waitFor(() => expect(screen.getByTestId("skills-badge").textContent).toBe("2"));
    expect(screen.getByTestId("open-skills").getAttribute("aria-label")).toBe("技能，2 项需要处理");
    expect(useSkillsStore.getState().scan?.tools[0]?.skills.length).toBe(3);
  });

  it("keeps native buttons so the pill is keyboard reachable", () => {
    render(<PageSwitch />);
    for (const name of ["工作台", "技能"]) {
      const button = screen.getByRole("button", { name });
      expect(button.tagName).toBe("BUTTON");
      expect(button.getAttribute("type")).toBe("button");
      expect(button.tabIndex).toBe(0);
    }
    expect(screen.getByRole("navigation", { name: "页面" })).toBeTruthy();
  });

  // Transitional until T09 rewrites the e2e entry: with settings open the
  // legacy floating panel still opens above it (AC-SKM-11).
  it("opens the floating panel over settings until the entry migration", () => {
    useAppStore.setState({ settingsOpen: true });
    render(<PageSwitch />);
    fireEvent.click(screen.getByRole("button", { name: "技能" }));
    expect(useAppStore.getState().skillsPanelOpen).toBe(true);
    expect(useAppStore.getState().page).toBe("work");
    expect(useAppStore.getState().pendingPage).toBeNull();
  });
});
