import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const registry = vi.fn();
const scan = vi.fn();
const restorePreview = vi.fn();
const checkUpdates = vi.fn();
const readDoc = vi.fn();
const settingsPatch = vi.fn();
const distributePreview = vi.fn();
const distribute = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      registry: (...args: unknown[]) => registry(...args),
      scan: (...args: unknown[]) => scan(...args),
      restorePreview: (...args: unknown[]) => restorePreview(...args),
      checkUpdates: (...args: unknown[]) => checkUpdates(...args),
      listTargets: vi.fn(async () => ({ items: [], dirs: [] })),
      confirmKbUpdate: vi.fn(),
      discardStage: vi.fn(async () => undefined),
      log: vi.fn(async () => ({ records: [] })),
      distributePreview: (...args: unknown[]) => distributePreview(...args),
      distribute: (...args: unknown[]) => distribute(...args),
    },
    renderApi: { document: vi.fn(async () => ({ html: "<p>预览正文</p>" })) },
    windowApi: { openExternal: vi.fn(async () => undefined) },
    docApi: {
      read: (...args: unknown[]) => readDoc(...args),
      noteOpened: vi.fn(async () => undefined),
      write: vi.fn(),
    },
    settingsApi: { patch: (...args: unknown[]) => settingsPatch(...args), get: vi.fn() },
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
  tools: [
    {
      toolId: "claude-code",
      skills: [
        { name: "vue", description: "", path: "~/.claude/skills/vue", toolId: "claude-code", origin: "linked", base: "install", rel: "vue" },
      ],
    },
    {
      toolId: "cursor",
      skills: [
        { name: "go", description: "", path: "~/.cursor/skills/go", toolId: "cursor", origin: "independent", base: "install", rel: "go" },
      ],
    },
  ],
};

let hashSeq = 0;
const emptyPreview = { preview: { status: "ok", items: [], pending: 0, localOnly: [], unmatched: [], warnings: [], canRebuild: true } };

