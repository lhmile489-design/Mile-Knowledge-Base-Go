import { create } from "zustand";
import {
  settingsApi,
  vaultApi,
  TransportError,
  events,
  type AppSettings,
} from "../lib/transport";
import type { AccentSetting, EffectsSetting, UnavailableVault, Vault } from "../mygo";
import { applyAccentTo, deriveAccent } from "../lib/accent";
import { applyEffectsTo } from "../lib/effects";

export type ThemeMode = "light" | "dark" | "system";
export type Page = "work" | "skills";
export type HealthTab = "unresolvedLinks" | "orphans" | "duplicates" | "untitled";
export type Notice = { id: number; text: string; level: string };

type AppState = {
  booted: boolean;
  settings: AppSettings | null;
  vaults: Vault[];
  unavailable: UnavailableVault[];
  activeVaultId: string;
  notices: Notice[];
  bootError: string | null;
  healthOpen: boolean;
  healthTab: HealthTab;
  auditOpen: boolean;
  settingsOpen: boolean;
  /** Root page (M8 T01): the workbench or the skills page; never persisted. */
  page: Page;
  /** A page request that waits for the settings leave guard (M8 T01). */
  pendingPage: Page | null;
  skillWizardOpen: boolean;
  /** Bumped when another page asks the panel to open the target manager. */
  targetsRequest: number;

  boot: () => Promise<void>;
  refreshVaults: () => Promise<void>;
  setTheme: (mode: ThemeMode) => Promise<void>;
  setAccent: (next: AccentSetting) => Promise<void>;
  setEffects: (patch: Partial<EffectsSetting>) => Promise<void>;
  setCloseBehavior: (mode: "ask" | "tray" | "quit") => Promise<void>;
  setLayout: (patch: Partial<AppSettings["layout"]>) => Promise<void>;
  openVault: (id: string) => Promise<void>;
  chooseVault: () => Promise<void>;
  forgetVault: (id: string) => Promise<void>;
  retryVault: (id: string) => Promise<void>;
  openSettings: () => void;
  closeSettings: () => void;
  showPage: (page: Page) => void;
  commitPendingPage: () => void;
  cancelPendingPage: () => void;
  openTargetsManager: () => void;
  consumeTargetsRequest: () => void;
  openSkillWizard: () => void;
  closeSkillWizard: () => void;
  openHealth: (tab?: HealthTab) => void;
  toggleAudit: () => void;
  closeHealth: () => void;
  pushNotice: (text: string, level?: string) => void;
  dismissNotice: (id: number) => void;
};

let noticeSeq = 0;

function asTheme(value: string | undefined): ThemeMode {
  return value === "light" || value === "dark" ? value : "system";
}

let layoutQueue: Promise<unknown> = Promise.resolve();
let effectsQueue: Promise<unknown> = Promise.resolve();

