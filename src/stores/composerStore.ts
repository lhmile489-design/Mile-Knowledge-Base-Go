// Composer state (T13): text, route preview, manual override, runtime
// parameters and the persisted height. The routing rules live in lib/routing
// so the composer and the tests share them.
import { create } from "zustand";
import type { AgentListResult, AgentPatch, EngineStatus, RoutePreviewResult } from "../mygo";
import { agentApi } from "../lib/agentApi";
import type { AgentId } from "../lib/agentMeta";
import {
  executionModeOfApproval,
  EXECUTION_MODE_TO_APPROVAL,
  normalizeExecutionMode,
  type ExecutionMode,
} from "../lib/executionMode";
import { resolveFollowUpRoute, type FollowUpRouteKind } from "../lib/routing";
import { settingsApi } from "../lib/transport";
import { useAgentStore } from "./agentStore";
import { useAppStore } from "./appStore";

export const MAX_PROMPT_CHARS = 32_000;
const PREVIEW_DEBOUNCE_MS = 180;
const HEIGHT_MIN = 72;
const HEIGHT_MAX = 400;
const HEIGHT_STEP = 16;

export function clampComposerHeight(value: number): number {
  if (!Number.isFinite(value)) return HEIGHT_MIN;
  return Math.min(HEIGHT_MAX, Math.max(HEIGHT_MIN, Math.round(value)));
}

interface ComposerState {
  text: string;
  preview: RoutePreviewResult | null;
  override: AgentId | null;
  rerouteRequested: boolean;
  pickerOpen: boolean;
  height: number;
  session: AgentListResult | null;
  paramsError: string | null;
  setText(text: string): void;
  togglePicker(): void;
  closePicker(): void;
  chooseAgent(agent: AgentId | null): void;
  setHeight(height: number, persist: boolean): void;
  toggleParams(): void;
  loadSession(vaultId: string | null): Promise<void>;
  updateModel(vaultId: string, model: string): Promise<void>;
  updateExecutionMode(vaultId: string, mode: ExecutionMode): Promise<void>;
  applyEngineStatus(vaultId: string, status: EngineStatus): void;
  send(vaultId: string | null): Promise<void>;
  route(): { agent: AgentId; kind: FollowUpRouteKind };
  hasConversation(): boolean;
}

let previewTimer: ReturnType<typeof setTimeout> | null = null;

function hasUserLine(agent: AgentId): boolean {
  const bucket = useAgentStore.getState().buckets[agent];
  return bucket.transcript.lines.some((line) => line.role === "user");
}

