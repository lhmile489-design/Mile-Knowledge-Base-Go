import { create } from "zustand";
import { indexApi, type IndexStatus, type SearchHit, type SearchQuery } from "../lib/transport";
import { useAppStore } from "./appStore";

/** Legacy debounce (searchStore.ts: DEBOUNCE_MS). */
const DEBOUNCE_MS = 180;

export type SortMode = "relevance" | "updated" | "path";

type SearchState = {
  open: boolean;
  text: string;
  areas: string[];
  tags: string[];
  extensions: string[];
  updatedAfter: number | null;
  sort: SortMode;
  hits: SearchHit[];
  total: number;
  partial: boolean;
  tookMs: number;
  searching: boolean;
  error: string | null;
  selectedIndex: number;
  status: IndexStatus | null;

  openPanel: () => void;
  closePanel: () => void;
  setText: (text: string) => void;
  toggleArea: (area: string) => void;
  toggleExtension: (extension: string) => void;
  setUpdatedAfter: (ms: number | null) => void;
  clearFilters: () => void;
  setSort: (sort: SortMode) => void;
  moveSelection: (delta: number) => void;
  runSearch: () => Promise<void>;
  refreshStatus: () => Promise<void>;
  applyStatus: (status: IndexStatus) => void;
};

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
/** Older responses must not overwrite newer ones. */
let queryToken = 0;

export const useSearchStore = create<SearchState>((set, get) => ({
  open: false,
  text: "",
  areas: [],
  tags: [],
  extensions: [],
  updatedAfter: null,
  sort: "relevance",
  hits: [],
  total: 0,
  partial: false,
  tookMs: 0,
  searching: false,
  error: null,
  selectedIndex: 0,
  status: null,

  openPanel: () => {
    set({ open: true });
    void get().refreshStatus();
  },

  closePanel: () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    set({ open: false, selectedIndex: 0 });
  },

  setText: (text) => {
    set({ text, selectedIndex: 0 });
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => void get().runSearch(), DEBOUNCE_MS);
  },

  toggleArea: (area) => {
    set((state) => ({
      areas: state.areas.includes(area) ? state.areas.filter((a) => a !== area) : [...state.areas, area],
      selectedIndex: 0,
    }));
    void get().runSearch();
  },

  toggleExtension: (extension) => {
    set((state) => ({
      extensions: state.extensions.includes(extension)
        ? state.extensions.filter((e) => e !== extension)
        : [...state.extensions, extension],
      selectedIndex: 0,
    }));
    void get().runSearch();
  },

  setUpdatedAfter: (ms) => {
    set({ updatedAfter: ms, selectedIndex: 0 });
    void get().runSearch();
  },

  clearFilters: () => {
    set({ areas: [], tags: [], extensions: [], updatedAfter: null, selectedIndex: 0 });
    void get().runSearch();
  },

  setSort: (sort) => {
    set({ sort, selectedIndex: 0 });
    void get().runSearch();
  },

  moveSelection: (delta) => {
    set((state) => {
      const count = state.hits.length;
      if (count === 0) return { selectedIndex: 0 };
      const next = Math.min(count - 1, Math.max(0, state.selectedIndex + delta));
      return { selectedIndex: next };
    });
  },

  runSearch: async () => {
    const vaultId = useAppStore.getState().activeVaultId;
    const { text, areas, tags, extensions, updatedAfter, sort } = get();
    if (!vaultId) {
      set({ hits: [], total: 0, searching: false, error: null });
      return;
    }
    if (text.trim() === "") {
      queryToken += 1;
      set({ hits: [], total: 0, tookMs: 0, partial: false, searching: false, error: null, selectedIndex: 0 });
      return;
    }
    const token = ++queryToken;
    set({ searching: true, error: null });
    const query: SearchQuery = {
      vaultId,
      text,
      sort,
      limit: 50,
      ...(areas.length || tags.length || extensions.length || updatedAfter
        ? {
            filters: {
              ...(areas.length ? { areas } : {}),
              ...(tags.length ? { tags } : {}),
              ...(extensions.length ? { extensions } : {}),
              ...(updatedAfter ? { updatedAfter } : {}),
            },
          }
        : {}),
    };
    try {
      const result = await indexApi.search(query);
      if (token !== queryToken) return; // a newer query already answered
      set({
        hits: result.hits ?? [],
        total: result.total ?? 0,
        partial: Boolean(result.partial),
        tookMs: result.tookMs ?? 0,
        searching: false,
        selectedIndex: 0,
      });
    } catch (error) {
      if (token !== queryToken) return;
      set({ searching: false, error: error instanceof Error ? error.message : String(error) });
    }
  },

  refreshStatus: async () => {
    const vaultId = useAppStore.getState().activeVaultId;
    if (!vaultId) return;
    try {
      set({ status: await indexApi.status(vaultId) });
    } catch {
      // The panel keeps working without a status line.
    }
  },

  applyStatus: (status) => {
    const vaultId = useAppStore.getState().activeVaultId;
    if (!vaultId || status.vaultId !== vaultId) return;
    set({ status });
    // A finished first build means earlier partial results are now complete.
    const { text, partial } = get();
    if (partial && status.phase === "ready" && text.trim() !== "") {
      void get().runSearch();
    }
  },
}));

// E2E hook: the panel's store, same instance the UI uses.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbSearch?: unknown }).__mkbSearch = useSearchStore;
}