beforeEach(() => {
  registry.mockReset().mockResolvedValue({
    tools: [
      { id: "claude-code", name: "Claude Code", iconKey: "claude-code", installDir: "<home>/.claude/skills", scanDirs: [], status: "detected", kind: "builtin", official: true, enabled: true },
      { id: "cursor", name: "Cursor", iconKey: "cursor", scanDirs: [], status: "detected", kind: "builtin", official: true, enabled: true },
    ],
  });
  scan.mockReset().mockResolvedValue({ scan: scanPayload });
  restorePreview.mockReset().mockResolvedValue(emptyPreview);
  checkUpdates.mockReset().mockResolvedValue({ checkedAt: new Date().toISOString(), checking: false, groups: 0, unchecked: 0 });
  hashSeq += 1;
  const hash = `h${hashSeq}`;
  readDoc.mockReset().mockImplementation(async (_vault: string, path: string) => ({
    doc: { vaultId: "v1", path, text: "", revision: 1, hash: `${hash}-${path}`, size: 0 },
  }));
  distributePreview.mockReset().mockResolvedValue({ items: [] });
  distribute.mockReset().mockResolvedValue({ items: [{ targetId: "claude-code", toolId: "claude-code", result: "linked" }] });
  const baseSettings = {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent: { preset: "orange" },
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
  };
  // The fake backend answers M61 with the layout it was given.
  settingsPatch.mockReset().mockImplementation(async (body: { layout: typeof baseSettings.layout }) => ({
    settings: { ...baseSettings, layout: body.layout },
  }));
  useAppStore.setState({ activeVaultId: "v1", page: "skills", settings: baseSettings as never });
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

  it("shows the tool groups for the selected navigation item", async () => {
    render(<SkillsPage />);
    await waitFor(() => expect(screen.getByRole("button", { name: /Cursor/ })).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(screen.getByTestId("tool-group-cursor")).toBeTruthy();
    expect(screen.getByTestId("tool-skill-go-cursor")).toBeTruthy();
    expect(screen.queryByTestId("tool-skill-vue-claude-code")).toBeNull();
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

  it("opens the distribute dialog over the list for a card", async () => {
    render(<SkillsPage />);
    const card = screen.getByTestId("kb-card-vue");
    fireEvent.click(withinCard("vue").getByRole("button", { name: "分发" }));
    await waitFor(() => expect(screen.getByRole("dialog", { name: "分发「vue」" })).toBeTruthy());
    expect(screen.getByTestId("skills-page").contains(screen.getByTestId("skills-modal"))).toBe(true);
    expect(screen.getByTestId("kb-card-vue")).toBe(card);
    expect(screen.getByTestId("skills-main").hasAttribute("inert")).toBe(true);
  });

  it("keeps the glass on by default and mounts the colour field", () => {
    render(<SkillsPage />);
    expect(screen.getByTestId("skills-page").getAttribute("data-glass")).toBe("on");
    expect(document.querySelector(".skills-field")).toBeTruthy();
  });

  it("offers the manual update check and shows the footer states", async () => {
    render(<SkillsPage />);
    await waitFor(() => expect(checkUpdates).toHaveBeenCalledWith("v1", false));
    checkUpdates.mockResolvedValueOnce({
      checkedAt: new Date().toISOString(),
      checking: false,
      groups: 2,
      unchecked: 2,
      rateLimitResetAt: new Date(Date.now() + 60000).toISOString(),
    });
    fireEvent.click(screen.getByTestId("skills-check-updates"));
    await waitFor(() => expect(checkUpdates).toHaveBeenLastCalledWith("v1", true));
    await waitFor(() => expect(screen.getByTestId("skills-unchecked").textContent).toContain("2 个来源"));
    expect(screen.getByTestId("skills-rate-limit").textContent).toContain("额度用完");
    expect(screen.getByTestId("skills-checked").textContent).toContain("刚刚检查过");
  });

  it("opens the targets manager for a pending targets request and consumes it", async () => {
    useAppStore.setState({ targetsRequest: 1 });
    render(<SkillsPage />);
    await waitFor(() => expect(screen.getByTestId("targets-view")).toBeTruthy());
    expect(useAppStore.getState().targetsRequest).toBe(0);
  });

  it("closes the distribute dialog with Esc and leaves the list as it was", async () => {
    render(<SkillsPage />);
    fireEvent.click(withinCard("vue").getByRole("button", { name: "分发" }));
    await waitFor(() => expect(screen.getByTestId("distribute-dialog")).toBeTruthy());
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByTestId("distribute-dialog")).toBeNull());
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    expect(screen.getByTestId("skills-main").hasAttribute("inert")).toBe(false);
  });

  it("batch distribute: the result stays in the dialog, 完成 refreshes the scan and ends the selection", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 vue" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 go" }));
    fireEvent.click(screen.getByTestId("kb-batch-distribute"));
    await waitFor(() => expect(screen.getByRole("dialog", { name: "分发 2 个技能" })).toBeTruthy());
    // The batch selection stays behind the dialog.
    expect(screen.getByTestId("kb-batchbar")).toBeTruthy();

    fireEvent.click(await screen.findByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    expect(distribute).toHaveBeenCalledTimes(2);
    expect(screen.getByTestId("kb-batchbar")).toBeTruthy();
    // No page banner for the result any more; it lives in the dialog.
    expect(screen.queryByTestId("skills-banner")).toBeNull();

    scan.mockClear();
    fireEvent.click(screen.getByTestId("distribute-done"));
    await waitFor(() => expect(screen.queryByTestId("skills-modal")).toBeNull());
    expect(screen.queryByTestId("kb-batchbar")).toBeNull();
    await waitFor(() => expect(scan).toHaveBeenCalledWith("v1"));
  });

  it("cancelling a batch distribute keeps the selection", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByRole("button", { name: "更多操作：vue" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "批量选择" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 vue" }));
    fireEvent.click(screen.getByTestId("kb-batch-distribute"));
    await waitFor(() => expect(screen.getByTestId("distribute-dialog")).toBeTruthy());
    fireEvent.click(screen.getByTestId("distribute-cancel"));
    await waitFor(() => expect(screen.queryByTestId("skills-modal")).toBeNull());
    expect(screen.getByTestId("kb-batchbar").textContent).toContain("已选 1 个");
  });

  it("opens install as a dialog over the list: the list stays mounted, the page goes inert, focus returns", async () => {
    render(<SkillsPage />);
    const opener = screen.getByTestId("kb-install-top");
    const card = screen.getByTestId("kb-card-vue");
    opener.focus();
    fireEvent.click(opener);
    await waitFor(() => expect(screen.getByRole("dialog", { name: "安装技能" })).toBeTruthy());

    // The modal is a child of the page panel, not of the main area.
    expect(screen.getByTestId("skills-page").contains(screen.getByTestId("skills-modal"))).toBe(true);
    expect(screen.getByTestId("skills-main").contains(screen.getByTestId("skills-modal"))).toBe(false);
    expect(screen.getByTestId("skills-main").hasAttribute("inert")).toBe(true);
    expect(screen.getByTestId("skills-nav").closest("[inert]")).not.toBeNull();
    expect(screen.getByTestId("kb-card-vue")).toBe(card);

    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByTestId("skills-modal")).toBeNull());
    expect(screen.getByTestId("skills-main").hasAttribute("inert")).toBe(false);
    expect(screen.getByTestId("skills-nav").closest("[inert]")).toBeNull();
    expect(screen.getByTestId("kb-card-vue")).toBe(card);
    expect(document.activeElement).toBe(opener);
  });

  it("opens the detail drawer from a card, keeps the list usable, and switches in place", async () => {
    render(<SkillsPage />);
    const goCard = screen.getByTestId("kb-card-go");
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer").getAttribute("data-state")).toBe("ready"));
    const drawer = screen.getByTestId("skill-drawer");
    expect(screen.getByTestId("skills-main").contains(drawer)).toBe(true);
    expect(screen.getByTestId("skill-drawer-path").textContent).toBe("skills/前端/vue/SKILL.md");
    expect(readDoc).toHaveBeenCalledWith("v1", "skills/前端/vue/SKILL.md");
    expect(screen.getByTestId("kb-card-vue").getAttribute("data-active")).toBe("true");
    // The list is still there and not inert: it stays scrollable and clickable.
    expect(screen.getByTestId("kb-card-go")).toBe(goCard);
    expect(goCard.closest("[inert]")).toBeNull();

    fireEvent.click(goCard.querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer-path").textContent).toBe("skills/后端/go/SKILL.md"));
    expect(screen.getByTestId("skill-drawer")).toBe(drawer);
    expect(screen.getByTestId("kb-card-vue").getAttribute("data-active")).toBe("false");
    expect(screen.getByTestId("kb-card-go").getAttribute("data-active")).toBe("true");
  });

  it("closes the drawer with Esc and returns focus to the card that opened it", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer")).toBeTruthy());
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByTestId("skill-drawer")).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(screen.getByTestId("kb-card-vue")));
    expect(screen.getByTestId("kb-card-vue").getAttribute("data-active")).toBe("false");
  });

  it("closes the drawer when another navigation item is chosen", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer")).toBeTruthy());
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(screen.queryByTestId("skill-drawer")).toBeNull();
    expect(screen.getByTestId("tool-group-cursor")).toBeTruthy();
    // Back on the knowledge base the drawer does not come back by itself.
    fireEvent.click(screen.getByRole("button", { name: /全部技能/ }));
    expect(screen.queryByTestId("skill-drawer")).toBeNull();
  });

  it("edits in the workbench from the drawer", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer-edit")).toBeTruthy());
    fireEvent.click(screen.getByTestId("skill-drawer-edit"));
    await waitFor(() => expect(useAppStore.getState().page).toBe("work"));
  });

  it("starts a distribute from the drawer and leaves the drawer open behind the dialog", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer")).toBeTruthy());
    fireEvent.click(screen.getByTestId("skill-drawer-distribute"));
    await waitFor(() => expect(screen.getByRole("dialog", { name: "分发「vue」" })).toBeTruthy());
    expect(screen.getByTestId("skill-drawer")).toBeTruthy();
    expect(screen.getByTestId("skill-drawer").closest("[inert]")).not.toBeNull();
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByTestId("skills-modal")).toBeNull());
    // Esc went to the dialog, not to the drawer.
    expect(screen.getByTestId("skill-drawer")).toBeTruthy();
  });

  it("keeps showing the open skill when a rescan no longer lists it", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer")).toBeTruthy());
    act(() =>
      useSkillsStore.setState({
        scan: { ...scanPayload, kb: scanPayload.kb.filter((group) => group.category !== "前端") } as never,
      }),
    );
    expect(screen.getByTestId("skill-drawer").textContent).toContain("vue");
  });

  it("drops the drawer when the vault changes", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("kb-card-vue").querySelector(".kb-card-name") as HTMLElement);
    await waitFor(() => expect(screen.getByTestId("skill-drawer")).toBeTruthy());
    act(() => useAppStore.setState({ activeVaultId: "v2" }));
    expect(screen.queryByTestId("skill-drawer")).toBeNull();
  });

  it("collapses a nav group through the settings and keeps it collapsed on the next visit", async () => {
    const view = render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    await waitFor(() => expect(settingsPatch).toHaveBeenCalledTimes(1));
    expect((settingsPatch.mock.calls[0]?.[0] as { layout: { skillsKbCollapsed: boolean } }).layout.skillsKbCollapsed).toBe(true);
    await waitFor(() => expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true));
    expect(screen.getByTestId("skills-nav-curmark-kb")).toBeTruthy();
    expect(useAppStore.getState().settings?.layout.skillsKbCollapsed).toBe(true);

    // A later visit (or a restart) reads the saved state.
    view.unmount();
    render(<SkillsPage />);
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    expect(screen.getByTestId("skills-nav-toggle-kb").getAttribute("aria-expanded")).toBe("false");
  });

  it("keeps the collapsed group folded when a tool is chosen from the other group (AC-UX-10)", async () => {
    render(<SkillsPage />);
    fireEvent.click(screen.getByTestId("skills-nav-toggle-kb"));
    await waitFor(() => expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true));
    fireEvent.click(screen.getByRole("button", { name: /Cursor/ }));
    expect(screen.getByTestId("tool-group-cursor")).toBeTruthy();
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    expect(screen.queryByTestId("skills-nav-curmark-kb")).toBeNull();
    expect(settingsPatch).toHaveBeenCalledTimes(1);
  });

  it("enters with the saved collapsed state and writes nothing on the way in", async () => {
    useAppStore.setState({
      settings: {
        ...useAppStore.getState().settings!,
        layout: { ...useAppStore.getState().settings!.layout, skillsKbCollapsed: true },
      },
    });
    render(<SkillsPage />);
    expect(screen.getByTestId("skills-nav-items-kb").hasAttribute("data-collapsed")).toBe(true);
    // A vault switch drops the category and falls back to 全部技能 — still folded, no write.
    expect(settingsPatch).not.toHaveBeenCalled();
  });

  it("filters the navigation without touching the list or the settings", async () => {
    render(<SkillsPage />);
    // The registry (tool display names) arrives after the first paint.
    await screen.findByRole("button", { name: /Cursor/ });
    fireEvent.change(screen.getByRole("textbox", { name: "筛选分类与工具" }), { target: { value: "cursor" } });
    expect(screen.getByRole("button", { name: /Cursor/ })).toBeTruthy();
    expect(screen.queryByRole("button", { name: /前端/ })).toBeNull();
    // The main list is unchanged.
    expect(screen.getByTestId("kb-card-vue")).toBeTruthy();
    expect(screen.getByTestId("kb-card-go")).toBeTruthy();
    expect(settingsPatch).not.toHaveBeenCalled();
  });

  it("keeps page selection stable when the scan refreshes", () => {
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
