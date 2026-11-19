// Overview tiles data (M3 T09): recent documents, index status, health, the
// drafts count and the recent audit records. Everything reloads on the events
// that can change it (E01 vault-changed, E02 index-status, E03 audit-changed).
import { create } from "zustand";
import type { ActionRecord, IndexStatus, KbHealthReport, RecentDocument } from "../mygo";
import { docApi, indexApi, mutationApi, TransportError } from "../lib/transport";
import { draftsCount } from "../lib/overview";

interface OverviewState {
  vaultId: string | null;
  recent: RecentDocument[];
  recentLoading: boolean;
  recentError: string | null;
  status: IndexStatus | null;
  health: KbHealthReport | null;
  drafts: number;
  changes: ActionRecord[];

  load(vaultId: string | null): Promise<void>;
  /** Debounced reload, used by the event subscriptions. */
  scheduleLoad(vaultId: string): void;
  setStatus(status: IndexStatus): void;
}

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  if (error instanceof Error) return error.message;
  return String(error);
}

let reloadTimer: number | null = null;
let requestSeq = 0;

export const useOverviewStore = create<OverviewState>()((set, get) => ({
  vaultId: null,
  recent: [],
  recentLoading: false,
  recentError: null,
  status: null,
  health: null,
  drafts: 0,
  changes: [],

  load: async (vaultId) => {
    if (!vaultId) {
      set({
        vaultId: null, recent: [], recentLoading: false, recentError: null,
        status: null, health: null, drafts: 0, changes: [],
      });
      return;
    }
    const seq = (requestSeq += 1);
    if (get().vaultId !== vaultId) {
      set({ vaultId, recent: [], recentLoading: true, recentError: null, status: null, health: null, drafts: 0, changes: [] });
    } else {
      set({ recentLoading: true });
    }
    const recentTask = docApi
      .recent(vaultId)
      .then((result) => ({ documents: result.documents ?? [], error: null as string | null }))
      .catch((error: unknown) => ({ documents: [], error: errorText(error) }));
    const [recentState, status, health, drafts, audit] = await Promise.all([
      recentTask,
      indexApi.status(vaultId).catch(() => null),
      indexApi.health(vaultId).catch(() => null),
      docApi
        .tree(vaultId, ".mile-kb/drafts")
        .then((result) => draftsCount(result.entries))
        .catch(() => 0),
      mutationApi.auditList(vaultId, "", 5).catch(() => null),
    ]);
    // 慢响应不得覆盖新库的数据（legacy requestSeq）。
    if (seq !== requestSeq || get().vaultId !== vaultId) return;
    set({
      recent: recentState.documents,
      recentLoading: false,
      recentError: recentState.error,
      status,
      health,
      drafts,
      changes: audit?.records ?? [],
    });
  },

  scheduleLoad: (vaultId) => {
    if (reloadTimer !== null) window.clearTimeout(reloadTimer);
    reloadTimer = window.setTimeout(() => {
      reloadTimer = null;
      void get().load(vaultId);
    }, 250);
  },

  setStatus: (status) => set({ status }),
}));
