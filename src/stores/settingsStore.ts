// Settings page state (M2 T18, M3 T04): the saved config, the shared draft
// table, the page-level save bar and the leave guard. Every text control
// writes a draft; saving is one explicit action (bar or Ctrl+S).
import { create } from "zustand";
import type { AgentConfig, AgentPatch, AppConfigResult, CategoryConfig, SkillEntry } from "../mygo";
import { isMeaningfulCategory } from "../lib/categoriesTable";
import type { AgentId } from "../lib/agentMeta";
import { AGENT_IDS } from "../lib/agentMeta";
import {
  aliasDrafts,
  applyAliasDrafts,
  categoryAliasDraftId,
  dirtyDraftIds,
  MAX_CATEGORY_ALIAS,
  overlongAliasDrafts,
  steeringDraftId,
  steeringDrafts,
} from "../lib/settingsDrafts";
import { settingsApi, TransportError } from "../lib/transport";
import { useComposerStore } from "./composerStore";
import { useTreeStore } from "./treeStore";

export type SettingsSection = "general" | "model" | "agents" | "categories" | "about";
export type LeaveTarget = "close" | SettingsSection;

interface SettingsState {
  config: AppConfigResult | null;
  skills: SkillEntry[];
  /** Draft text by id (steering:<agent> | category-alias:<dir>). */
  drafts: Record<string, string>;
  error: string | null;
  leaveTarget: LeaveTarget | null;
  saving: boolean;
  notice: string | null;

  load(vaultId: string | null): Promise<void>;
  /** Steering draft for one agent (the Agents section's text control). */
  setDraft(agent: AgentId, text: string): void;
  /** Display-name draft for one directory (the categories table). */
  setAliasDraft(directory: string, text: string): void;
  /** Drop one alias draft (used when a leftover configuration is cleared). */
  clearAliasDraft(directory: string): void;
  /** Immediate category save for the include-in-search switch (M65). */
  setCategories(vaultId: string | null, categories: CategoryConfig[]): Promise<void>;
  discardDrafts(): void;
  saveDrafts(vaultId: string | null): Promise<void>;
  patchAgent(vaultId: string | null, agent: AgentId, patch: AgentPatch): Promise<void>;
  toggleSkill(vaultId: string | null, agent: AgentId, path: string, checked: boolean): Promise<void>;
  clearNotice(): void;

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
  saving: false,
  notice: null,

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

  setDraft: (agent, text) =>
    set((state) => ({ drafts: { ...state.drafts, [steeringDraftId(agent)]: text }, notice: null })),

  setAliasDraft: (directory, text) =>
    set((state) => ({
      drafts: { ...state.drafts, [categoryAliasDraftId(directory)]: text },
      notice: null,
    })),

  clearAliasDraft: (directory) =>
    set((state) => {
      const id = categoryAliasDraftId(directory);
      if (!(id in state.drafts)) return {};
      const next = { ...state.drafts };
      delete next[id];
      return { drafts: next };
    }),

  setCategories: async (vaultId, categories) => {
    if (!vaultId) return;
    try {
      const result = await settingsApi.appConfigSetCategories(
        vaultId,
        categories.filter(isMeaningfulCategory),
      );
      set({ config: result, error: null });
      // 显示名参与侧栏展示：根层树要重读。
      await useTreeStore.getState().invalidate([""]);
    } catch (error) {
      set({ error: errorText(error) });
    }
  },

  discardDrafts: () => set({ drafts: {}, leaveTarget: null, error: null }),

  saveDrafts: async (vaultId) => {
    if (!vaultId) return;
    const { drafts, config, saving } = get();
    if (saving) return;
    const dirty = dirtyDraftIds(drafts, config);
    if (dirty.length === 0) {
      set({ drafts: {} });
      return;
    }
    const overlong = overlongAliasDrafts(drafts);
    if (overlong.length > 0) {
      // 拒绝而不是截断：截断会让保存条永远亮着（旧版同样处理）。
      set({ error: `显示名不能超过 ${MAX_CATEGORY_ALIAS} 个字符：${overlong.join("、")}` });
      return;
    }
    set({ saving: true, error: null });
    try {
      let latest: AppConfigResult | null = config;
      const steering = steeringDrafts(drafts);
      for (const agent of AGENT_IDS) {
        const text = steering[agent];
        if (text === undefined || !dirty.includes(steeringDraftId(agent))) continue;
        latest = await settingsApi.appConfigPatchAgent(vaultId, agent, { steering: text });
      }
      const aliases = aliasDrafts(drafts);
      const aliasDirty = Object.keys(aliases).some((directory) =>
        dirty.includes(categoryAliasDraftId(directory)),
      );
      if (aliasDirty) {
        const next = applyAliasDrafts(latest?.categories ?? [], drafts);
        latest = await settingsApi.appConfigSetCategories(vaultId, next);
        // 分类显示名参与侧栏展示，根层树要重读。
        await useTreeStore.getState().invalidate([""]);
      }
      // 保存期间又被改动的草稿保留下来，不清掉。
      set((state) => {
        const remaining: Record<string, string> = {};
        for (const [id, text] of Object.entries(state.drafts)) {
          if (dirty.includes(id) && text === drafts[id]) continue;
          remaining[id] = text;
        }
        return {
          drafts: remaining,
          config: latest ?? state.config,
          saving: false,
          notice: "已保存",
        };
      });
      void useComposerStore.getState().loadSession(vaultId);
    } catch (error) {
      set({ error: errorText(error), saving: false });
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

  clearNotice: () => set({ notice: null }),

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
    return dirtyDraftIds(drafts, config).length > 0;
  },
}));

export type { AgentConfig };
