import { memo, useEffect, useMemo, useRef, useState } from "react";
import { ArrowClockwise, CaretRight, Copy } from "@phosphor-icons/react";
import { AGENT_META, type AgentId } from "../../lib/agentMeta";
import { lineTimeLabel } from "../../lib/conversationTime";
import { flattenSegments } from "../../lib/turnProcess";
import { buildProcess, formatThoughtMs, type ProcessSummary } from "../../lib/turnProcess";
import type { TranscriptSegment } from "../../lib/transcriptGroups";
import { turnStartedAt, type TurnStatus } from "../../lib/turnGroups";
import { MarkdownBlock } from "./MarkdownBlock";
import { EvidenceRadar } from "./EvidenceRadar";

const STATUS_META: Record<TurnStatus, { readonly label: string; readonly tone: string }> = {
  running: { label: "进行中", tone: "info" },
  waiting: { label: "等待你确认", tone: "warn" },
  done: { label: "已完成", tone: "live" },
  failed: { label: "失败", tone: "fault" },
  cancelled: { label: "已取消", tone: "idle" },
};

interface AgentMessageProps {
  readonly agent: AgentId;
  readonly segments: readonly TranscriptSegment[];
  readonly status: TurnStatus;
  readonly live: boolean;
  readonly userAt?: string;
  readonly canRetry: boolean;
  readonly onRetry: (text: string) => void;
}

/** Agent message: label row (avatar, names, time, status pill), the process
 * summary, the answer text and the per-message actions. Markdown rendering
 * arrives with T12; T11 keeps the answer as plain, pre-wrapped text. */
export const AgentMessage = memo(function AgentMessage({
  agent,
  segments,
  status,
  live,
  userAt,
  canRetry,
  onRetry,
}: AgentMessageProps) {
  const meta = AGENT_META[agent];
  const Icon = meta.icon;
  const now = useMemo(() => new Date(), []);
  const startedAt = turnStartedAt(segments, userAt);
  const time = lineTimeLabel(startedAt, now);
  const state = STATUS_META[status];

  const process = useMemo(() => buildProcess(segments, live), [segments, live]);
  const lines = useMemo(() => flattenSegments(segments), [segments]);
  const answers = useMemo(
    () => lines.filter((line) => line.role === "agent").map((line) => line.text),
    [lines],
  );
  const answerText = answers.join("\n\n");
  const notes = useMemo(
    () => lines.filter((line) => line.role === "system" && line.toolCallId === undefined),
    [lines],
  );

  return (
    <article className="msg-agent" data-testid="msg-agent" data-agent={agent} aria-label={`${meta.name}的回答`}>
      <div className="agent-label" data-testid="agent-label">
        <span className="agent-avatar" aria-hidden>
          <Icon size={14} aria-hidden />
        </span>
        <span className="agent-name">{meta.name}</span>
        <span className="agent-latin">{meta.latin}</span>
        {time ? <span className="agent-time">{time}</span> : null}
        <span className="pill" data-tone={state.tone} data-state={status} data-testid="msg-status">
          {state.label}
        </span>
      </div>

      <div className="agent-content">
        {process ? <ProcessSummaryView process={process} live={live} /> : null}

        {answers.map((text, index) => (
          <MarkdownBlock
            key={`answer-${index}`}
            text={text}
            streaming={live && index === answers.length - 1}
          />
        ))}

        {notes.map((line) => (
          <p key={line.id} className="chat-note" data-testid="turn-note">
            {line.text}
          </p>
        ))}

        <EvidenceRadar lines={lines} />

        {!live && answerText ? (
          <MsgActions text={answerText} retryText={canRetry ? answerText : null} onRetry={onRetry} />
        ) : null}
      </div>
    </article>
  );
});

/** "N steps done · thought x.xs", collapsed by default; expanded steps list
 * the same facts on one timeline. While live the current step is shown with
 * the authorized breathing dots. */
const ProcessSummaryView = memo(function ProcessSummaryView({
  process,
  live,
}: {
  readonly process: ProcessSummary;
  readonly live: boolean;
}) {
  const [open, setOpen] = useState(false);
  const duration = process.thoughtMs === null ? null : formatThoughtMs(process.thoughtMs);

  return (
    <div className="process" data-testid="activity-pipeline" data-open={open ? "true" : "false"} aria-label="推理流水线">
      <button
        type="button"
        className="process-summary"
        aria-expanded={open}
        data-testid="process-toggle"
        onClick={() => setOpen(!open)}
      >
        <CaretRight size={11} weight="bold" aria-hidden className="caret" style={{ transform: open ? "rotate(90deg)" : undefined }} />
        {live && process.current ? (
          <span className="thinking-row" data-animate="true" role="status" aria-live="polite">
            <span className="thinking-dots" aria-hidden>
              <span />
              <span />
              <span />
            </span>
            <span className="thinking-label">正在 {process.current}</span>
          </span>
        ) : (
          <>
            <span>已完成 {process.doneCount} 步</span>
            {duration ? <span className="sep">|</span> : null}
            {duration ? <span>思考 {duration}</span> : null}
          </>
        )}
        <span className="process-right">{open ? "收起" : "展开过程"}</span>
      </button>
      {open ? (
        <ol className="steps" data-testid="process-steps">
          {process.steps.map((step) => (
            <li key={step.id} className="step" data-kind={step.kind} data-state={step.state}>
              <span className="step-kind">{step.kind === "tool" ? "执行" : "思考"}</span>
              <span className="step-text">{step.label}</span>
              {step.detail ? <span className="step-detail">{step.detail}</span> : null}
            </li>
          ))}
        </ol>
      ) : null}
    </div>
  );
});

const MsgActions = memo(function MsgActions({
  text,
  retryText,
  onRetry,
}: {
  readonly text: string;
  readonly retryText: string | null;
  readonly onRetry: (text: string) => void;
}) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      return;
    }
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="msg-actions" data-testid="msg-actions">
      <button type="button" className="btn" data-testid="msg-copy" onClick={() => void copy()}>
        <Copy size={12} weight="bold" aria-hidden />
        {copied ? "已复制" : "复制"}
      </button>
      {retryText ? (
        <button type="button" className="btn" data-testid="msg-retry" onClick={() => onRetry(retryText)}>
          <ArrowClockwise size={12} weight="bold" aria-hidden />
          重新回答
        </button>
      ) : null}
    </div>
  );
});
