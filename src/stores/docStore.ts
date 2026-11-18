import { useEffect } from "react";
import { create } from "zustand";
import { docApi, TransportError, windowApi } from "../lib/transport";
import { affectedBy } from "../lib/docUtils";
import { useAppStore } from "./appStore";
import { useTreeStore } from "./treeStore";

export type DocState = {
  path: string;
  text: string;
  savedText: string;
  revision: number;
  hash: string;
  size: number;
  eol: string;
  bom: boolean;
  degraded: string;
  loading: boolean;
  error: string | null;
  conflict?: { diskText: string; diskRevision: number; diskHash: string; diskSize: number; baseRevision: number };
  externalRevision?: number;
};

type Tab = { path: string; pinned: boolean };

type DocStore = {
  tabs: Tab[];
  docs: Record<string, DocState>;
  activePath: string;

  open: (vaultId: string, path: string) => Promise<void>;
  close: (vaultId: string, path: string) => Promise<void>;
  setText: (path: string, text: string) => void;
  save: (vaultId: string, path: string) => Promise<void>;
  resolveConflictKeepMine: (vaultId: string, path: string) => Promise<void>;
  resolveConflictTakeDisk: (path: string) => void;
  adoptDisk: (path: string, text: string, revision: number, hash: string, size: number) => void;
  togglePin: (path: string) => void;
  setActive: (path: string) => void;
  reset: () => void;
  applyExternalChange: (vaultId: string, dirs: string[]) => Promise<void>;
  reloadFromDisk: (vaultId: string, path: string) => Promise<void>;
  dismissConflict: (path: string) => void;
};

function pathTitle(path: string): string {
  const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  return i >= 0 ? path.slice(i + 1) : path;
}

