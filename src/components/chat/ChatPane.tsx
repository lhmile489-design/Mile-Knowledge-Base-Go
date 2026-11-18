import { useEffect } from "react";
import { useAppStore } from "../../stores/appStore";
import { useAgentStore } from "../../stores/agentStore";
import { useHistoryStore } from "../../stores/historyStore";
import { agentApi } from "../../lib/agentApi";
import { ChatHeader } from "./ChatHeader";
import { MessageList } from "./MessageList";
import { Composer } from "./Composer";
import { HistoryRail } from "./HistoryRail";
import { ConversationViewer } from "./ConversationViewer";
import { RestoreNote } from "./RestoreNote";

/** The Agnes chat panel: header, history rail, messages, composer, viewer. */
export function ChatPane() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const setVault = useAgentStore((state) => state.setVault);
  const restoreNote = useHistoryStore((state) => state.restoreNote);
  const viewerOpen = useHistoryStore((state) => state.viewer !== null);

  useEffect(() => {
    setVault(vaultId);
  }, [vaultId, setVault]);

  useEffect(() => {
    void useHistoryStore.getState().refresh(vaultId);
    void useHistoryStore.getState().restoreOnStart(vaultId);
  }, [vaultId]);

  // E07: another tab of the app or the same window changed the history.
  useEffect(() => {
    if (!vaultId) return;
    const off = agentApi.onConversationChanged((event) => {
      if (event.vaultId === vaultId) void useHistoryStore.getState().refresh(vaultId);
    });
    return off;
  }, [vaultId]);

  return (
    <div className="chat-pane" data-testid="chat-pane">
      <ChatHeader />
      <div className="chat-body-row">
        <HistoryRail vaultId={vaultId} />
        <div className="chat-main">
          {restoreNote && !viewerOpen ? (
            <RestoreNote
              state={restoreNote}
              onStartNew={() => void useHistoryStore.getState().startNewConversation(vaultId)}
            />
          ) : null}
          <MessageList />
          <Composer />
          {viewerOpen ? <ConversationViewer vaultId={vaultId} /> : null}
        </div>
      </div>
    </div>
  );
}
