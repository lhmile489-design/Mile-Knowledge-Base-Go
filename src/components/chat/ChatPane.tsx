import { useEffect } from "react";
import { useAppStore } from "../../stores/appStore";
import { useAgentStore } from "../../stores/agentStore";
import { ChatHeader } from "./ChatHeader";
import { MessageList } from "./MessageList";
import { ChatComposer } from "./ChatComposer";

/** The Agnes chat panel: header, message list, composer (T11 skeleton). */
export function ChatPane() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const setVault = useAgentStore((state) => state.setVault);

  useEffect(() => {
    setVault(vaultId);
  }, [vaultId, setVault]);

  return (
    <div className="chat-pane" data-testid="chat-pane">
      <ChatHeader />
      <MessageList />
      <ChatComposer />
    </div>
  );
}
