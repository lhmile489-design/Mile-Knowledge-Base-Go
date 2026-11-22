import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft, ArrowClockwise, WarningCircle } from "@phosphor-icons/react";
import { events, type SkillStageProgressEvent } from "../../mygo";
import {
  skillsApi,
  type SkillRestoreItem,
  type SkillRestoreOpItem,
  type SkillRestorePreview,
} from "../../lib/transport";

const STATE_ORDER = ["create", "conflict", "exists", "skip"] as const;

const STATE_LABEL: Record<string, { title: string; tone: "info" | "warn" | "idle" }> = {
  create: { title: "新建", tone: "info" },
  conflict: { title: "冲突", tone: "warn" },
  exists: { title: "已一致", tone: "idle" },
  skip: { title: "跳过", tone: "idle" },
};

function subscribeProgress(listener: (event: SkillStageProgressEvent) => void): () => void {
  try {
    return events.skillStageProgress.on(listener);
  } catch {
    return () => undefined;
  }
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

/** 从清单还原 (M7 T06; UI 稿 9.6, DESIGN.md 11.7): the restore preview, the
 * execution with progress, the result list and the rebuild confirmation. */
export function RestoreView({
  vaultId,
  onBack,
  onChanged,
  onResult,
}: {
  readonly vaultId: string;
  readonly onBack: () => void;
  readonly onChanged: () => void;
  readonly onResult?: (lines: string[]) => void;
}) {
  const [preview, setPreview] = useState<SkillRestorePreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [running, setRunning] = useState(false);
  const [progress, setProgress] = useState<{ phase: string; received: number } | null>(null);
  const [results, setResults] = useState<SkillRestoreOpItem[] | null>(null);
  const [confirmRebuild, setConfirmRebuild] = useState(false);
  const [rebuilding, setRebuilding] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const requestRef = useRef("");

  const reload = async () => {
    try {
      const value = await skillsApi.restorePreview(vaultId);
      setPreview(value.preview ?? null);
      setSelected(
        Object.fromEntries((value.preview?.items ?? []).filter((item) => item.defaultSelected).map((item) => [item.id, true])),
      );
      setError(null);
    } catch (err) {
      setError(messageOf(err));
    }
  };

  useEffect(() => {
    void reload();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vaultId]);

  // Esc: confirmation first, then abort a running restore, then leave.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (confirmRebuild) {
        event.stopPropagation();
        setConfirmRebuild(false);
        return;
      }
      if (running) {
        event.stopPropagation();
        abortRef.current?.abort();
        return;
      }
      event.stopPropagation();
      onBack();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [confirmRebuild, running, onBack]);

  const groups = useMemo(() => {
    const byState = new Map<string, SkillRestoreItem[]>();
    for (const item of preview?.items ?? []) {
      const list = byState.get(item.state) ?? [];
      list.push(item);
      byState.set(item.state, list);
    }
    return STATE_ORDER.filter((state) => (byState.get(state)?.length ?? 0) > 0).map((state) => ({
      state,
      items: byState.get(state) ?? [],
    }));
  }, [preview]);

  const selectable = (item: SkillRestoreItem) => item.state === "create" || item.state === "conflict";
  const chosen = (preview?.items ?? []).filter((item) => selected[item.id] && selectable(item));
  const chosenNetwork = chosen.filter((item) => item.needsNetwork).length;
  const status = preview?.status ?? "missing";
  const unreadable = status === "unreadable" || status === "too-large";
  const futureVersion = status === "version-unknown";

  const toggle = (item: SkillRestoreItem) => {
    if (!selectable(item) || running) return;
    setSelected((current) => ({ ...current, [item.id]: !current[item.id] }));
  };

  const run = async () => {
    if (running || chosen.length === 0) return;
    const controller = new AbortController();
    abortRef.current = controller;
    const requestId = `restore-${Date.now().toString(36)}`;
    requestRef.current = requestId;
    setRunning(true);
    setProgress(null);
    setResults(null);
    const unsubscribe = subscribeProgress((event) => {
      if (event.requestId === requestId) setProgress({ phase: event.phase, received: event.received });
    });
    try {
      const value = await skillsApi.restoreManifest(
        vaultId,
        chosen.map((item) => ({ id: item.id, overwrite: item.state === "conflict" })),
        requestId,
        controller.signal,
      );
      setResults(value.items);
      onChanged();
    } catch (err) {
      setError(messageOf(err));
    } finally {
      unsubscribe();
      abortRef.current = null;
      setRunning(false);
    }
  };

  const rebuild = async () => {
    if (rebuilding) return;
    setRebuilding(true);
    try {
      const value = await skillsApi.rebuildManifest(vaultId);
      const lines = [`已重建分发清单：写入 ${value.written} 项，跳过 ${value.skipped} 项`];
      if (value.corruptCopy) lines.push(`原文件已保留为 ${value.corruptCopy}`);
      onResult?.(lines);
      setConfirmRebuild(false);
      await reload();
      onChanged();
    } catch (err) {
      setError(messageOf(err));
    } finally {
      setRebuilding(false);
    }
  };

  const back = () => {
    if (running) abortRef.current?.abort();
    onBack();
  };

  return (
    <div className="restore-view" data-testid="restore-view">
      <header className="install-head">
        <button type="button" className="icon-button" aria-label="返回" onClick={back} data-testid="restore-back">
          <ArrowLeft size={14} aria-hidden />
        </button>
        <strong>从清单还原</strong>
      </header>

      <div className="restore-body">
          {error ? (
            <p className="install-fault" data-testid="restore-error">
              {error}
            </p>
          ) : null}
          {!preview ? (
            <p className="field-hint" data-testid="restore-loading">
              读取分发清单…
            </p>
          ) : futureVersion ? (
            <div className="restore-note" data-tone="warn" data-testid="restore-status-warn">
              <WarningCircle size={15} aria-hidden />
              <span>分发清单来自更新版本的应用，请升级后再还原。清单不会被改写。</span>
            </div>
          ) : unreadable ? (
            <div className="restore-note" data-tone="warn" data-testid="restore-status-warn">
              <WarningCircle size={15} aria-hidden />
              <span>分发清单无法读取，之后的分发不会写入清单。可以用本机记录重建。</span>
              <span className="spacer" />
              <button type="button" className="btn-quiet" onClick={() => setConfirmRebuild(true)} data-testid="restore-rebuild">
                <ArrowClockwise size={13} aria-hidden />
                用本机记录重建
              </button>
            </div>
          ) : results ? (
            <div className="skills-result" data-testid="restore-result">
              <p>
                已还原 {results.filter((item) => item.result === "linked" || item.result === "copied" || item.result === "installed").length} 项，
                跳过 {results.filter((item) => item.result === "skipped").length} 项，
                失败 {results.filter((item) => item.result === "failed").length} 项
              </p>
              <ul>
                {results.map((item) => (
                  <li key={item.id} data-testid={`restore-result-${item.id}`}>
                    {item.name} → {item.targetLabel}：{RESULT_LABEL[item.result] ?? item.result}
                    {item.detail ? `（${item.detail}）` : ""}
                    {item.backupId ? " · 可从操作记录恢复" : ""}
                  </li>
                ))}
              </ul>
              {(preview.unmatched ?? []).length > 0 ? (
                <p className="field-hint" data-testid="restore-unmatched">
                  {preview.unmatched.length} 项未匹配：{preview.unmatched.join("、")}（不算失败）
                </p>
              ) : null}
            </div>
          ) : groups.length === 0 ? (
            <div className="skills-empty" data-testid="restore-empty">
              <p>清单里没有需要还原的项。</p>
            </div>
          ) : (
            <>
              {running ? (
                <div className="restore-progress" data-testid="restore-progress">
                  <span>
                    {progress ? `正在还原（${progress.phase === "downloading" ? `下载 ${formatBytes(progress.received)}` : "解压"}）…` : "正在还原…"}
                  </span>
                  <div className="install-progress" data-indeterminate="true">
                    <i />
                  </div>
                </div>
              ) : null}
              {groups.map((group) => (
                <section key={group.state} data-testid={`restore-group-${group.state}`}>
                  <div className="skills-group-title">
                    {STATE_LABEL[group.state]?.title ?? group.state}
                    <span className="tool-chip-count">{group.items.length}</span>
                  </div>
                  {group.state === "conflict" ? (
                    <p className="field-hint">勾选的项会先备份原目录再覆盖，可从操作记录恢复。</p>
                  ) : null}
                  {group.items.map((item) => (
                    <div
                      key={item.id}
                      className="restore-row"
                      data-testid={`restore-item-${item.id}`}
                      data-dim={item.state === "exists" || item.state === "skip" ? "1" : undefined}
                    >
                      {selectable(item) ? (
                        <input
                          type="checkbox"
                          className="restore-check"
                          aria-label={`还原 ${item.name}`}
                          checked={Boolean(selected[item.id])}
                          disabled={running}
                          onChange={() => toggle(item)}
                          data-testid={`restore-check-${item.id}`}
                        />
                      ) : (
                        <span aria-hidden />
                      )}
                      <span className="restore-main">
                        <span className="restore-name">
                          {item.name}
                          {item.needsNetwork ? (
                            <span className="pill" data-tone="info" data-testid={`restore-net-${item.id}`}>
                              需要联网
                            </span>
                          ) : null}
                        </span>
                        <span className="restore-target">{item.targetLabel}</span>
                        {item.reason ? <span className="restore-why">{item.reason}</span> : null}
                      </span>
                      <span className="restore-source">
                        {item.source === "kb"
                          ? `知识库 · ${item.mode === "copy" ? "拷贝" : "链接"}`
                          : `GitHub · ${item.origin?.owner ?? ""}/${item.origin?.repo ?? ""}${item.origin?.path ? ` · ${item.origin.path}` : ""}`}
                      </span>
                    </div>
                  ))}
                </section>
              ))}
              {(preview.localOnly ?? []).length > 0 ? (
                <details className="restore-local" data-testid="restore-local">
                  <summary>本机有、清单没有（{preview.localOnly.length}）</summary>
                  <ul>
                    {preview.localOnly.map((entry) => (
                      <li key={`${entry.name}-${entry.targetLabel}`}>
                        {entry.name} → {entry.targetLabel}
                      </li>
                    ))}
                  </ul>
                </details>
              ) : null}
            </>
          )}
      </div>

      {results ? (
        <div className="restore-foot" data-testid="restore-footer">
          <span className="spacer" />
          <button type="button" className="btn-accent" onClick={back} data-testid="restore-done">
            完成
          </button>
        </div>
      ) : preview && !futureVersion && !unreadable && groups.length > 0 ? (
        <div className="restore-foot" data-testid="restore-footer">
          <span className="field-hint" data-testid="restore-count">
            已选 {chosen.length} 项{chosenNetwork > 0 ? `，其中 ${chosenNetwork} 项需要联网` : ""}
          </span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={back} data-testid="restore-cancel">
            取消
          </button>
          <button
            type="button"
            className="btn-accent"
            disabled={running || chosen.length === 0}
            onClick={() => void run()}
            data-testid="restore-run"
          >
            {running ? "还原中…" : "还原所选"}
          </button>
        </div>
      ) : null}

      {confirmRebuild ? (
        <div className="skills-subview" data-testid="restore-rebuild-dialog">
          <div className="skills-subview-card">
            <header className="skills-subview-head">
              <span className="skills-subview-title">用本机记录重建分发清单</span>
            </header>
            <div className="skills-subview-row">
              <span>会使用本机记录生成清单；登记已移除或目录已不存在的项会跳过。</span>
            </div>
            <div className="skills-subview-row">
              <span>原文件会保留为 skill-distributions.json.corrupt-&lt;时间&gt; 副本，确认无误后可自己删除。</span>
            </div>
            <footer className="skills-subview-foot">
              <button type="button" className="btn-quiet" onClick={() => setConfirmRebuild(false)} data-testid="restore-rebuild-cancel">
                取消
              </button>
              <button type="button" className="btn-accent" disabled={rebuilding} onClick={() => void rebuild()} data-testid="restore-rebuild-confirm">
                {rebuilding ? "重建中…" : "重建清单"}
              </button>
            </footer>
          </div>
        </div>
      ) : null}
    </div>
  );
}

const RESULT_LABEL: Record<string, string> = {
  linked: "已链接",
  copied: "已拷贝",
  installed: "已安装",
  skipped: "跳过",
  failed: "失败",
};
