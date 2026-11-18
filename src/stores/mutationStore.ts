import { create } from "zustand";
import {
  mutationApi,
  type MutationPlan,
  type MutationReport,
  type RestorePreview,
} from "../lib/transport";
import { useAppStore } from "./appStore";
import { useTreeStore } from "./treeStore";
import { useDocStore } from "./docStore";

/**
 * Pending change plan and the running state. The plan card is the only place
 * a plan is confirmed; nothing here writes until confirm() (PRD FR-MUT).
 */
type MutationState = {
  pending: MutationPlan | null;
  title: string;
  running: boolean;
  report: MutationReport | null;
  error: string | null;
  preview: RestorePreview | null;

  propose: (plan: MutationPlan, title?: string) => void;
  cancel: () => void;
  confirm: (freshApproval: boolean) => Promise<MutationReport | null>;
  dismissReport: () => void;
  prepareRestore: (transactionID: string) => Promise<void>;
  confirmRestore: () => Promise<void>;
  cancelRestore: () => void;
};

export const useMutationStore = create<MutationState>((set, get) => ({
  pending: null,
  title: "",
  running: false,
  report: null,
  error: null,
  preview: null,

  propose: (plan, title = "") => {
    set({ pending: plan, title, report: null, error: null });
  },

  cancel: () => set({ pending: null, title: "", error: null }),

  confirm: async (freshApproval) => {
    const plan = get().pending;
    if (!plan) return null;
    const vaultId = useAppStore.getState().activeVaultId;
    set({ running: true, error: null });
    try {
      const report = await mutationApi.execute({ ...plan, freshApproval });
      set({ running: false, pending: null, report });
      // Refresh what the plan touched immediately instead of waiting for the
      // watcher (legacy mutationStore did the same).
      if (vaultId && report.touchedDirectories?.length) {
        await useTreeStore.getState().invalidate([...report.touchedDirectories, ""]);
        void useDocStore.getState().applyExternalChange(vaultId, report.touchedDirectories);
      }
      return report;
    } catch (error) {
      set({ running: false, error: error instanceof Error ? error.message : String(error) });
      return null;
    }
  },

  dismissReport: () => set({ report: null, error: null }),

  prepareRestore: async (transactionID) => {
    const vaultId = useAppStore.getState().activeVaultId;
    if (!vaultId) return;
    try {
      const preview = await mutationApi.rewindPrepare(vaultId, transactionID);
      set({ preview });
    } catch (error) {
      set({ error: error instanceof Error ? error.message : String(error) });
    }
  },

  confirmRestore: async () => {
    const preview = get().preview;
    const vaultId = useAppStore.getState().activeVaultId;
    if (!preview || !vaultId) return;
    set({ running: true });
    try {
      const result = await mutationApi.rewindCommit(vaultId, preview.transactionId);
      set({ running: false, preview: null });
      if (result.touchedDirectories?.length) {
        await useTreeStore.getState().invalidate([...result.touchedDirectories, ""]);
        void useDocStore.getState().applyExternalChange(vaultId, result.touchedDirectories);
      }
    } catch (error) {
      set({ running: false, error: error instanceof Error ? error.message : String(error) });
    }
  },

  cancelRestore: () => set({ preview: null }),
}));

// E2E hook: the plan store used by the card.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbMutation?: unknown }).__mkbMutation = useMutationStore;
}
