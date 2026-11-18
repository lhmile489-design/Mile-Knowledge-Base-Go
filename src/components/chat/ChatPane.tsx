import { useEffect, useState } from "react";
import { useAppStore } from "../../stores/appStore";
import { useAgentStore } from "../../stores/agentStore";
import { useHistoryStore } from "../../stores/historyStore";
import { useComposerStore } from "../../stores/composerStore";
import { agentApi } from "../../lib/agentApi";
import { ChatHeader } from "./ChatHeader";
import { MessageList } from "./MessageList";
import { Composer } from "./Composer";
import { HistoryRail } from "./HistoryRail";
import { ConversationViewer } from "./ConversationViewer";
import { RestoreNote } from "./RestoreNote";
import { SessionInfo } from "./SessionInfo";

/** The Agnes chat panel: header, history rail, messages, composer, viewer. */
export function ChatPane() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const setVault = useAgentStore((state) => state.setVault);
  const restoreNote = useHistoryStore((state) => state.restoreNote);
  const viewerOpen = useHistoryStore((state) => state.viewer !== null);
  const [infoOpen, setInfoOpen] = useState(false);

  useEffect(() => {
    setVault(vaultId);
  }, [vaultId, setVault]);

  useEffect(() => {
    void useHistoryStore.getState().refresh(vaultId);
    void useHistoryStore.getState().restoreOnStart(vaultId);
  }, [vaultId]);

  // E10: the engine phase changed (provider configured, key missing, ...).
  useEffect(() => {
    if (!vaultId) return;
    const off = agentApi.onEngineStatus((status) => {
      useComposerStore.getState().applyEngineStatus(vaultId, status);
    });
    return off;
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
      <ChatHeader infoOpen={infoOpen} onOpenInfo={() => setInfoOpen(true)} />
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
      {infoOpen ? <SessionInfo vaultId={vaultId} onClose={() => setInfoOpen(false)} /> : null}
    </div>
  );
}
