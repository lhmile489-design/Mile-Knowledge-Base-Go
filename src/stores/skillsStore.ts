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

// A background refresh has no reliable completion event (E12 from a plain Go
// goroutine is not delivered), so the store polls briefly until the scan
// reports scanning=false. The Go side answers from a cache younger than two
// seconds without starting another scan, so this cannot loop.
const SCAN_POLL_MS = 1200;
const SCAN_POLL_LIMIT = 20;
let scanPollTimer: ReturnType<typeof setTimeout> | null = null;
let scanPollCount = 0;

function clearScanTimer(): void {
  if (scanPollTimer) {
    clearTimeout(scanPollTimer);
    scanPollTimer = null;
  }
}

function stopScanPoll(): void {
  clearScanTimer();
  scanPollCount = 0;
}

function pollScan(vaultId: string): void {
  if (scanPollTimer || scanPollCount >= SCAN_POLL_LIMIT) return;
  scanPollCount += 1;
  scanPollTimer = setTimeout(() => {
    scanPollTimer = null;
    void useSkillsStore.getState().loadScan(vaultId);
  }, SCAN_POLL_MS);
}

function afterScan(vaultId: string, scanning: boolean): void {
  if (scanning) pollScan(vaultId);
  else stopScanPoll();
}

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
    // A new request supersedes a pending poll; its own result decides again.
    clearScanTimer();
    const seq = ++scanSeq;
    try {
      const result = await skillsApi.scan(vaultId);
      // A newer request won; drop this response.
      if (seq !== scanSeq) return;
      set({ scan: result.scan, error: null });
      afterScan(vaultId, result.scan.scanning);
    } catch (error) {
      if (seq !== scanSeq) return;
      set({ error: message(error) });
    }
  },

  refresh: async (vaultId) => {
    clearScanTimer();
    set({ loading: true });
    try {
      const [registry, result] = await Promise.all([skillsApi.registry(), skillsApi.scan(vaultId)]);
      set({ tools: registry.tools, scan: result.scan, error: null });
      afterScan(vaultId, result.scan.scanning);
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