export const useComposerStore = create<ComposerState>()((set, get) => ({
  text: "",
  preview: null,
  override: null,
  rerouteRequested: false,
  pickerOpen: false,
  height: 96,
  session: null,
  paramsError: null,

  setText: (text) => {
    set({ text, paramsError: null });
    if (previewTimer) clearTimeout(previewTimer);
    const trimmed = text.trim();
    if (trimmed === "") {
      set({ preview: null });
      return;
    }
    previewTimer = setTimeout(() => {
      agentApi
        .routePreview(trimmed)
        .then((result) => {
          if (get().text.trim() === trimmed) set({ preview: result });
        })
        .catch(() => {
          // Preview is advisory; a failure leaves the previous one.
        });
    }, PREVIEW_DEBOUNCE_MS);
  },

  togglePicker: () => set({ pickerOpen: !get().pickerOpen }),
  closePicker: () => set({ pickerOpen: false }),

  chooseAgent: (agent) => {
    if (agent === null) {
      // "自动" applies to the next message only, then sticky again.
      set({ override: null, rerouteRequested: true, pickerOpen: false });
      return;
    }
    useAgentStore.getState().setAgent(agent);
    set({ override: agent, pickerOpen: false });
  },

  setHeight: (height, persist) => {
    const next = clampComposerHeight(height);
    set({ height: next });
    if (persist) {
      void useAppStore.getState().setLayout({ agnesComposerHeight: next });
    }
  },

  toggleParams: () => {
    const current = useAppStore.getState().settings?.layout?.agnesControlsCollapsed ?? false;
    void useAppStore.getState().setLayout({ agnesControlsCollapsed: !current });
  },

  loadSession: async (vaultId) => {
    if (!vaultId) {
      set({ session: null });
      return;
    }
    const agent = useAgentStore.getState().agent;
    try {
      const result = await agentApi.list(agent, vaultId);
      if (useAgentStore.getState().agent === agent) set({ session: result });
    } catch {
      set({ session: null });
    }
  },

  updateModel: async (vaultId, model) => {
    const agent = useAgentStore.getState().agent;
    const patch: AgentPatch = { model };
    try {
      await settingsApi.appConfigPatchAgent(vaultId, agent, patch);
      set({ paramsError: null });
    } catch (error) {
      set({ paramsError: error instanceof Error ? error.message : "保存模型失败" });
      return;
    }
    await get().loadSession(vaultId);
  },

  updateExecutionMode: async (vaultId, mode) => {
    const agent = useAgentStore.getState().agent;
    const approval = EXECUTION_MODE_TO_APPROVAL[normalizeExecutionMode(mode)];
    try {
      await settingsApi.appConfigPatchAgent(vaultId, agent, { approval });
      set({ paramsError: null });
    } catch (error) {
      set({ paramsError: error instanceof Error ? error.message : "保存执行模式失败" });
      return;
    }
    await get().loadSession(vaultId);
  },

  applyEngineStatus: (vaultId, status) => {
    const previous = get().session?.engine;
    const changed = previous?.phase !== status.phase || previous?.message !== status.message;
    if (!changed) return;
    set((state) => ({
      session: state.session ? { ...state.session, engine: status } : null,
    }));
    if (status.phase === "ready") void get().loadSession(vaultId);
  },

  hasConversation: () => hasUserLine(useAgentStore.getState().agent),

  route: () => {
    const override = get().override;
    const currentTarget = useAgentStore.getState().agent;
    const decisionAgent = get().preview?.agent ?? currentTarget;
    return resolveFollowUpRoute({
      decisionAgent: decisionAgent as AgentId,
      override,
      currentTarget,
      hasCurrentConversation: hasUserLine(currentTarget),
      rerouteRequested: get().rerouteRequested,
    });
  },

  send: async (vaultId) => {
    const trimmed = get().text.trim();
    if (!vaultId || trimmed === "") return;
    if (trimmed.length > MAX_PROMPT_CHARS) return;
    if (useAgentStore.getState().buckets[useAgentStore.getState().agent].busy) return;

    // The composer decides who handles this message: an override wins, an
    // explicit "auto" or a fresh conversation follows the preview.
    let decisionAgent = get().preview?.agent as AgentId | undefined;
    if (!decisionAgent) {
      try {
        decisionAgent = (await agentApi.routePreview(trimmed)).agent as AgentId;
      } catch {
        decisionAgent = useAgentStore.getState().agent;
      }
    }
    const currentTarget = useAgentStore.getState().agent;
    const route = resolveFollowUpRoute({
      decisionAgent: decisionAgent ?? currentTarget,
      override: get().override,
      currentTarget,
      hasCurrentConversation: hasUserLine(currentTarget),
      rerouteRequested: get().rerouteRequested,
    });

    useAgentStore.getState().setAgent(route.agent);
    set({ text: "", preview: null, rerouteRequested: false, pickerOpen: false });
    await useAgentStore.getState().send(trimmed);
  },
}));

/** The execution mode shown in the parameters bar. */
export function currentExecutionMode(session: AgentListResult | null): ExecutionMode {
  return executionModeOfApproval(session?.session?.approval);
}

/** The stored reasoning effort; the control is disabled (D-002). */
export function currentEffort(session: AgentListResult | null): string {
  return session?.session?.effort ?? "auto";
}

export { HEIGHT_MAX, HEIGHT_MIN, HEIGHT_STEP };
