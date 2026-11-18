import { create } from "zustand";
import { mutationApi, type MutationPlan, type MutationReport } from "../lib/transport";
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

  propose: (plan: MutationPlan, title?: string) => void;
  cancel: () => void;
  confirm: (freshApproval: boolean) => Promise<MutationReport | null>;
  dismissReport: () => void;
};

export const useMutationStore = create<MutationState>((set, get) => ({
  pending: null,
  title: "",
  running: false,
  report: null,
  error: null,

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
}));

// E2E hook: the plan store used by the card.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbMutation?: unknown }).__mkbMutation = useMutationStore;
}
