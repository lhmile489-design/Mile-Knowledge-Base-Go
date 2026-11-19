// History rail, read-only viewer and startup restore (T15). All persistence
// goes through ConversationService (M50–M59); this store only orchestrates.
import { create } from "zustand";
import type { ConversationIndex, ConversationMeta, ConversationSearchHit } from "../mygo";
import { agentApi } from "../lib/agentApi";
import { autoRestoreId, restoreTargetAgent } from "../lib/conversationRestore";
import { eventsToLines, linesToTranscript, type ViewLine } from "../lib/conversationView";
import { useAgentStore } from "./agentStore";
import { useAppStore } from "./appStore";

const SEARCH_DEBOUNCE_MS = 250;

export type RestoreNote =
  | { readonly kind: "restored"; readonly updatedAt: string }
  | { readonly kind: "failed" }
  | null;

interface ViewerState {
  readonly id: string;
  readonly meta: ConversationMeta;
  readonly lines: readonly ViewLine[];
}

interface HistoryState {
  open: boolean;
  index: ConversationIndex | null;
  searchText: string;
  hits: ConversationSearchHit[];
  searching: boolean;
  viewer: ViewerState | null;
  restoreNote: RestoreNote;

  toggleRail(): void;
  refresh(vaultId: string | null): Promise<void>;
  setSearch(vaultId: string | null, text: string): void;
  hydrateOpen(open: boolean | undefined): void;
  openViewer(vaultId: string | null, id: string): Promise<void>;
  closeViewer(): void;
  continueConversation(vaultId: string | null): Promise<void>;
  rename(vaultId: string, id: string, title: string): Promise<void>;
  togglePin(vaultId: string, id: string, pinned: boolean): Promise<void>;
  remove(vaultId: string, id: string): Promise<void>;
  clearAll(vaultId: string): Promise<void>;
  exportConversation(vaultId: string, id: string): Promise<boolean>;
  restoreOnStart(vaultId: string | null): Promise<void>;
  startNewConversation(vaultId: string | null): Promise<void>;
}

let searchTimer: ReturnType<typeof setTimeout> | null = null;
const restoredFor = new Set<string>();
// The rail's open state comes from the saved layout once; a later toggle must
// not be overwritten when the layout object is refreshed.
let openHydrated = false;

function conversationTitle(index: ConversationIndex | null, id: string): ConversationMeta | null {
  return index?.conversations.find((item) => item.id === id) ?? null;
}

/** Load one conversation into the live chat as the active continuation. */
async function loadIntoChat(vaultId: string, id: string): Promise<ConversationMeta> {
  const read = await agentApi.conversations.activate(vaultId, id);
  if (!read.meta) throw new Error("对话不存在");
  const events = read.events ?? [];
  const lines = eventsToLines(events);
  const target = restoreTargetAgent(events) ?? "general";
  useAgentStore.getState().setAgent(target);
  useAgentStore.setState((state) => ({
    buckets: {
      ...state.buckets,
      [target]: {
        ...state.buckets[target],
        transcript: linesToTranscript(lines),
        busy: false,
        phase: "completed",
        phaseDetail: "",
        error: null,
        pendingPermissions: [],
        usage: null,
        stopReason: null,
      },
    },
  }));
  return read.meta;
}

export const useHistoryStore = create<HistoryState>()((set, get) => ({
  open: false,
  index: null,
  searchText: "",
  hits: [],
  searching: false,
  viewer: null,
  restoreNote: null,

  toggleRail: () => {
    const next = !get().open;
    set({ open: next });
    void useAppStore.getState().setLayout({ agnesHistoryOpen: next });
  },

  hydrateOpen: (open) => {
    if (openHydrated || typeof open !== "boolean") return;
    openHydrated = true;
    set({ open });
  },

  refresh: async (vaultId) => {
    if (!vaultId) {
      set({ index: null });
      return;
    }
    try {
      const result = await agentApi.conversations.list(vaultId);
      set({ index: result.index ?? null });
    } catch {
      set({ index: null });
    }
  },

  setSearch: (vaultId, text) => {
    set({ searchText: text });
    if (searchTimer) clearTimeout(searchTimer);
    const trimmed = text.trim();
    if (!vaultId || trimmed === "") {
      set({ hits: [], searching: false });
      return;
    }
    set({ searching: true });
    searchTimer = setTimeout(() => {
      agentApi.conversations
        .search(vaultId, trimmed)
        .then((result) => {
          if (get().searchText.trim() === trimmed) set({ hits: result.hits ?? [], searching: false });
        })
        .catch(() => {
          if (get().searchText.trim() === trimmed) set({ hits: [], searching: false });
        });
    }, SEARCH_DEBOUNCE_MS);
  },

  openViewer: async (vaultId, id) => {
    if (!vaultId) return;
    try {
      const read = await agentApi.conversations.get(vaultId, id);
      if (!read.meta) return;
      set({ viewer: { id, meta: read.meta, lines: eventsToLines(read.events ?? []) } });
    } catch {
      // A missing or unreadable conversation simply does not open.
    }
  },

  closeViewer: () => set({ viewer: null }),

  continueConversation: async (vaultId) => {
    const viewer = get().viewer;
    if (!vaultId || !viewer) return;
    if (Object.values(useAgentStore.getState().buckets).some((bucket) => bucket.busy)) return;
    try {
      await loadIntoChat(vaultId, viewer.id);
    } catch {
      return;
    }
    set({ viewer: null });
    await get().refresh(vaultId);
  },

  rename: async (vaultId, id, title) => {
    try {
      await agentApi.conversations.rename(vaultId, id, title);
    } catch {
      return;
    }
    await get().refresh(vaultId);
  },

  togglePin: async (vaultId, id, pinned) => {
    try {
      await agentApi.conversations.pin(vaultId, id, pinned);
    } catch {
      return;
    }
    await get().refresh(vaultId);
  },

  remove: async (vaultId, id) => {
    try {
      await agentApi.conversations.remove(vaultId, id);
    } catch {
      return;
    }
    await get().refresh(vaultId);
  },

  clearAll: async (vaultId) => {
    try {
      await agentApi.conversations.clear(vaultId);
    } catch {
      return;
    }
    await get().refresh(vaultId);
  },

  exportConversation: async (vaultId, id) => {
    try {
      const result = await agentApi.conversations.export(vaultId, id);
      return result.saved;
    } catch {
      return false;
    }
  },

  restoreOnStart: async (vaultId) => {
    if (!vaultId || restoredFor.has(vaultId)) return;
    restoredFor.add(vaultId);
    await get().refresh(vaultId);
    const index = get().index;
    const id = autoRestoreId(index);
    if (!id) return;
    // Only restore into an empty chat: a page reload during a live exchange
    // must not replace what is on screen.
    const agent = useAgentStore.getState().agent;
    const bucket = useAgentStore.getState().buckets[agent];
    if (bucket.transcript.lines.some((line) => line.role === "user") || bucket.busy) return;
    const meta = conversationTitle(index, id);
    try {
      await loadIntoChat(vaultId, id);
      set({ restoreNote: { kind: "restored", updatedAt: meta?.updatedAt ?? "" } });
    } catch {
      set({ restoreNote: { kind: "failed" } });
    }
  },

  startNewConversation: async (vaultId) => {
    set({ restoreNote: null });
    if (!vaultId) return;
    await useAgentStore.getState().newConversation();
    await get().refresh(vaultId);
  },
}));

// E2E hook (AC-M2): the acceptance script reads the rail and searches.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbHistory?: unknown }).__mkbHistory = useHistoryStore;
}
