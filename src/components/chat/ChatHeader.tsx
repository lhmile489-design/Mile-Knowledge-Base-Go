import { ChatCircleDots, Info, Plus } from "@phosphor-icons/react";
import { AGENT_META } from "../../lib/agentMeta";
import { useHistoryStore } from "../../stores/historyStore";
import { deriveConversationTitle } from "../../lib/conversationTime";
import { useAgentStore } from "../../stores/agentStore";

/** Chat header, 44px (PRD FR-AI-CHAT). History, session info and the pending
 * approvals tab are wired by T14–T16; the buttons exist so the bar keeps its
 * final shape. */
export function ChatHeader() {
  const agent = useAgentStore((state) => state.agent);
  const bucket = useAgentStore((state) => state.buckets[state.agent]);
  const newConversation = useAgentStore((state) => state.newConversation);
  const vaultId = useAgentStore((state) => state.vaultId);
  const historyOpen = useHistoryStore((state) => state.open);
  const toggleRail = useHistoryStore((state) => state.toggleRail);

  const firstUser = bucket.transcript.lines.find((line) => line.role === "user");
  const title = deriveConversationTitle(firstUser?.text);
  const pending = bucket.pendingPermissions.length;
  const meta = AGENT_META[agent];

  return (
    <header className="chat-header" data-testid="chat-header">
      <button
        type="button"
        className="btn"
        aria-pressed={historyOpen}
        title={historyOpen ? "收起历史栏" : "展开历史栏"}
        data-testid="history-toggle"
        onClick={toggleRail}
      >
        <ChatCircleDots size={14} aria-hidden />
        历史
      </button>
      <span className="chat-title" data-testid="chat-title">
        {title}
      </span>
      {pending > 0 ? (
        <span className="pill" data-tone="warn" data-testid="pending-approvals">
          待审批 {pending}
        </span>
      ) : null}
      <span className="chat-agent" title={`${meta.name} · ${meta.scope}`}>
        {meta.name}
      </span>
      <button type="button" className="icon-button" disabled aria-disabled="true" aria-label="会话信息">
        <Info size={16} aria-hidden />
      </button>
      <button
        type="button"
        className="btn line"
        data-testid="new-conversation"
        disabled={!vaultId}
        onClick={() => void newConversation()}
      >
        <Plus size={14} aria-hidden />
        新对话
      </button>
    </header>
  );
}
