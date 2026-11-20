import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

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
    vaultApi: { list: vi.fn(), choose: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { SkillButton } = await import("./SkillButton");
const { SkillsPanel } = await import("./SkillsPanel");
const { useSkillsStore } = await import("../../stores/skillsStore");
const { useAppStore } = await import("../../stores/appStore");

const emptyScan = { cachedAt: "2026-11-20T09:00:00Z", scanning: false, kb: [], tools: [] };

beforeEach(() => {
  registry.mockReset().mockResolvedValue({ tools: [] });
  scan.mockReset().mockResolvedValue({ scan: emptyScan });
  useSkillsStore.setState({ tools: [], scan: null, loading: false, error: null });
  useAppStore.setState({ skillsPanelOpen: false, activeVaultId: "v1", vaults: [] });
});

afterEach(cleanup);

function renderShell() {
  return render(
    <>
      <SkillButton />
      <SkillsPanel />
    </>,
  );
}

describe("skills panel shell", () => {
  it("opens from the title-bar pill and closes again", async () => {
    renderShell();
    const button = screen.getByTestId("open-skills");
    expect(button.getAttribute("aria-expanded")).toBe("false");
    fireEvent.click(button);
    expect(screen.getByTestId("skills-panel")).toBeTruthy();
    expect(button.getAttribute("aria-expanded")).toBe("true");
    fireEvent.click(button);
    expect(screen.queryByTestId("skills-panel")).toBeNull();
    // The click itself already leaves focus on the pill in a real browser.
  });

  it("closes on Escape and returns focus to the pill", async () => {
    renderShell();
    const button = screen.getByTestId("open-skills");
    fireEvent.click(button);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByTestId("skills-panel")).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(button));
  });

  it("shows the attention badge from the scan", () => {
    useSkillsStore.setState({
      tools: [],
      scan: {
        ...emptyScan,
        tools: [
          {
            toolId: "claude-code",
            skills: [
              { name: "a", description: "", path: "p1", toolId: "claude-code", origin: "copied", sync: "vault-updated" },
              { name: "b", description: "", path: "p2", toolId: "claude-code", origin: "link-broken" },
              { name: "c", description: "", path: "p3", toolId: "claude-code", origin: "linked" },
            ],
          },
        ],
      },
    });
    renderShell();
    expect(screen.getByTestId("skills-badge").textContent).toBe("2");
    expect(screen.getByTestId("open-skills").getAttribute("aria-label")).toBe("技能，2 项需要处理");
  });

  it("lists knowledge-base groups and switches to the tools page", () => {
    registry.mockResolvedValue({
      tools: [
        {
          id: "claude-code",
          name: "Claude Code",
          iconKey: "claude-code",
          scanDirs: [],
          readers: [],
          status: "detected",
          official: true,
          enabled: true,
        },
      ],
    });
    useSkillsStore.setState({
      tools: [
        {
          id: "claude-code",
          name: "Claude Code",
          iconKey: "claude-code",
          scanDirs: [],
          readers: [],
          status: "detected",
          official: true,
          enabled: true,
        },
      ],
      scan: {
        ...emptyScan,
        kb: [
          {
            category: "前端",
            skills: [{ name: "vue", description: "组合式 API", path: "skills/前端/vue", category: "前端" }],
          },
        ],
        tools: [
          {
            toolId: "claude-code",
            skills: [{ name: "vue", description: "", path: "C:\\t\\vue", toolId: "claude-code", origin: "linked" }],
          },
        ],
      },
    });
    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    expect(screen.getByTestId("kb-group-前端")).toBeTruthy();
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    fireEvent.click(screen.getByRole("tab", { name: /工具技能/ }));
    expect(screen.getByTestId("skills-tool-claude-code")).toBeTruthy();
  });

  it("explains the no-vault state on the knowledge-base page", () => {
    useAppStore.setState({ activeVaultId: "" });
    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    expect(screen.getByText("还没有打开知识库。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "打开知识库…" })).toBeTruthy();
  });
});