export const useAppStore = create<AppState>((set, get) => ({
  booted: false,
  settings: null,
  vaults: [],
  unavailable: [],
  activeVaultId: "",
  notices: [],
  healthOpen: false,
  healthTab: "unresolvedLinks",
  auditOpen: false,
  settingsOpen: false,
  page: "work",
  pendingPage: null,
  skillWizardOpen: false,
  targetsRequest: 0,
  bootError: null,

  boot: async () => {
    try {
      await windowApiReady();
      const [settings, vaults] = await Promise.all([settingsApi.get(), vaultApi.list()]);
      set({
        booted: true,
        settings: settings.settings ?? null,
        vaults: vaults.vaults,
        unavailable: vaults.unavailable,
        activeVaultId: vaults.activeVaultId,
      });
      applyTheme(asTheme(settings.settings?.theme));
      applyAccentTo(document.documentElement, settings.settings?.accent);
      applyEffectsTo(document.documentElement, settings.settings?.effects);

      events.settingsChanged.on((next) => {
        set({ settings: next });
        applyTheme(asTheme(next.theme));
        applyAccentTo(document.documentElement, next.accent);
        applyEffectsTo(document.documentElement, next.effects);
      });
      events.vaultChanged.on(() => {
        void get().refreshVaults();
      });
      events.appNotice.on((notice) => get().pushNotice(notice.text, notice.level));
    } catch (error) {
      set({ booted: true, bootError: error instanceof TransportError ? error.message : String(error) });
    }
  },

  refreshVaults: async () => {
    try {
      const list = await vaultApi.list();
      set({ vaults: list.vaults, unavailable: list.unavailable, activeVaultId: list.activeVaultId });
    } catch {
      // Keep the previous view: the watcher may fire while the vault is gone.
    }
  },

  setTheme: async (mode) => {
    const result = await settingsApi.patch({ theme: mode });
    set({ settings: result.settings ?? null });
    applyTheme(mode);
  },

  setAccent: async (next) => {
    // 先在页面上换色，再写设置；失败时回到原色并提示（技术设计 14.2 第 3 条）。
    const previous = get().settings?.accent ?? { preset: "orange" };
    let outgoing = next;
    if (next.preset === "custom" && !next.cache) {
      try {
        outgoing = { ...next, cache: deriveAccent(next.hex ?? "") };
      } catch {
        get().pushNotice("主题色值不合法", "fault");
        return;
      }
    }
    applyAccentTo(document.documentElement, outgoing);
    try {
      const result = await settingsApi.patch({ accent: outgoing });
      if (result.failure || !result.settings) {
        applyAccentTo(document.documentElement, previous);
        get().pushNotice(result.failure?.message ?? "主题色保存失败", "fault");
        return;
      }
      set({ settings: result.settings });
    } catch (error) {
      applyAccentTo(document.documentElement, previous);
      get().pushNotice(error instanceof TransportError ? error.message : String(error), "fault");
    }
  },

  // Effects change all pages at once: paint the new levels first, then persist
  // the full pair (API §17 requires both fields). Patches go out one at a time
  // so two quick clicks merge instead of overwriting each other.
  setEffects: (patch) => {
    const run = async () => {
      const current = get().settings;
      if (!current) return;
      const previous = current.effects;
      const next = { ...previous, ...patch };
      applyEffectsTo(document.documentElement, next);
      const result = await settingsApi.patch({ effects: next });
      if (result.failure || !result.settings) {
        applyEffectsTo(document.documentElement, previous);
        get().pushNotice(result.failure?.message ?? "外观档位保存失败", "fault");
        return;
      }
      set({ settings: result.settings });
    };
    const next = effectsQueue.then(run, run);
    effectsQueue = next.catch(() => undefined);
    return next;
  },

  setCloseBehavior: async (mode) => {
    const result = await settingsApi.patch({ closeBehavior: mode });
    set({ settings: result.settings ?? null });
  },

  // Layout patches go out one at a time: each one merges into the layout the
  // previous one produced, so two quick changes (a collapsed nav group and the
  // next click) cannot overwrite each other.
  setLayout: (patch) => {
    const run = async () => {
      const current = get().settings;
      if (!current) return;
      const result = await settingsApi.patch({ layout: { ...current.layout, ...patch } });
      set({ settings: result.settings ?? null });
    };
    const next = layoutQueue.then(run, run);
    layoutQueue = next.catch(() => undefined);
    return next;
  },

  openVault: async (id) => {
    const list = await vaultApi.open(id);
    set({ vaults: list.vaults, unavailable: list.unavailable, activeVaultId: list.activeVaultId });
  },

  chooseVault: async () => {
    const result = await vaultApi.choose();
    if (result.cancelled) return;
    const list = await vaultApi.list();
    set({ vaults: list.vaults, unavailable: list.unavailable, activeVaultId: list.activeVaultId });
  },

  forgetVault: async (id) => {
    const list = await vaultApi.forget(id);
    set({ vaults: list.vaults, unavailable: list.unavailable, activeVaultId: list.activeVaultId });
  },

  retryVault: async (id) => {
    const list = await vaultApi.retry(id);
    set({ vaults: list.vaults, unavailable: list.unavailable, activeVaultId: list.activeVaultId });
  },

  openSettings: () => set({ settingsOpen: true }),
  closeSettings: () => set({ settingsOpen: false }),
  // Switching to the skills page while settings is open must go through the
  // settings leave guard, so the request parks in pendingPage (M8 T01).
  showPage: (page) => {
    if (page === "skills" && get().settingsOpen) {
      set({ pendingPage: page });
      return;
    }
    set({ page });
  },
  commitPendingPage: () => {
    const pending = get().pendingPage;
    if (!pending) return;
    set({ page: pending, pendingPage: null });
  },
  cancelPendingPage: () => set({ pendingPage: null, targetsRequest: 0 }),
  // M8 T07: the settings entry goes to the skills-page targets view; when the
  // settings page is open the switch still goes through its leave guard.
  openTargetsManager: () => {
    set((state) => ({ targetsRequest: state.targetsRequest + 1 }));
    if (get().settingsOpen) set({ pendingPage: "skills" });
    else set({ page: "skills" });
  },
  consumeTargetsRequest: () => set({ targetsRequest: 0 }),
  openSkillWizard: () => set({ skillWizardOpen: true }),
  closeSkillWizard: () => set({ skillWizardOpen: false }),
  openHealth: (tab) => set({ healthOpen: true, healthTab: tab ?? "unresolvedLinks" }),
  toggleAudit: () => set((state) => ({ auditOpen: !state.auditOpen })),
  closeHealth: () => set({ healthOpen: false }),

  pushNotice: (text, level = "info") => {
    noticeSeq += 1;
    set((state) => ({ notices: [...state.notices, { id: noticeSeq, text, level }] }));
  },

  dismissNotice: (id) => set((state) => ({ notices: state.notices.filter((n) => n.id !== id) })),
}));

async function windowApiReady() {
  const { windowApi } = await import("../lib/transport");
  await windowApi.ready();
}

/** Mirrors the resolved appearance onto <html data-theme> (DESIGN.md §3.4). */
export function applyTheme(mode: ThemeMode) {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const resolved = mode === "system" ? (media.matches ? "dark" : "light") : mode;
  const root = document.documentElement;
  root.classList.add("theme-switching");
  root.dataset.theme = resolved;
  root.style.colorScheme = resolved;
  window.setTimeout(() => root.classList.remove("theme-switching"), 32);
}

// E2E hook: the same store the UI uses.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbApp?: unknown }).__mkbApp = useAppStore;
}
