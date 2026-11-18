import { useCallback, useEffect, useState } from "react";
import { X } from "@phosphor-icons/react";
import { indexApi, events, type KbHealthReport } from "../lib/transport";
import { useAppStore } from "../stores/appStore";
import { useDocStore } from "../stores/docStore";

type TabId = "unresolvedLinks" | "orphans" | "duplicates" | "untitled";

const TABS: { id: TabId; label: string }[] = [
  { id: "unresolvedLinks", label: "未解析链接" },
  { id: "orphans", label: "孤立文档" },
  { id: "duplicates", label: "同名不同目录" },
  { id: "untitled", label: "缺标题" },
];

/** Knowledge-base health (PRD FR-HLTH, M28). The panel ships in M1 without a
 *  user entry; the overview tile that opens it arrives with M3. */
export function HealthPanel() {
  const open = useAppStore((state) => state.healthOpen);
  const close = useAppStore((state) => state.closeHealth);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const openDoc = useDocStore((state) => state.open);
  const setLayout = useAppStore((state) => state.setLayout);

  const [report, setReport] = useState<KbHealthReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [tab, setTab] = useState<TabId>("unresolvedLinks");

  const load = useCallback(async () => {
    if (!vaultId) return;
    try {
      setError(null);
      setReport(await indexApi.health(vaultId));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [vaultId]);

  useEffect(() => {
    if (!open) return;
    void load();
    const off = events.indexStatus.on((status) => {
      if (status.vaultId === vaultId && status.phase === "ready") void load();
    });
    return () => off();
  }, [open, load, vaultId]);

  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, close]);

  if (!open) return null;

  const go = (path: string) => {
    void openDoc(vaultId, path);
    void setLayout({ docCollapsed: false });
    close();
  };

  const counts: Record<TabId, number> = {
    unresolvedLinks: report?.unresolvedLinks?.length ?? 0,
    orphans: report?.orphans?.length ?? 0,
    duplicates: report?.duplicates?.length ?? 0,
    untitled: report?.untitled?.length ?? 0,
  };

  return (
    <div className="health-backdrop" role="presentation" onClick={close}>
      <section
        className="health-panel"
        role="dialog"
        aria-label="知识库健康"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="health-head">
          <span className="eyebrow">健康</span>
          {report?.partial ? <span className="health-partial">索引未完成</span> : null}
          <button type="button" className="icon-btn health-close" aria-label="关闭健康面板" onClick={close}>
            <X size={14} aria-hidden />
          </button>
        </header>

        <div className="segmented health-tabs" role="tablist" aria-label="健康分类">
          {TABS.map((item) => (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={tab === item.id}
              className="segmented-item"
              onClick={() => setTab(item.id)}
            >
              {item.label} {counts[item.id]}
            </button>
          ))}
        </div>

        <div className="health-body" role="tabpanel">
          {error ? (
            <p className="health-status" role="alert">
              {error}
            </p>
          ) : !report ? (
            <p className="health-status muted">正在读取…</p>
          ) : counts[tab] === 0 ? (
            <p className="health-status muted">这一类没有问题。</p>
          ) : tab === "unresolvedLinks" ? (
            report.unresolvedLinks.map((item) => (
              <div key={item.from + "-" + item.target} className="health-row">
                <button type="button" className="health-link" onClick={() => go(item.from)}>
                  {item.from}
                </button>
                <span className="health-detail">
                  {item.target} · {item.candidates > 1 ? item.candidates + " 个同名目标" : "未找到"}
                </span>
              </div>
            ))
          ) : tab === "orphans" ? (
            report.orphans.map((item) => (
              <button key={item.path} type="button" className="health-row health-link" onClick={() => go(item.path)}>
                {item.title}
                <span className="health-detail">{item.path}</span>
              </button>
            ))
          ) : tab === "duplicates" ? (
            report.duplicates.map((item) => (
              <div key={item.name} className="health-row">
                <span className="health-title">{item.name}</span>
                {item.paths.map((path) => (
                  <button key={path} type="button" className="health-link" onClick={() => go(path)}>
                    {path}
                  </button>
                ))}
              </div>
            ))
          ) : (
            report.untitled.map((item) => (
              <button key={item.path} type="button" className="health-row health-link" onClick={() => go(item.path)}>
                {item.fileName}
                <span className="health-detail">{item.path}</span>
              </button>
            ))
          )}
        </div>

        <footer className="health-foot">
          <span>基于索引版本 {report?.indexRevision ?? 0}</span>
        </footer>
      </section>
    </div>
  );
}
