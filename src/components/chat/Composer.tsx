import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent, PointerEvent } from "react";
import { ArrowUp } from "@phosphor-icons/react";
import { useAppStore } from "../../stores/appStore";
import { useAgentStore } from "../../stores/agentStore";
import {
  clampComposerHeight,
  HEIGHT_STEP,
  MAX_PROMPT_CHARS,
  useComposerStore,
} from "../../stores/composerStore";
import { RouteLine } from "./RouteLine";
import { RunParams } from "./RunParams";

/** The composer (T13): Enter sends, Shift+Enter inserts a newline, the height
 * drags (or ↑/↓ by 16px) and persists, and prompts over 32 000 characters are
 * refused instead of truncated. */
export function Composer() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const agent = useAgentStore((state) => state.agent);
  const busy = useAgentStore((state) => state.buckets[state.agent].busy);
  const cancel = useAgentStore((state) => state.cancel);

  const text = useComposerStore((state) => state.text);
  const setText = useComposerStore((state) => state.setText);
  const send = useComposerStore((state) => state.send);
  const height = useComposerStore((state) => state.height);
  const setHeight = useComposerStore((state) => state.setHeight);
  const loadSession = useComposerStore((state) => state.loadSession);
  const storedHeight = useAppStore((state) => state.settings?.layout?.agnesComposerHeight ?? null);

  const [draftHeight, setDraftHeight] = useState<number | null>(null);
  const dragging = useRef<{ startY: number; startHeight: number } | null>(null);

  useEffect(() => {
    if (storedHeight !== null) setHeight(storedHeight, false);
  }, [storedHeight, setHeight]);

  useEffect(() => {
    void loadSession(vaultId);
  }, [vaultId, agent, loadSession]);

  const overLimit = text.length > MAX_PROMPT_CHARS;
  const shownHeight = draftHeight ?? height;

  const submit = (): void => {
    if (busy || overLimit || text.trim() === "") return;
    void send(vaultId);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>): void => {
    if (event.key !== "Enter" || event.shiftKey) return;
    if (event.nativeEvent.isComposing) return;
    event.preventDefault();
    submit();
  };

  const onResizerDown = (event: PointerEvent<HTMLButtonElement>): void => {
    dragging.current = { startY: event.clientY, startHeight: shownHeight };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const onResizerMove = (event: PointerEvent<HTMLButtonElement>): void => {
    const state = dragging.current;
    if (!state) return;
    setDraftHeight(clampComposerHeight(state.startHeight - (event.clientY - state.startY)));
  };

  const onResizerUp = (event: PointerEvent<HTMLButtonElement>): void => {
    if (!dragging.current) return;
    dragging.current = null;
    event.currentTarget.releasePointerCapture(event.pointerId);
    const next = clampComposerHeight(shownHeight);
    setDraftHeight(null);
    setHeight(next, true);
  };

  const onResizerKey = (event: KeyboardEvent<HTMLButtonElement>): void => {
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
    event.preventDefault();
    const delta = event.key === "ArrowUp" ? HEIGHT_STEP : -HEIGHT_STEP;
    setHeight(shownHeight + delta, true);
  };

  return (
    <footer className="chat-footer">
      <div className="chat-footer-inner">
        <RunParams />
        <button
          type="button"
          className="composer-resizer"
          role="separator"
          aria-label="调整输入框高度"
          aria-orientation="horizontal"
          aria-valuenow={shownHeight}
          aria-valuemin={72}
          aria-valuemax={400}
          onPointerDown={onResizerDown}
          onPointerMove={onResizerMove}
          onPointerUp={onResizerUp}
          onKeyDown={onResizerKey}
        />
        <div className="composer-card chat-composer">
          <RouteLine />
          <textarea
            className="composer-input"
            style={{ height: shownHeight }}
            value={text}
            placeholder="说明你想做什么，系统会自动选择处理者…"
            aria-label="对话输入"
            onChange={(event) => setText(event.target.value)}
            onKeyDown={onKeyDown}
          />
          <div className="chat-composer-actions">
            {overLimit ? (
              <span className="chat-counter" data-over="true" role="alert">
                超过 32000 字符，无法发送
              </span>
            ) : text.length > 30_000 ? (
              <span className="chat-counter">
                {text.length}/32000
              </span>
            ) : null}
            {busy ? (
              <button type="button" className="btn line" onClick={() => void cancel()}>
                停止
              </button>
            ) : null}
            <button
              type="button"
              className="composer-send"
              aria-label="发送"
              disabled={busy || !vaultId || overLimit || text.trim() === ""}
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
