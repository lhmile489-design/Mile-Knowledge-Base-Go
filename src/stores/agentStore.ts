// Chat state for the four agents (T11). Frames arrive from agentApi.prompt and
// go through the pure reducer in lib/chatStream: only the last line is
// replaced while streaming, so finished messages keep their React identity.
import { create } from "zustand";
import type { StreamFrame } from "../mygo";
import { agentApi, type FrameStream } from "../lib/agentApi";
import { AGENT_IDS, type AgentId } from "../lib/agentMeta";
import {
  applyFrame,
  applyStreamError,
  emptyBucket,
  startTurn,
  type ChatBucket,
} from "../lib/chatStream";
import { TransportError } from "../lib/transport";

let nextLineId = 0;
const makeId = (): string => `line-${(nextLineId += 1)}`;

function emptyBuckets(): Record<AgentId, ChatBucket> {
  const buckets = {} as Record<AgentId, ChatBucket>;
  for (const id of AGENT_IDS) buckets[id] = emptyBucket();
  return buckets;
}

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  if (error instanceof Error) return error.message;
  return String(error);
}

interface AgentState {
  vaultId: string | null;
  agent: AgentId;
  buckets: Record<AgentId, ChatBucket>;
  stream: FrameStream | null;
  setVault(vaultId: string | null): void;
  setAgent(agent: AgentId): void;
  send(text: string): Promise<void>;
  retry(text: string): Promise<void>;
  cancel(): Promise<void>;
  approve(requestId: string): Promise<void>;
  reject(requestId: string): Promise<void>;
  newConversation(): Promise<void>;
  lastUserText(): string | null;
}

export const useAgentStore = create<AgentState>()((set, get) => ({
  vaultId: null,
  agent: "general",
  buckets: emptyBuckets(),
  stream: null,

  setVault: (vaultId) => {
    if (get().vaultId === vaultId) return;
    get().stream?.abort();
    set({ vaultId, buckets: emptyBuckets(), stream: null });
  },

  setAgent: (agent) => set({ agent }),

  send: async (text) => {
    const state = get();
    const vaultId = state.vaultId;
    const agent = state.agent;
    const trimmed = text.trim();
    if (!vaultId || !trimmed) return;
    if (state.buckets[agent].busy) return;

    const beginTurn = (bucket: ChatBucket, at: string): ChatBucket =>
      startTurn(bucket, trimmed, at, makeId);

    set({
      buckets: {
        ...state.buckets,
        [agent]: beginTurn(state.buckets[agent], new Date().toISOString()),
      },
    });

    const stream = agentApi.prompt(agent, vaultId, trimmed, (frame: StreamFrame) => {
      const latest = get();
      if (latest.vaultId !== vaultId) return;
      set({
        buckets: {
          ...latest.buckets,
          [agent]: applyFrame(
            latest.buckets[agent],
            frame,
            makeId,
            new Date().toISOString(),
          ),
        },
      });
    });
    set({ stream });

    try {
      await stream.done;
    } catch (error) {
      const latest = get();
      if (latest.vaultId === vaultId) {
        set({
          buckets: {
            ...latest.buckets,
            [agent]: applyStreamError(
              latest.buckets[agent],
              errorText(error),
              new Date().toISOString(),
              makeId,
            ),
          },
        });
      }
    } finally {
      if (get().stream === stream) set({ stream: null });
    }
  },

  retry: async (text) => {
    await get().send(text);
  },

  cancel: async () => {
    try {
      await agentApi.cancel(get().agent);
    } catch {
      // A cancel with no running turn is a silent success on the Go side.
    }
  },

  approve: async (requestId) => {
    try {
      await agentApi.approve(requestId, "allow_once");
    } catch (error) {
      // The card may have expired meanwhile; the Go side pushes the expired
      // frame and the card disappears by itself.
      if (error instanceof TransportError) return;
      throw error;
    }
  },

  reject: async (requestId) => {
    try {
      await agentApi.reject(requestId);
    } catch (error) {
      if (error instanceof TransportError) return;
      throw error;
    }
  },

  newConversation: async () => {
    const vaultId = get().vaultId;
    if (!vaultId) return;
    get().stream?.abort();
    try {
      await agentApi.reset(vaultId);
    } catch {
      // Reset failures must not block starting a fresh transcript.
    }
    set({ buckets: emptyBuckets(), stream: null });
  },

  lastUserText: () => {
    const lines = get().buckets[get().agent].transcript.lines;
    for (let i = lines.length - 1; i >= 0; i -= 1) {
      const line = lines[i];
      if (line && line.role === "user") return line.text;
    }
    return null;
  },
}));

// E2E hook (AC-M2): the acceptance script drives the chat through the store.
if (typeof window !== "undefined") {
  (window as unknown as { __mkbAgent?: unknown }).__mkbAgent = useAgentStore;
}
