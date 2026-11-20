import { create } from "zustand";
import { skillsApi, TransportError, type SkillScan, type SkillTool } from "../lib/transport";

/** The skill panel's read side (M4 T04): registry + scan payloads. Write
 * operations and the panel UI arrive in later tasks. */
interface SkillsState {
  tools: SkillTool[];
  scan: SkillScan | null;
  loading: boolean;
  error: string | null;
  loadRegistry: () => Promise<void>;
  loadScan: (vaultId: string) => Promise<void>;
  refresh: (vaultId: string) => Promise<void>;
  applyScan: (scan: SkillScan) => void;
}

function message(error: unknown): string {
  return error instanceof TransportError ? error.message : String(error);
}

let scanSeq = 0;

export const useSkillsStore = create<SkillsState>((set) => ({
  tools: [],
  scan: null,
  loading: false,
  error: null,

  loadRegistry: async () => {
    try {
      const result = await skillsApi.registry();
      set({ tools: result.tools, error: null });
    } catch (error) {
      set({ error: message(error) });
    }
  },

  loadScan: async (vaultId) => {
    const seq = ++scanSeq;
    try {
      const result = await skillsApi.scan(vaultId);
      // A newer request won; drop this response.
      if (seq !== scanSeq) return;
      set({ scan: result.scan, error: null });
    } catch (error) {
      if (seq !== scanSeq) return;
      set({ error: message(error) });
    }
  },

  refresh: async (vaultId) => {
    set({ loading: true });
    try {
      const [registry, result] = await Promise.all([skillsApi.registry(), skillsApi.scan(vaultId)]);
      set({ tools: registry.tools, scan: result.scan, error: null });
    } catch (error) {
      set({ error: message(error) });
    } finally {
      set({ loading: false });
    }
  },

  applyScan: (scan) => set({ scan }),
}));

// E2E hook: the same store the panel uses.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbSkills?: unknown }).__mkbSkills = useSkillsStore;
}
