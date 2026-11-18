import { useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ArrowClockwise, ArrowLeft, DownloadSimple } from "@phosphor-icons/react";
import { isAgentId, type AgentId } from "../../lib/agentMeta";
import { linesToTranscript } from "../../lib/conversationView";
import { groupTranscript } from "../../lib/transcriptGroups";
import { groupTurns, splitTurn, turnStatus, type TurnGroup } from "../../lib/turnGroups";
import { useAgentStore } from "../../stores/agentStore";
import { useHistoryStore } from "../../stores/historyStore";
import { AgentMessage } from "./AgentMessage";
import { UserMessage } from "./UserMessage";

/** Read-only viewer (T15): replay one conversation with the same message
 * components, virtualized (P-4), offering 接着问 / 导出 / 返回. */
export function ConversationViewer({ vaultId }: { readonly vaultId: string | null }) {
  const viewer = useHistoryStore((state) => state.viewer);
  const closeViewer = useHistoryStore((state) => state.closeViewer);
  const continueConversation = useHistoryStore((state) => state.continueConversation);
  const exportConversation = useHistoryStore((state) => state.exportConversation);
  const anyBusy = useAgentStore((state) => Object.values(state.buckets).some((bucket) => bucket.busy));
  const [exported, setExported] = useState(false);

  const groups = useMemo<TurnGroup[]>(() => {
    if (!viewer) return [];
    return groupTurns(groupTranscript(linesToTranscript(viewer.lines).lines));
  }, [viewer]);

  const parentRef = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: groups.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 160,
    overscan: 4,
  });

  if (!viewer) return null;

  const lastAgent = viewer.meta.agents[viewer.meta.agents.length - 1];
  const agent: AgentId = lastAgent && isAgentId(lastAgent) ? lastAgent : "general";

  const onExport = async (): Promise<void> => {
    if (!vaultId) return;
    const saved = await exportConversation(vaultId, viewer.id);
    if (!saved) return;
    setExported(true);
    window.setTimeout(() => setExported(false), 2000);
  };

  return (
    <div className="conversation-viewer" data-testid="conversation-viewer">
      <header className="viewer-header">
        <button type="button" className="icon-button" aria-label="返回对话" onClick={closeViewer}>
          <ArrowLeft size={16} aria-hidden />
        </button>
        <span className="viewer-title" data-testid="viewer-title">
          {viewer.meta.title}
        </span>
        <span className="viewer-sub">
          {viewer.meta.turnCount} 轮 · 只读回放
        </span>
        <button type="button" className="btn line" onClick={() => void onExport()} data-testid="viewer-export">
          <DownloadSimple size={13} aria-hidden />
          {exported ? "已导出" : "导出"}
        </button>
        <button
          type="button"
          className="btn-primary"
          disabled={anyBusy || !vaultId}
          title={anyBusy ? "Agent 正在运行，无法切换对话" : undefined}
          data-testid="viewer-continue"
          onClick={() => void continueConversation(vaultId)}
        >
          <ArrowClockwise size={13} aria-hidden />
          接着问
        </button>
      </header>
      <div className="viewer-body" ref={parentRef}>
        <div className="chat-column">
          <div className="chat-virtual" style={{ height: virtualizer.getTotalSize() }}>
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
                  <ViewerGroup group={group} agent={agent} />
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

function ViewerGroup({ group, agent }: { readonly group: TurnGroup; readonly agent: AgentId }) {
  if (group.kind === "segment") {
    if (group.segment.kind === "line" && group.segment.line.role === "user") {
      return <UserMessage line={group.segment.line} />;
    }
    return null;
  }
  const { user, rest } = splitTurn(group.segments);
  const status = turnStatus(group.segments, {
    isCurrent: false,
    busy: false,
    waiting: false,
    failed: false,
  });
  return (
    <div className="chat-turn">
      {user ? <UserMessage line={user} /> : null}
      <AgentMessage
        agent={agent}
        segments={rest}
        status={status}
        live={false}
        userAt={user?.at}
        canRetry={false}
        onRetry={() => undefined}
      />
    </div>
  );
}
