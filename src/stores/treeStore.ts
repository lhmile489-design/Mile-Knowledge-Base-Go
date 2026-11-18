import { create } from "zustand";
import { docApi, TransportError } from "../lib/transport";
import type { FileEntry } from "../mygo";

type TreeState = {
  vaultId: string;
  nodes: Record<string, FileEntry[]>;
  expanded: Record<string, boolean>;
  loading: Record<string, boolean>;
  error: string | null;
  selected: string;
  skillCategories: number;

  reset: (vaultId: string) => Promise<void>;
  load: (path: string) => Promise<void>;
  toggle: (path: string) => Promise<void>;
  select: (path: string) => void;
  invalidate: (dirs: string[]) => Promise<void>;
};

export const useTreeStore = create<TreeState>((set, get) => ({
  vaultId: "",
  nodes: {},
  expanded: {},
  loading: {},
  error: null,
  selected: "",
  skillCategories: 0,

  reset: async (vaultId) => {
    set({ vaultId, nodes: {}, expanded: {}, loading: {}, error: null, selected: "" });
    await get().load("");
  },

  load: async (path) => {
    const { vaultId, loading } = get();
    if (!vaultId || loading[path]) return;
    set((state) => ({ loading: { ...state.loading, [path]: true }, error: null }));
    try {
      const result = await docApi.tree(vaultId, path);
      set((state) => ({
        nodes: { ...state.nodes, [path]: result.entries },
        loading: { ...state.loading, [path]: false },
        skillCategories: path === "" ? (result.skillCategoryCount ?? 0) : state.skillCategories,
      }));
    } catch (err) {
      set((state) => ({
        loading: { ...state.loading, [path]: false },
        error: err instanceof TransportError ? err.message : String(err),
      }));
    }
  },

  toggle: async (path) => {
    const { expanded, nodes } = get();
    const next = !expanded[path];
    set((state) => ({ expanded: { ...state.expanded, [path]: next } }));
    if (next && !nodes[path]) await get().load(path);
  },

  select: (path) => set({ selected: path }),

  invalidate: async (dirs) => {
    const { nodes, expanded, load } = get();
    const keep = { ...nodes };
    let touchedRoot = false;
    for (const loaded of Object.keys(nodes)) {
      const affected = dirs.some(
        (dir) => dir === loaded || loaded === "" || dir.startsWith(loaded === "" ? "\u0000" : loaded + "/") || loaded.startsWith(dir + "/"),
      );
      if (affected) {
        delete keep[loaded];
        if (loaded === "") touchedRoot = true;
      }
    }
    set({ nodes: keep });
    if (touchedRoot || Object.keys(keep).length === 0) await load("");
    for (const path of Object.keys(expanded)) {
      if (expanded[path] && !keep[path]) await load(path);
    }
  },
}));
