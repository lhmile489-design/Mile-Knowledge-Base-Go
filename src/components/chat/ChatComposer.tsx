import { useState } from "react";
import type { KeyboardEvent } from "react";
import { ArrowUp } from "@phosphor-icons/react";
import { useAgentStore } from "../../stores/agentStore";

/** Minimal composer (T11). Enter sends, Shift+Enter inserts a newline; the
 * full composer — run parameters, routing line, height persistence — is T13. */
export function ChatComposer() {
  const vaultId = useAgentStore((state) => state.vaultId);
  const busy = useAgentStore((state) => state.buckets[state.agent].busy);
  const send = useAgentStore((state) => state.send);
  const cancel = useAgentStore((state) => state.cancel);
  const [text, setText] = useState("");

  const submit = (): void => {
    if (!vaultId || busy || text.trim() === "") return;
    void send(text);
    setText("");
  };

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>): void => {
    if (event.key !== "Enter" || event.shiftKey) return;
    event.preventDefault();
    submit();
  };

  return (
    <footer className="chat-footer">
      <div className="chat-footer-inner">
        <div className="composer-card chat-composer">
          <textarea
            className="composer-input"
            rows={3}
            value={text}
            placeholder="说明你想做什么，系统会自动选择处理者…"
            aria-label="对话输入"
            onChange={(event) => setText(event.target.value)}
            onKeyDown={onKeyDown}
          />
          <div className="chat-composer-actions">
            {busy ? (
              <button type="button" className="btn line" onClick={() => void cancel()}>
                停止
              </button>
            ) : null}
            <button
              type="button"
              className="composer-send"
              aria-label="发送"
              disabled={busy || !vaultId || text.trim() === ""}
              onClick={submit}
            >
              <ArrowUp size={16} weight="bold" aria-hidden />
            </button>
          </div>
        </div>
      </div>
    </footer>
  );
}
