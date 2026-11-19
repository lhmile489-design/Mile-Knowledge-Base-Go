import { useEffect, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Warning } from "@phosphor-icons/react";
import { useAgentStore } from "../../stores/agentStore";
import { OverviewGrid } from "../overview/OverviewGrid";
import { groupTranscript, type TranscriptSegment } from "../../lib/transcriptGroups";
import { groupTurns, splitTurn, turnStatus, type TurnGroup } from "../../lib/turnGroups";
import type { AgentId } from "../../lib/agentMeta";
import { AgentMessage } from "./AgentMessage";
import { UserMessage } from "./UserMessage";
import { ThinkingIndicator } from "./ThinkingIndicator";
import { PermissionCard } from "./PermissionCard";

/** Virtualized message list (P-3/P-5). Streaming only mutates the last turn,
 * so finished turns keep their identity and never re-render. */
export function MessageList() {
  const bucket = useAgentStore((state) => state.buckets[state.agent]);
  const vaultId = useAgentStore((state) => state.vaultId);
  const agent = useAgentStore((state) => state.agent);
  const send = useAgentStore((state) => state.send);

  const groups = useMemo<TurnGroup[]>(
    () => groupTurns(groupTranscript(bucket.transcript.lines)),
    [bucket.transcript.lines],
  );

  const parentRef = useRef<HTMLDivElement | null>(null);
  const stickToBottom = useRef(true);
  const virtualizer = useVirtualizer({
    count: groups.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 160,
    overscan: 4,
  });

  // Follow the stream only while the user is already at the bottom; scroll
  // up during a long answer and the view stays where it is.
  const totalSize = virtualizer.getTotalSize();
  const lastLine = bucket.transcript.lines[bucket.transcript.lines.length - 1];
  const lastTextLength = lastLine?.text.length ?? 0;
  useEffect(() => {
    if (!stickToBottom.current || groups.length === 0) return;
    virtualizer.scrollToIndex(groups.length - 1, { align: "end" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groups.length, bucket.transcript.lines.length, lastTextLength, bucket.busy, totalSize]);

  const retry = (text: string): void => {
    if (!vaultId) return;
    void send(text);
  };

  const waiting = bucket.pendingPermissions.length > 0;
  const empty = groups.length === 0 && !bucket.busy && !waiting && !bucket.error;

  return (
    <div
      className="chat-body"
      ref={parentRef}
      data-testid="chat-body"
      onScroll={(event) => {
        const el = event.currentTarget;
        stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
      }}
    >
      <div className="chat-column">
        {empty ? <OverviewGrid /> : null}

        {groups.length > 0 ? (
          <div className="chat-virtual" style={{ height: totalSize }}>
            {virtualizer.getVirtualItems().map((item) => {
              const group = groups[item.index];
              if (!group) return null;
              return (
                <div
                  key={item.key}
                  className="chat-virtual-item"
                  data-index={item.index}
                  ref={virtualizer.measureElement}
                  style={{ transform: `translateY(${item.start}px)` }}
                >
                  <GroupView
                    group={group}
                    agent={agent}
                    index={item.index}
                    total={groups.length}
                    busy={bucket.busy}
                    waiting={waiting}
                    failed={bucket.error !== null}
                    vaultId={vaultId}
                    onRetry={retry}
                  />
                </div>
              );
            })}
          </div>
        ) : null}

        {bucket.pendingPermissions.map((permission) => (
          <PermissionCard key={permission.requestId} agent={agent} permission={permission} />
        ))}

        {bucket.busy ? (
          <ThinkingIndicator
            phase={waiting ? "waiting_permission" : bucket.phase}
            detail={waiting ? undefined : bucket.phaseDetail}
          />
        ) : null}

        {bucket.error ? (
          <div className="status-card" data-tone="fault" role="alert" data-testid="turn-error">
            <div className="status-card-head">
              <Warning size={13} weight="bold" aria-hidden />
              这一轮没有完成
            </div>
            <div className="status-card-body">{bucket.error}</div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function GroupView({
  group,
  agent,
  index,
  total,
  busy,
  waiting,
  failed,
  vaultId,
  onRetry,
}: {
  readonly group: TurnGroup;
  readonly agent: AgentId;
  readonly index: number;
  readonly total: number;
  readonly busy: boolean;
  readonly waiting: boolean;
  readonly failed: boolean;
  readonly vaultId: string | null;
  readonly onRetry: (text: string) => void;
}) {
  if (group.kind === "segment") {
    return <StandaloneSegment segment={group.segment} />;
  }
  const { user, rest } = splitTurn(group.segments);
  const isCurrent = index === total - 1;
  const live = isCurrent && busy;
  const status = turnStatus(group.segments, { isCurrent, busy, waiting, failed });
  const canRetry = isCurrent && !busy && user !== null && vaultId !== null;

  return (
    <div className="chat-turn">
      {user ? <UserMessage line={user} /> : null}
      <AgentMessage
        agent={agent}
        segments={rest}
        status={status}
        live={live}
        userAt={user?.at}
        canRetry={canRetry}
        onRetry={onRetry}
      />
    </div>
  );
}

function StandaloneSegment({ segment }: { readonly segment: TranscriptSegment }) {
  if (segment.kind === "line") {
    if (segment.line.role === "user") return <UserMessage line={segment.line} />;
    return <p className="chat-note">{segment.line.text}</p>;
  }
  return (
    <ol className="steps">
      {segment.steps.map((step) => (
        <li key={step.id} className="step" data-state={step.toolStatus === "completed" ? "done" : undefined}>
          <span className="step-kind">执行</span>
          <span className="step-text">{step.text}</span>
        </li>
      ))}
    </ol>
  );
}
