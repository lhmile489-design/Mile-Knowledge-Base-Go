import { useEffect, useState } from "react";
import { FloppyDisk, Warning } from "@phosphor-icons/react";
import { TabStrip } from "./TabStrip";
import { SourceEditor } from "./SourceEditor";
import { ConflictCard } from "./ConflictCard";
import { useAppStore } from "../stores/appStore";
import { useDocStore, isDirty, tabTitle, useDirtyReporter } from "../stores/docStore";
import { formatBytes } from "../lib/bytes";

/** Document panel: tabs, toolbar, source editor and the cards around saving
 * (PRD FR-DOC). Reading mode and split view arrive with T09. */
export function EditorPane() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const tabs = useDocStore((state) => state.tabs);
  const docs = useDocStore((state) => state.docs);
  const activePath = useDocStore((state) => state.activePath);
  const setText = useDocStore((state) => state.setText);
  const save = useDocStore((state) => state.save);
  const close = useDocStore((state) => state.close);
  const keepMine = useDocStore((state) => state.resolveConflictKeepMine);
  const takeDisk = useDocStore((state) => state.resolveConflictTakeDisk);
  const reloadFromDisk = useDocStore((state) => state.reloadFromDisk);
  const dismissConflict = useDocStore((state) => state.dismissConflict);
  const setActive = useDocStore((state) => state.setActive);

  const [pendingClose, setPendingClose] = useState<string | null>(null);
  useDirtyReporter();

  const doc = activePath ? docs[activePath] : undefined;
  const dirty = isDirty(doc);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        if (!activePath) return;
        event.preventDefault();
        void save(vaultId, activePath);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [activePath, save, vaultId]);

  if (tabs.length === 0) {
    return (
      <div className="panel-placeholder">
        <span className="eyebrow">未打开文件</span>
        <p className="muted">从左侧目录树打开一个文件，它会显示在这里。</p>
      </div>
    );
  }

  const requestClose = (path: string) => {
    const target = docs[path];
    if (isDirty(target)) {
      setPendingClose(path);
      return;
    }
    void close(vaultId, path);
  };

  return (
    <div className="editor-pane">
      <TabStrip onCloseRequest={requestClose} />

      {doc ? (
        <div className="editor-toolbar">
          <span className="editor-path" title={activePath}>
            {activePath}
          </span>
          <span className="editor-meta">
            {dirty ? "未保存" : "已保存"} · rev {doc.revision} · {doc.eol.toUpperCase()}
            {doc.bom ? " · BOM" : ""} · {formatBytes(doc.size)}
          </span>
          <button
            type="button"
            className="btn btn-quiet"
            onClick={() => void save(vaultId, activePath)}
            disabled={doc.loading || doc.degraded === "large-read-only"}
          >
            <FloppyDisk size={14} aria-hidden /> 保存
          </button>
        </div>
      ) : null}

      {pendingClose ? (
        <div className="surface-card" data-tone="warn" role="alertdialog" aria-label="关闭未保存的标签">
          <div className="status-card-head">{tabTitle(pendingClose)} 有未保存的修改</div>
          <div className="status-card-actions">
            <button type="button" className="btn-quiet" onClick={() => setPendingClose(null)}>
              继续编辑
            </button>
            <button
              type="button"
              className="btn-accent"
              onClick={() => {
                void close(vaultId, pendingClose);
                setPendingClose(null);
              }}
            >
              丢弃并关闭
            </button>
          </div>
        </div>
      ) : null}

      {doc?.conflict ? (
        <ConflictCard
          diskText={doc.conflict.diskText}
          mineText={doc.text}
          onKeepMine={() => void keepMine(vaultId, activePath)}
          onTakeDisk={() => takeDisk(activePath)}
          onDismiss={() => dismissConflict(activePath)}
        />
      ) : null}

      {doc?.externalRevision !== undefined && dirty ? (
        <div className="surface-card" data-tone="info" role="status">
          <div className="status-card-head">磁盘上有更新的版本</div>
          <div className="status-card-body">
            你有未保存的修改，所以没有自动替换。保存时会提示冲突；也可以放弃本地修改，直接采用磁盘版本。
          </div>
          <div className="status-card-actions">
            <button type="button" className="btn-quiet" onClick={() => void reloadFromDisk(vaultId, activePath)}>
              采用磁盘版本
            </button>
          </div>
        </div>
      ) : null}

      {doc?.degraded ? (
        <div className="surface-card" data-tone="info" role="status">
          <div className="status-card-head">
            <Warning size={14} aria-hidden />{" "}
            {doc.degraded === "large-read-only"
              ? "文件超过 2 MiB，只能只读预览"
              : "文件超过 1 MiB，阅读模式不可用"}
          </div>
        </div>
      ) : null}

      {doc?.error ? (
        <div className="surface-card" data-tone="fault" role="alert">
          <div className="status-card-body">{doc.error}</div>
        </div>
      ) : null}

      {doc ? (
        <div className="editor-body">
          {doc.loading ? (
            <p className="muted editor-loading">正在读取文件…</p>
          ) : (
            <SourceEditor
              path={activePath}
              value={doc.text}
              readOnly={doc.degraded === "large-read-only"}
              onChange={(text) => setText(activePath, text)}
              onSave={() => void save(vaultId, activePath)}
            />
          )}
        </div>
      ) : null}
    </div>
  );
}
