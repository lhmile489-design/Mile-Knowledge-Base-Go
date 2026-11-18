import { create } from "zustand";
import {
  settingsApi,
  vaultApi,
  TransportError,
  events,
  type AppSettings,
} from "../lib/transport";
import type { UnavailableVault, Vault } from "../mygo";

export type ThemeMode = "light" | "dark" | "system";
export type Notice = { id: number; text: string; level: string };

type AppState = {
  booted: boolean;
  settings: AppSettings | null;
  vaults: Vault[];
  unavailable: UnavailableVault[];
  activeVaultId: string;
  notices: Notice[];
  bootError: string | null;

  boot: () => Promise<void>;
  refreshVaults: () => Promise<void>;
  setTheme: (mode: ThemeMode) => Promise<void>;
  setCloseBehavior: (mode: "ask" | "tray" | "quit") => Promise<void>;
  setLayout: (patch: Partial<AppSettings["layout"]>) => Promise<void>;
  openVault: (id: string) => Promise<void>;
  chooseVault: () => Promise<void>;
  forgetVault: (id: string) => Promise<void>;
  retryVault: (id: string) => Promise<void>;
  pushNotice: (text: string, level?: string) => void;
  dismissNotice: (id: number) => void;
};

let noticeSeq = 0;

function asTheme(value: string | undefined): ThemeMode {
  return value === "light" || value === "dark" ? value : "system";
}

export const useAppStore = create<AppState>((set, get) => ({
  booted: false,
  settings: null,
  vaults: [],
  unavailable: [],
  activeVaultId: "",
  notices: [],
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

      events.settingsChanged.on((next) => {
        set({ settings: next });
        applyTheme(asTheme(next.theme));
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

  setCloseBehavior: async (mode) => {
    const result = await settingsApi.patch({ closeBehavior: mode });
    set({ settings: result.settings ?? null });
  },

  setLayout: async (patch) => {
    const current = get().settings;
    if (!current) return;
    const result = await settingsApi.patch({ layout: { ...current.layout, ...patch } });
    set({ settings: result.settings ?? null });
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
