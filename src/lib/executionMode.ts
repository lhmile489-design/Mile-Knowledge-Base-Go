// Execution mode is the UI concept; ask/auto/yolo are the protocol values.
// The mapping below is the only place the two layers touch (ported from
// executionMode.ts). ask and auto both prompt for every write (D-002); only
// full skips the cards, and the system-area rules never change.
export const EXECUTION_MODES = ["manual", "auto", "full"] as const;
export type ExecutionMode = (typeof EXECUTION_MODES)[number];

export const EXECUTION_MODE_TO_APPROVAL: Record<ExecutionMode, string> = {
  manual: "ask",
  auto: "auto",
  full: "yolo",
};

export const APPROVAL_TO_EXECUTION_MODE: Record<string, ExecutionMode> = {
  ask: "manual",
  auto: "auto",
  yolo: "full",
};

export interface ExecutionModeDescription {
  readonly mode: ExecutionMode;
  readonly label: string;
  readonly summary: string;
  readonly consequence: string | null;
}

const DESCRIPTIONS: Record<ExecutionMode, ExecutionModeDescription> = {
  manual: {
    mode: "manual",
    label: "手动执行",
    summary: "每次写入、移动或删除都先问你",
    consequence: null,
  },
  auto: {
    mode: "auto",
    label: "自动执行",
    summary: "每次改动知识库仍要你同意",
    consequence: null,
  },
  full: {
    mode: "full",
    label: "全自动",
    summary: "不再逐条询问，内容区操作直接执行",
    consequence:
      "不再弹出审批卡；四个 Agent 可写知识库内容区，但应用配置、事务恢复记录、索引、来源关系等系统区仍禁止写入。",
  },
};

export function describeExecutionMode(mode: ExecutionMode): ExecutionModeDescription {
  return DESCRIPTIONS[mode];
}

export function isExecutionMode(value: unknown): value is ExecutionMode {
  return typeof value === "string" && (EXECUTION_MODES as readonly string[]).includes(value);
}

/** Unknown values fall back to the strictest mode. */
export function normalizeExecutionMode(value: unknown): ExecutionMode {
  return isExecutionMode(value) ? value : "manual";
}

export function executionModeOfApproval(approval: unknown): ExecutionMode {
  return APPROVAL_TO_EXECUTION_MODE[String(approval)] ?? "manual";
}
