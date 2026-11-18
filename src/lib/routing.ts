// Follow-up route: override > explicit "auto"/no conversation > sticky.
// Mirrors the Go ResolveFollowUpRoute (T07) so the composer and the server
// agree without another binding.
import type { AgentId } from "./agentMeta";

export type FollowUpRouteKind = "auto" | "continue" | "override";

export interface FollowUpRouteInput {
  /** The agent the route preview picked for this text. */
  readonly decisionAgent: AgentId;
  readonly override: AgentId | null;
  /** The agent of the conversation on screen. */
  readonly currentTarget: AgentId;
  readonly hasCurrentConversation: boolean;
  /** The user explicitly chose "自动" for the next message. */
  readonly rerouteRequested: boolean;
}

export interface FollowUpRoute {
  readonly agent: AgentId;
  readonly kind: FollowUpRouteKind;
}

export function resolveFollowUpRoute(input: FollowUpRouteInput): FollowUpRoute {
  if (input.override !== null) return { agent: input.override, kind: "override" };
  if (input.rerouteRequested || !input.hasCurrentConversation) {
    return { agent: input.decisionAgent, kind: "auto" };
  }
  return { agent: input.currentTarget, kind: "continue" };
}
