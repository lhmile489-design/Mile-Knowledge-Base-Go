// Agent display metadata (ported from agentMeta.ts). Names and scopes describe
// responsibilities only; permissions are uniform across the four agents and
// are expressed by the Go side's policy, never by a second local copy.
import { Archive, ChatCircle, Flask, MagnifyingGlass, type Icon } from "@phosphor-icons/react";

export const AGENT_IDS = ["curator", "scout", "distiller", "general"] as const;
export type AgentId = (typeof AGENT_IDS)[number];

export function isAgentId(value: unknown): value is AgentId {
  return typeof value === "string" && (AGENT_IDS as readonly string[]).includes(value);
}

export interface AgentMeta {
  readonly id: AgentId;
  readonly name: string;
  readonly latin: string;
  readonly scope: string;
  readonly icon: Icon;
}

export const AGENT_META: Record<AgentId, AgentMeta> = {
  curator: {
    id: "curator",
    name: "整理归档",
    latin: "Curator",
    scope: "分类、移动、归档、索引维护",
    icon: Archive,
  },
  scout: {
    id: "scout",
    name: "推荐检索",
    latin: "Scout",
    scope: "全文检索、推荐与来源解释",
    icon: MagnifyingGlass,
  },
  distiller: {
    id: "distiller",
    name: "蒸馏学习",
    latin: "Distiller",
    scope: "提炼原始材料、发现冲突、生成草稿",
    icon: Flask,
  },
  general: {
    id: "general",
    name: "自由问答",
    latin: "General",
    scope: "开放问答、解释与跨领域分析",
    icon: ChatCircle,
  },
};
