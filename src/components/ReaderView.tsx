import { useEffect, useRef, useState } from "react";
import { renderApi, TransportError } from "../lib/transport";
import { useDocStore } from "../stores/docStore";
import { useAppStore } from "../stores/appStore";

type Props = {
  vaultId: string;
  path: string;
  revision: number;
  hash: string;
  /** Called right before a resolved in-vault link opens a document, so a host
   * that is not the workbench (the skill drawer) can switch to it first. */
  onBeforeNavigate?: () => void;
};

/** Reading mode: Go renders the sanitised HTML, the page only injects it and
 * delegates clicks (PRD FR-DOC, D-004). Rendered output is cached by revision. */
export function ReaderView({ vaultId, path, revision, hash, onBeforeNavigate }: Props) {
  const cache = useReaderCache();
  const openDoc = useDocStore((state) => state.open);
  const setLayout = useAppStore((state) => state.setLayout);
  const [html, setHtml] = useState("");
  const [error, setError] = useState<string | null>(null);
  const hostRef = useRef<HTMLDivElement | null>(null);
  const key = `${path}@${revision}@${hash}`;

  useEffect(() => {
    let alive = true;
    const cached = cache.get(key);
    if (cached !== undefined) {
      setHtml(cached);
      setError(null);
      return;
    }
    setError(null);
    renderApi
      .document(vaultId, path, revision)
      .then((result) => {
        if (!alive) return;
        cache.set(key, result.html ?? "");
        setHtml(result.html ?? "");
      })
      .catch((err) => {
        if (!alive) return;
        setError(err instanceof TransportError ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [cache, key, path, revision, vaultId]);

  // Event delegation: links, copy buttons. The HTML is already sanitised.
  useEffect(() => {
    const host = hostRef.current;
    if (!host) return;
    const onClick = (event: MouseEvent) => {
      const target = event.target as HTMLElement | null;
      if (!target) return;
      const copy = target.closest<HTMLElement>("[data-act='copy']");
      if (copy) {
        const block = copy.closest<HTMLElement>(".codeblock");
        const code = block?.querySelector("code")?.textContent ?? "";
        void navigator.clipboard.writeText(code);
        return;
      }
      const link = target.closest<HTMLAnchorElement>("a");
      if (!link) return;
      const kind = link.dataset.kind;
      event.preventDefault();
      if (kind === "external") {
        const href = link.getAttribute("href") ?? link.dataset.target ?? "";
        if (href) void import("../lib/transport").then(({ windowApi }) => windowApi.openExternal(href).catch(() => undefined));
        return;
      }
      // Plain links carry data-kind="doc"; wikilinks carry "wiki" plus a
      // status. Both resolve to a vault-relative path in data-target.
      if (kind === "doc" || kind === "wiki") {
        const status = link.dataset.status;
        const target2 = link.dataset.path ?? link.dataset.target;
        if (status === "ok" && target2) {
          onBeforeNavigate?.();
          void openDoc(vaultId, target2).catch(() => undefined);
          void setLayout({ docCollapsed: false });
        }
      }
      // anchor and blocked kinds do nothing, matching the legacy reader.
    };
    host.addEventListener("click", onClick);
    return () => host.removeEventListener("click", onClick);
  }, [html, onBeforeNavigate, openDoc, setLayout, vaultId]);

  if (error) {
    return (
      <div className="surface-card" data-tone="fault" role="alert" style={{ margin: 12 }}>
        <div className="status-card-body">渲染失败：{error}</div>
      </div>
    );
  }

  return (
    <div className="reader" ref={hostRef}>
      <article className="prose" dangerouslySetInnerHTML={{ __html: html }} />
    </div>
  );
}

/** A tiny per-session cache so switching tabs does not re-render the document. */
const htmlCache = new Map<string, string>();
function useReaderCache() {
  return htmlCache;
}
