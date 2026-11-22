import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const registry = vi.fn();
const scan = vi.fn();
const checkUpdates = vi.fn();
const updatePreview = vi.fn();
const discardStage = vi.fn();
const confirmKbUpdate = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      registry: (...args: unknown[]) => registry(...args),
      scan: (...args: unknown[]) => scan(...args),
      checkUpdates: (...args: unknown[]) => checkUpdates(...args),
      updatePreview: (...args: unknown[]) => updatePreview(...args),
      discardStage: (...args: unknown[]) => discardStage(...args),
      confirmKbUpdate: (...args: unknown[]) => confirmKbUpdate(...args),
      restorePreview: vi.fn().mockResolvedValue({ preview: { status: "missing", items: [], pending: 0, localOnly: [], unmatched: [], warnings: [], canRebuild: true } }),
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
const { useMutationStore } = await import("../../stores/mutationStore");

const emptyScan = { cachedAt: "2026-11-20T09:00:00Z", scanning: false, kb: [], tools: [] };

beforeEach(() => {
  registry.mockReset().mockResolvedValue({ tools: [] });
  scan.mockReset().mockResolvedValue({ scan: emptyScan });
  checkUpdates.mockReset().mockResolvedValue({ checkedAt: "", checking: false, groups: 0, unchecked: 0 });
  updatePreview.mockReset();
  discardStage.mockReset().mockResolvedValue({});
  confirmKbUpdate.mockReset().mockResolvedValue({ updated: true });
  useSkillsStore.setState({ tools: [], scan: null, loading: false, error: null });
  useAppStore.setState({ skillsPanelOpen: false, activeVaultId: "v1", vaults: [] });
  useMutationStore.setState({ pending: null, title: "", running: false, report: null, error: null, preview: null });
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
          kind: "builtin",
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
          kind: "builtin",
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
    expect(screen.getByTestId("tool-skill-vue-claude-code")).toBeTruthy();
  });

  it("explains the no-vault state and still loads the tool registry", async () => {
    useAppStore.setState({ activeVaultId: "" });
    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    expect(screen.getByText("还没有打开知识库。")).toBeTruthy();
    expect(screen.getByRole("button", { name: "打开知识库…" })).toBeTruthy();
    await waitFor(() => expect(registry).toHaveBeenCalled());
  });
});

describe("skills panel states", () => {
  it("shows scan skeleton before the first payload arrives", async () => {
    useSkillsStore.setState({ tools: [], scan: null, loading: true, error: null });
    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    expect(screen.getByTestId("skills-skeleton")).toBeTruthy();
    useSkillsStore.setState({ scan: emptyScan });
    await waitFor(() => expect(screen.queryByTestId("skills-skeleton")).toBeNull());
  });
});

describe("skills panel updates", () => {
  it("asks M93 when the panel opens and offers a manual check", async () => {
    checkUpdates.mockResolvedValue({ checkedAt: new Date().toISOString(), checking: false, groups: 2, unchecked: 1 });
    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    await waitFor(() => expect(checkUpdates).toHaveBeenCalledWith("v1", false));
    await waitFor(() => expect(screen.getByTestId("skills-checked").textContent).toContain("刚刚检查过"));
    expect(screen.getByTestId("skills-unchecked").textContent).toContain("1 个来源");
    fireEvent.click(screen.getByTestId("skills-check-updates"));
    await waitFor(() => expect(checkUpdates).toHaveBeenCalledWith("v1", true));
  });

  it("proposes the plan and confirms the update after the report", async () => {
    const commit = "0123456789abcdef0123456789abcdef01234567";
    const latest = "89abcdef0123456789abcdef0123456789abcdef";
    const plan = {
      id: "plan-aaaaaaaaaaaaaaaa",
      vaultId: "v1",
      origin: "user",
      risk: "medium",
      actions: [{ id: "a1", kind: "edit", target: "skills/文档/pdf/SKILL.md", text: "# new", reason: "更新技能" }],
    };
    const customScan = {
      ...emptyScan,
      kb: [
        {
          category: "文档",
          skills: [
            {
              name: "pdf",
              description: "PDF",
              path: "skills/文档/pdf",
              category: "文档",
              distributed: [],
              remote: {
                owner: "anthropics",
                repo: "skills",
                ref: "main",
                path: "skills/pdf",
                commit,
                pathCommit: commit,
                installedAt: "2026-11-21T09:00:00Z",
                updateState: "update-available",
                latestPathCommit: latest,
              },
            },
          ],
        },
      ],
    };
    scan.mockResolvedValue({ scan: customScan });
    useSkillsStore.setState({ tools: [], scan: customScan, loading: false, error: null });
    updatePreview.mockResolvedValue({
      stageId: "s-0123456789ab",
      from: commit,
      to: latest,
      files: [{ path: "SKILL.md", change: "modified", bytes: 10, binary: false }],
      plan,
    });

    renderShell();
    fireEvent.click(screen.getByTestId("open-skills"));
    fireEvent.click(screen.getByTestId("kb-update-pdf"));
    await waitFor(() => expect(screen.getByTestId("update-propose")).toBeTruthy());
    fireEvent.click(screen.getByTestId("update-propose"));
    expect(useMutationStore.getState().pending?.id).toBe(plan.id);
    expect(useMutationStore.getState().title).toContain("pdf");

    useMutationStore.setState({
      pending: null,
      running: false,
      report: { planId: plan.id, status: "completed" } as never,
    });
    await waitFor(() => expect(confirmKbUpdate).toHaveBeenCalledWith("v1", "s-0123456789ab"));
    await waitFor(() => expect(screen.getByTestId("skills-banner").textContent).toContain("已更新"));
  });
});