export const useDocStore = create<DocStore>((set, get) => ({
  tabs: [],
  docs: {},
  activePath: "",

  open: async (vaultId, path) => {
    const existing = get().docs[path];
    if (existing) {
      set({ activePath: path });
      void docApi.noteOpened(vaultId, path).catch(() => undefined);
      return;
    }
    set((state) => ({
      tabs: state.tabs.some((t) => t.path === path) ? state.tabs : [...state.tabs, { path, pinned: false }],
      activePath: path,
      docs: {
        ...state.docs,
        [path]: {
          path,
          text: "",
          savedText: "",
          revision: 0,
          hash: "",
          size: 0,
          eol: "lf",
          bom: false,
          degraded: "",
          loading: true,
          error: null,
        },
      },
    }));
    try {
      const result = await docApi.read(vaultId, path);
      const doc = result.doc;
      if (!doc) throw new TransportError("not-found", "文件不存在");
      set((state) => ({
        docs: {
          ...state.docs,
          [path]: {
            ...state.docs[path]!,
            text: doc.text,
            savedText: doc.text,
            revision: doc.revision,
            hash: doc.hash,
            size: doc.size,
            eol: doc.eol,
            bom: doc.bom,
            degraded: doc.degraded ?? "",
            loading: false,
            error: null,
          },
        },
      }));
      void docApi.noteOpened(vaultId, path).catch(() => undefined);
    } catch (err) {
      set((state) => ({
        docs: {
          ...state.docs,
          [path]: {
            ...state.docs[path]!,
            loading: false,
            error: err instanceof TransportError ? err.message : String(err),
          },
        },
      }));
    }
  },

  close: async (_vaultId, path) => {
    const { tabs, docs, activePath } = get();
    const index = tabs.findIndex((t) => t.path === path);
    const nextTabs = tabs.filter((t) => t.path !== path);
    const nextDocs = { ...docs };
    delete nextDocs[path];
    let nextActive = activePath;
    if (activePath === path) {
      const neighbour = nextTabs[Math.min(index, nextTabs.length - 1)];
      nextActive = neighbour?.path ?? "";
    }
    set({ tabs: nextTabs, docs: nextDocs, activePath: nextActive });
  },

  setText: (path, text) => {
    set((state) => {
      const doc = state.docs[path];
      if (!doc) return state;
      return { docs: { ...state.docs, [path]: { ...doc, text } } };
    });
  },

  save: async (vaultId, path) => {
    const doc = get().docs[path];
    if (!doc || doc.loading) return;
    try {
      const outcome = await docApi.write(vaultId, path, doc.revision, doc.text);
      if (outcome.status === "conflict") {
        set((state) => ({
          docs: {
            ...state.docs,
            [path]: {
              ...state.docs[path]!,
              conflict: {
                diskText: outcome.diskText ?? "",
                diskRevision: outcome.diskRevision ?? 0,
                diskHash: outcome.diskHash ?? "",
                diskSize: outcome.diskSize ?? 0,
                baseRevision: outcome.baseRevision ?? 0,
              },
            },
          },
        }));
        return;
      }
      set((state) => {
        const current = state.docs[path]!;
        return {
          docs: {
            ...state.docs,
            [path]: {
              ...current,
              text: current.text,
              savedText: current.text,
              revision: outcome.revision ?? current.revision,
              hash: outcome.hash ?? current.hash,
              size: outcome.size ?? current.size,
              conflict: undefined,
              error: null,
            },
          },
        };
      });
      useTreeStore.getState().invalidate([path]);
    } catch (err) {
      set((state) => ({
        docs: {
          ...state.docs,
          [path]: {
            ...state.docs[path]!,
            error: err instanceof TransportError ? err.message : String(err),
          },
        },
      }));
    }
  },

  resolveConflictKeepMine: async (vaultId, path) => {
    const doc = get().docs[path];
    if (!doc?.conflict) return;
    // Overwriting the disk version is an explicit user choice: save against the
    // revision we just observed so the writer accepts it.
    try {
      const outcome = await docApi.write(vaultId, path, doc.conflict.diskRevision, doc.text);
      if (outcome.status === "conflict") {
        set((state) => ({
          docs: {
            ...state.docs,
            [path]: {
              ...state.docs[path]!,
              conflict: {
                diskText: outcome.diskText ?? "",
                diskRevision: outcome.diskRevision ?? 0,
                diskHash: outcome.diskHash ?? "",
                diskSize: outcome.diskSize ?? 0,
                baseRevision: outcome.baseRevision ?? 0,
              },
            },
          },
        }));
        return;
      }
      set((state) => ({
        docs: {
          ...state.docs,
          [path]: {
            ...state.docs[path]!,
            savedText: state.docs[path]!.text,
            revision: outcome.revision ?? 0,
            hash: outcome.hash ?? "",
            size: outcome.size ?? 0,
            conflict: undefined,
          },
        },
      }));
    } catch (err) {
      set((state) => ({
        docs: {
          ...state.docs,
          [path]: { ...state.docs[path]!, error: err instanceof TransportError ? err.message : String(err) },
        },
      }));
    }
  },

  resolveConflictTakeDisk: (path) => {
    const doc = get().docs[path];
    if (!doc?.conflict) return;
    get().adoptDisk(path, doc.conflict.diskText, doc.conflict.diskRevision, doc.conflict.diskHash, doc.conflict.diskSize);
  },

  adoptDisk: (path, text, revision, hash, size) => {
    set((state) => ({
      docs: {
        ...state.docs,
        [path]: {
          ...state.docs[path]!,
          text,
          savedText: text,
          revision,
          hash,
          size,
          conflict: undefined,
          error: null,
        },
      },
    }));
  },

  togglePin: (path) =>
    set((state) => {
      const tabs = state.tabs.map((t) => (t.path === path ? { ...t, pinned: !t.pinned } : t));
      tabs.sort((a, b) => Number(b.pinned) - Number(a.pinned));
      return { tabs };
    }),

  setActive: (path) => set({ activePath: path }),

  reset: () => set({ tabs: [], docs: {}, activePath: "" }),

  dismissConflict: (path) =>
    set((state) => {
      const doc = state.docs[path];
      if (!doc) return state;
      return { docs: { ...state.docs, [path]: { ...doc, conflict: undefined } } };
    }),

  applyExternalChange: async (vaultId, dirs) => {
    const { docs } = get();
    for (const path of Object.keys(docs)) {
      if (!affectedBy(dirs, path)) continue;
      const doc = docs[path];
      if (!doc) continue;
      try {
        const result = await docApi.read(vaultId, path);
        const fresh = result.doc;
        if (!fresh) continue;
        if (fresh.hash === doc.hash) {
          // Same bytes: our own save or an unrelated touch. Nothing to do.
          if (!isDirty(doc) && fresh.revision !== doc.revision) {
            set((state) => ({ docs: { ...state.docs, [path]: { ...state.docs[path]!, revision: fresh.revision } } }));
          }
          continue;
        }
        if (!isDirty(doc)) {
          get().adoptDisk(path, fresh.text, fresh.revision, fresh.hash, fresh.size);
        } else {
          set((state) => ({
            docs: { ...state.docs, [path]: { ...state.docs[path]!, externalRevision: fresh.revision } },
          }));
        }
      } catch {
        // The file may have been deleted: leave the tab as it is; saving will
        // surface a conflict or an error.
      }
    }
  },

  reloadFromDisk: async (vaultId, path) => {
    try {
      const result = await docApi.read(vaultId, path);
      const fresh = result.doc;
      if (!fresh) return;
      get().adoptDisk(path, fresh.text, fresh.revision, fresh.hash, fresh.size);
    } catch {
      // ignore: the card stays and saving keeps working
    }
  },
}));

export function isDirty(doc: DocState | undefined): boolean {
  return !!doc && doc.text !== doc.savedText;
}

export function tabTitle(path: string): string {
  return pathTitle(path);
}

/** Dirty state is mirrored to Go so the close guard can ask before quitting. */
export function useDirtyReporter(): void {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const activePath = useDocStore((state) => state.activePath);
  const doc = useDocStore((state) => (state.activePath ? state.docs[state.activePath] : undefined));
  // The close guard must protect *any* unsaved tab, not just the visible one.
  const dirty = useDocStore((state) => Object.values(state.docs).some((d) => isDirty(d)));
  const revision = doc?.revision ?? 0;
  useEffect(() => {
    void windowApi.reportDocument(vaultId, activePath, revision, dirty).catch(() => undefined);
  }, [vaultId, activePath, revision, dirty]);
}

// E2E scripts drive the real store, never a mock (PLAN-M0 G-E2E).
if (typeof window !== "undefined") {
  (window as unknown as { __mkbDocs?: unknown }).__mkbDocs = useDocStore;
}
