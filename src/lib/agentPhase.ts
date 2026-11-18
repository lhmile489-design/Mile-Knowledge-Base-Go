// Turn phase → label and animation intent (ported from agentPhase.ts).
// waiting_permission is "waiting for the user" and must not animate: an
// animation there would suggest work is still happening without them.

export type PhaseTone = "thinking" | "working" | "waiting" | "settled" | "failed";

export interface PhasePresentation {
  readonly label: string;
  readonly tone: PhaseTone;
  readonly animate: boolean;
}

const PHASES: Record<string, PhasePresentation> = {
  idle: { label: "空闲", tone: "settled", animate: false },
  starting: { label: "正在启动", tone: "thinking", animate: true },
  working: { label: "正在思考", tone: "thinking", animate: true },
  compacting: { label: "正在整理前文", tone: "working", animate: true },
  retrying: { label: "供应商暂时不可用，正在重试", tone: "working", animate: true },
  waiting_permission: { label: "等待你确认", tone: "waiting", animate: false },
  completed: { label: "已完成", tone: "settled", animate: false },
  cancelled: { label: "已取消", tone: "settled", animate: false },
  error: { label: "出错了", tone: "failed", animate: false },
};

const FALLBACK: PhasePresentation = { label: "处理中", tone: "working", animate: true };
const PENDING: PhasePresentation = { label: "正在思考", tone: "thinking", animate: true };

export function describePhase(phase: unknown): PhasePresentation {
  if (phase === null || phase === undefined || phase === "") return PENDING;
  if (typeof phase !== "string") return FALLBACK;
  return PHASES[phase] ?? FALLBACK;
}

export const KNOWN_PHASES: readonly string[] = Object.keys(PHASES);
