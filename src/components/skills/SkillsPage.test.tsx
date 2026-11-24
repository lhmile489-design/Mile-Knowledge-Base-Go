import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";

const registry = vi.fn();
const scan = vi.fn();

vi.mock("../../lib/transport", () => {
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

const { SkillsPage } = await import("./SkillsPage");
const { useAppStore } = await import("../../stores/appStore");
const { useSkillsStore } = await import("../../stores/skillsStore");

const scanPayload = {
  cachedAt: "2026-11-24T09:00:00Z",
  scanning: false,
  kb: [
    {
      category: "前端",
      skills: [{ name: "vue", description: "", path: "skills/前端/vue", category: "前端", distributed: [] }],
    },
  ],
  tools: [],
};

beforeEach(() => {
  registry.mockReset().mockResolvedValue({ tools: [] });
  scan.mockReset().mockResolvedValue({ scan: scanPayload });
  useAppStore.setState({ activeVaultId: "v1" });
  useSkillsStore.setState({ tools: [], scan: scanPayload as never, loading: false, error: null });
});

afterEach(cleanup);

describe("skills page shell (M8 T03)", () => {
  it("renders the navigation and focuses its current item on entry", async () => {
    render(<SkillsPage />);
    expect(screen.getByTestId("skills-page")).toBeTruthy();
    const current = screen.getByRole("button", { name: /全部技能/ });
    await waitFor(() => expect(document.activeElement).toBe(current));
    expect(screen.getByTestId("skills-main")).toBeTruthy();
  });

  it("keeps selection in the page and refreshes the scan for the active vault", async () => {
    render(<SkillsPage />);
    await waitFor(() => expect(scan).toHaveBeenCalledWith("v1"));
    const category = screen.getByRole("button", { name: /前端/ });
    category.click();
    await waitFor(() => expect(category.getAttribute("aria-current")).toBe("page"));
    expect(screen.getByRole("button", { name: /全部技能/ }).getAttribute("aria-current")).toBeNull();
  });
});
