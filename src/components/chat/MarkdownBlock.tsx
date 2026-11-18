import { useEffect, useRef, useState } from "react";
import type { MouseEvent } from "react";
import { useAppStore } from "../../stores/appStore";
import { openReference } from "../../lib/openReference";
import { renderApi, windowApi } from "../../lib/transport";

interface MarkdownBlockProps {
  readonly text: string;
  /** Streaming only affects the placeholder the renderer emits; the HTML
   * always comes from the sanitised Go pipeline (M37). */
  readonly streaming?: boolean;
  readonly kind?: "message" | "thought" | "user";
}

/**
 * One rendered answer. The Go side returns sanitised HTML with checkboxes,
 * tables, code blocks and reference buttons already classified; this component
 * only adds the click behaviour: copy a code block, open a reference after a
 * probe, open a vault image, open an external link in the browser.
 */
export function MarkdownBlock({ text, streaming = false, kind = "message" }: MarkdownBlockProps) {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const requestRef = useRef(0);
  const [html, setHtml] = useState("");

  useEffect(() => {
    const requestId = (requestRef.current += 1);
    if (text.trim() === "") {
      setHtml("");
      return;
    }
    renderApi
      .block(text, kind)
      .then((result) => {
        if (requestRef.current === requestId) setHtml(result.html ?? "");
      })
      .catch(() => {
        // Keep the last good render; the next frame retries.
      });
  }, [text, kind]);

  const handleClick = async (event: MouseEvent<HTMLDivElement>): Promise<void> => {
    const target = event.target as HTMLElement | null;
    if (!target) return;

    const copy = target.closest("button.copy[data-code]") as HTMLButtonElement | null;
    if (copy) {
      event.preventDefault();
      try {
        await navigator.clipboard.writeText(copy.dataset.code ?? "");
      } catch {
        return;
      }
      const original = copy.textContent ?? "复制";
      copy.textContent = "已复制";
      window.setTimeout(() => {
        copy.textContent = original;
      }, 2000);
      return;
    }

    const ref = target.closest("button.chat-ref") as HTMLButtonElement | null;
    if (ref && vaultId) {
      const refKind = ref.dataset.refKind === "wiki" ? "wiki" : "path";
      const status = await openReference(vaultId, refKind, ref.dataset.refTarget ?? "");
      ref.dataset.status = status;
      if (status !== "ok" && !ref.querySelector(".chat-ref-note")) {
        const note = document.createElement("span");
        note.className = "chat-ref-note";
        note.textContent = status === "ambiguous" ? "有多个同名" : "找不到";
        ref.appendChild(note);
      }
      return;
    }

    const image = target.closest("button.img-label.vault") as HTMLButtonElement | null;
    if (image && vaultId) {
      const path = image.dataset.vault ?? "";
      if (path !== "") void openReference(vaultId, "path", path);
      return;
    }

    const external = target.closest("a.link-external") as HTMLAnchorElement | null;
    if (external) {
      const url = external.dataset.target ?? "";
      if (url !== "") void windowApi.openExternal(url);
    }
  };

  return (
    <div
      className="answer"
      data-testid="chat-answer"
      data-streaming={streaming ? "true" : undefined}
      onClick={(event) => void handleClick(event)}
      // The HTML is sanitised by the Go renderer before it leaves the process.
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
