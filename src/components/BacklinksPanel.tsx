import { useEffect, useState } from "react";
import { ArrowBendUpLeft } from "@phosphor-icons/react";
import { indexApi, type LinkGraphResult } from "../lib/transport";
import { useAppStore } from "../stores/appStore";
import { useDocStore } from "../stores/docStore";

type Props = { vaultId: string; path: string; revision: number };

/** Backlinks and unresolved links for the open document (PRD FR-LINK,
 *  screenshot 20). Refetches when the document revision changes. */
export function BacklinksPanel({ vaultId, path, revision }: Props) {
  const [graph, setGraph] = useState<LinkGraphResult | null>(null);
  const openDoc = useDocStore((state) => state.open);
  const setLayout = useAppStore((state) => state.setLayout);

  useEffect(() => {
    let alive = true;
    indexApi
      .links(vaultId, path)
      .then((result) => {
        if (alive) setGraph(result);
      })
      .catch(() => {
        if (alive) setGraph(null);
      });
    return () => {
      alive = false;
    };
  }, [vaultId, path, revision]);

  if (!graph) return null;
  const unresolved = (graph.outgoing ?? []).filter((link) => !link.path);
  if ((graph.backlinks?.length ?? 0) === 0 && unresolved.length === 0) return null;

  const open = (target: string) => {
    void openDoc(vaultId, target);
    void setLayout({ docCollapsed: false });
  };

  return (
    <section className="backlinks" aria-label="反向链接">
      {graph.backlinks.length > 0 ? (
        <div className="backlinks-group">
          <div className="backlinks-head eyebrow">
            <ArrowBendUpLeft size={11} aria-hidden /> 反向链接 {graph.backlinks.length}
          </div>
          {graph.backlinks.map((source) => (
            <button key={source} type="button" className="backlinks-item" onClick={() => open(source)}>
              {source}
            </button>
          ))}
        </div>
      ) : null}

      {unresolved.length > 0 ? (
        <div className="backlinks-group">
          <div className="backlinks-head eyebrow">未解析链接 {unresolved.length}</div>
          {unresolved.map((link) => (
            <span key={link.target} className="backlinks-item is-static">
              {link.target}
              <span className="backlinks-reason">
                {link.candidates > 1 ? `${link.candidates} 个同名目标` : "未找到"}
              </span>
            </span>
          ))}
        </div>
      ) : null}
    </section>
  );
}
