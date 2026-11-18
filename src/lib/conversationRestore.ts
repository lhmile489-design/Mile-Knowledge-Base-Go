// Startup restore rules (ported from conversationRestore.ts): restore the
// active conversation only when it has content, under the agent the user
// last asked.
import type { ConversationEvent, ConversationIndex } from "../mygo";
import { isAgentId, type AgentId } from "./agentMeta";

export function autoRestoreId(index: ConversationIndex | null): string | null {
  if (!index?.activeId) return null;
  const active = index.activeId;
  const meta = index.conversations.find((item) => item.id === active);
  if (!meta || meta.turnCount <= 0) return null;
  return active;
}

export function restoreTargetAgent(events: readonly ConversationEvent[]): AgentId | null {
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const event = events[i];
    if (event?.t === "user" && isAgentId(event.agent)) return event.agent;
  }
  return null;
}
