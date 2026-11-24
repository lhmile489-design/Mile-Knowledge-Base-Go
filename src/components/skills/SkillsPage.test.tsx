import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const registry = vi.fn();
const scan = vi.fn();
const restorePreview = vi.fn();
const readDoc = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      registry: (...args: unknown[]) => registry(...args),
      scan: (...args: unknown[]) => scan(...args),
      restorePreview: (...args: unknown[]) => restorePreview(...args),
      confirmKbUpdate: vi.fn(),
      discardStage: vi.fn(async () => undefined),
    },
    docApi: {
      read: (...args: unknown[]) => readDoc(...args),
      noteOpened: vi.fn(async () => undefined),
      write: vi.fn(),
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
      skills: [{ name: "vue", description: "组合式 API", path: "skills/前端/vue", category: "前端", distributed: [] }],
    },
    {
      category: "后端",
      skills: [{ name: "go", description: "Go 规范", path: "skills/后端/go", category: "后端", distributed: [] }],
    },
  ],
  tools: [],
};

const emptyPreview = { preview: { status: "ok", items: [], pending: 0, localOnly: [], unmatched: [], warnings: [], canRebuild: true } };

beforeEach(() => {
  registry.mockReset().mockResolvedValue({ tools: [] });
  scan.mockReset().mockResolvedValue({ scan: scanPayload });
  restorePreview.mockReset().mockResolvedValue(emptyPreview);
  readDoc.mockReset().mockResolvedValue({ doc: { text: "", revision: 1, hash: "h", size: 0 } });
  useAppStore.setState({ activeVaultId: "v1", page: "skills" });
  useSkillsStore.setState({ tools: [], scan: scanPayload as never, loading: false, error: null });
});

afterEach(cleanup);

describe("skills page (M8 T03 shell / T04 knowledge-base view)", () => {
  it("renders the navigation and focuses its current item on entry", async () => {
    render(<SkillsPage />);
    expect(screen.getByTestId("skills-page")).toBeTruthy();
    const current = screen.getByRole("button", { name: /全部技能/ });
    await waitFor(() => expect(document.activeElement).toBe(current));
    expect(screen.getByTestId("skills-main")).toBeTruthy();
  });

  it("refreshes the scan and filters the cards by the selected category", async () => {
    render(<SkillsPage />);
    await waitFor(() => expect(scan).toHaveBeenCalledWith("v1"));
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    expect(screen.getByTestId("kb-card-go")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: /后端/ }));
    expect(screen.getByTestId("kb-card-go")).toBeTruthy();
    expect(screen.queryByTestId("kb-card-vue")).toBeNull();
    expect(screen.getByRole("heading", { name: "后端" })).toBeTruthy();
  });

  it("returns to the workbench before opening SKILL.md", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "打开 SKILL.md" }));
    await waitFor(() => expect(useAppStore.getState().page).toBe("work"));
    await waitFor(() => expect(readDoc).toHaveBeenCalledWith("v1", "skills/前端/vue/SKILL.md"));
  });

  it("puts the restore banner at the top of the page content", async () => {
    restorePreview.mockResolvedValue({
      preview: { ...emptyPreview.preview, pending: 2 },
    });
    render(<SkillsPage />);
    await waitFor(() => expect(screen.getByTestId("restore-banner")).toBeTruthy());
    expect(screen.getByTestId("restore-banner").textContent).toContain("2 项尚未在本机还原");
    expect(screen.getByTestId("skills-main").contains(screen.getByTestId("restore-banner"))).toBe(true);
  });

  it("opens the existing distribute dialog for a card", async () => {
    render(<SkillsPage />);
    fireEvent.click(withinCard("vue").getByRole("button", { name: "分发" }));
    await waitFor(() => expect(screen.getByTestId("distribute-dialog")).toBeTruthy());
  });

  it("keeps page selection stable when the scan refreshes", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByRole("button", { name: /前端/ }));
    act(() => useSkillsStore.setState({ scan: { ...scanPayload, cachedAt: "2026-11-24T10:00:00Z" } as never }));
    expect(screen.getByRole("button", { name: /前端/ }).getAttribute("aria-current")).toBe("page");
  });
});

function withinCard(name: string) {
  const card = screen.getByTestId(`kb-card-${name}`);
  return {
    getByRole: (role: string, options?: { name?: string | RegExp }) => {
      const buttons = Array.from(card.querySelectorAll("button"));
      const match = buttons.find((button) => button.textContent?.trim() === options?.name);
      if (!match) throw new Error(`no ${role} ${String(options?.name)} in ${name}`);
      return match as HTMLElement;
    },
  };
}
