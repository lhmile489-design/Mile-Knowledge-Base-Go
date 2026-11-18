import { useEffect, useState } from "react";
import { FloppyDisk } from "@phosphor-icons/react";
import { TabStrip } from "./TabStrip";
import { SourceEditor } from "./SourceEditor";
import { ReaderView } from "./ReaderView";
import { ConflictCard } from "./ConflictCard";
import { BacklinksPanel } from "./BacklinksPanel";
import { useAppStore } from "../stores/appStore";
import { useDocStore, isDirty, tabTitle, useDirtyReporter } from "../stores/docStore";
import { formatBytes } from "../lib/bytes";

type Mode = "read" | "source" | "split";

const MODE_LABEL: Record<Mode, string> = { read: "阅读", source: "源码", split: "分栏" };

/** Document panel: tabs, toolbar, reading/source/split modes and the cards
 * around saving (PRD FR-DOC). Reading mode is the default, like the legacy app. */
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
  const dismissConflict = useDocStore((state) => state.dismissConflict);
  const reloadFromDisk = useDocStore((state) => state.reloadFromDisk);

  const [pendingClose, setPendingClose] = useState<string | null>(null);
  const [modes, setModes] = useState<Record<string, Mode>>({});
  useDirtyReporter();

  const doc = activePath ? docs[activePath] : undefined;
  const dirty = isDirty(doc);
  const degraded = doc?.degraded ?? "";
  const readDisabled = degraded !== "";
  const mode: Mode = modes[activePath] ?? "read";
  const effectiveMode: Mode = readDisabled && mode !== "source" ? "source" : mode;
  const editing = effectiveMode !== "read";

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        if (!activePath || !editing) return;
        event.preventDefault();
        void save(vaultId, activePath);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [activePath, editing, save, vaultId]);

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

  const editor = doc ? (
    <SourceEditor
      path={activePath}
      value={doc.text}
      readOnly={degraded === "large-read-only"}
      onChange={(text) => setText(activePath, text)}
      onSave={() => void save(vaultId, activePath)}
    />
  ) : null;

  const reader = doc ? (
    <ReaderView vaultId={vaultId} path={activePath} revision={doc.revision} hash={doc.hash} />
  ) : null;

  return (
    <div className="editor-pane">
      <TabStrip onCloseRequest={requestClose} />

      {doc ? (
        <div className="editor-toolbar">
          <span className="editor-path" title={activePath}>
            {activePath}
          </span>
          <div className="segmented" role="tablist" aria-label="文档模式">
            {(["read", "source", "split"] as Mode[]).map((option) => (
              <button
                key={option}
                type="button"
                role="tab"
                aria-selected={effectiveMode === option}
                className="segmented-item"
                disabled={readDisabled && option !== "source"}
                title={readDisabled && option !== "source" ? "文件过大，阅读模式不可用" : undefined}
                onClick={() => setModes((current) => ({ ...current, [activePath]: option }))}
              >
                {MODE_LABEL[option]}
              </button>
            ))}
          </div>
          <span className="editor-meta">
            {dirty ? "未保存" : "已保存"} · rev {doc.revision} · {doc.eol.toUpperCase()}
            {doc.bom ? " · BOM" : ""} · {formatBytes(doc.size)}
          </span>
          {editing ? (
            <button
              type="button"
              className="btn btn-quiet"
              onClick={() => void save(vaultId, activePath)}
              disabled={doc.loading || degraded === "large-read-only"}
            >
              <FloppyDisk size={14} aria-hidden /> 保存
            </button>
          ) : null}
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

      {degraded ? (
        <div className="surface-card" data-tone="info" role="status">
          <div className="status-card-head">
            {degraded === "large-read-only"
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
          ) : effectiveMode === "read" ? (
            <div className="editor-scroll">{reader}</div>
          ) : effectiveMode === "source" ? (
            editor
          ) : (
            <div className="editor-split">
              <div className="editor-split-left">{editor}</div>
              <div className="editor-split-right editor-scroll">{reader}</div>
            </div>
          )}
        </div>
      ) : null}

      {doc && !doc.loading && !doc.error ? (
        <BacklinksPanel vaultId={vaultId} path={activePath} revision={doc.revision} />
      ) : null}
    </div>
  );
}
