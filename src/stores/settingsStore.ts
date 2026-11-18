// Settings page state for the Agents section (T18): the saved config, the
// unsaved steering drafts, immediate patches and the leave guard.
import { create } from "zustand";
import type { AgentConfig, AgentPatch, AppConfigResult, SkillEntry } from "../mygo";
import type { AgentId } from "../lib/agentMeta";
import { dirtyAgents } from "../lib/agentSettings";
import { settingsApi, TransportError } from "../lib/transport";
import { useComposerStore } from "./composerStore";

export type LeaveTarget = "close" | "model" | "agents";

interface SettingsState {
  config: AppConfigResult | null;
  skills: SkillEntry[];
  drafts: Partial<Record<AgentId, string>>;
  error: string | null;
  leaveTarget: LeaveTarget | null;

  load(vaultId: string | null): Promise<void>;
  setDraft(agent: AgentId, text: string): void;
  discardDrafts(): void;
  saveDrafts(vaultId: string | null): Promise<void>;
  patchAgent(vaultId: string | null, agent: AgentId, patch: AgentPatch): Promise<void>;
  toggleSkill(vaultId: string | null, agent: AgentId, path: string, checked: boolean): Promise<void>;

  /** Returns true when leaving may proceed; otherwise opens the guard. */
  requestLeave(target: LeaveTarget): boolean;
  /** confirm=true discards drafts and returns the pending target. */
  resolveLeave(confirm: boolean): LeaveTarget | null;
  isDirty(): boolean;
}

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  if (error instanceof Error) return error.message;
  return String(error);
}

export const useSettingsStore = create<SettingsState>()((set, get) => ({
  config: null,
  skills: [],
  drafts: {},
  error: null,
  leaveTarget: null,

  load: async (vaultId) => {
    if (!vaultId) {
      set({ config: null, skills: [] });
      return;
    }
    try {
      const [config, catalog] = await Promise.all([
        settingsApi.appConfigGet(vaultId),
        settingsApi.skillCatalog(vaultId).catch(() => ({ skills: [] })),
      ]);
      set({ config, skills: catalog.skills ?? [], error: null });
    } catch (error) {
      set({ error: errorText(error) });
    }
  },

  setDraft: (agent, text) => set((state) => ({ drafts: { ...state.drafts, [agent]: text } })),

  discardDrafts: () => set({ drafts: {}, leaveTarget: null }),

  saveDrafts: async (vaultId) => {
    if (!vaultId) return;
    const { drafts, config } = get();
    const dirty = dirtyAgents(drafts, config?.agents);
    if (dirty.length === 0) {
      set({ drafts: {} });
      return;
    }
    try {
      let latest: AppConfigResult | null = null;
      for (const agent of dirty) {
        const text = drafts[agent];
        if (text === undefined) continue;
        latest = await settingsApi.appConfigPatchAgent(vaultId, agent, { steering: text });
      }
      set({ drafts: {}, config: latest ?? config, error: null });
      void useComposerStore.getState().loadSession(vaultId);
    } catch (error) {
      set({ error: errorText(error) });
    }
  },

  patchAgent: async (vaultId, agent, patch) => {
    if (!vaultId) return;
    try {
      const result = await settingsApi.appConfigPatchAgent(vaultId, agent, patch);
      set({ config: result, error: null });
      void useComposerStore.getState().loadSession(vaultId);
    } catch (error) {
      set({ error: errorText(error) });
    }
  },

  toggleSkill: async (vaultId, agent, path, checked) => {
    const current = get().config?.agents?.[agent];
    if (!current) return;
    const next = checked
      ? current.skillPaths.includes(path)
        ? current.skillPaths
        : [...current.skillPaths, path]
      : current.skillPaths.filter((item) => item !== path);
    await get().patchAgent(vaultId, agent, { skillPaths: next });
  },

  requestLeave: (target) => {
    if (!get().isDirty()) return true;
    set({ leaveTarget: target });
    return false;
  },

  resolveLeave: (confirm) => {
    const target = get().leaveTarget;
    set({ leaveTarget: null });
    if (!confirm || !target) return null;
    set({ drafts: {} });
    return target;
  },

  isDirty: () => {
    const { drafts, config } = get();
    return dirtyAgents(drafts, config?.agents).length > 0;
  },
}));

export type { AgentConfig };
