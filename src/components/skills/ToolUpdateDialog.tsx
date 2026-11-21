import { useEffect, useRef, useState } from "react";
import { FolderOpen, GithubLogo, WarningCircle } from "@phosphor-icons/react";
import {
  TransportError,
  skillsApi,
  type SkillRemote,
  type SkillUpdatePreviewResult,
} from "../../lib/transport";
import { shortSha, sourceOf } from "../../lib/skillRemote";

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  return error instanceof Error ? error.message : String(error);
}

function counts(files: readonly { change: string }[]): string {
  const modified = files.filter((file) => file.change === "modified").length;
  const added = files.filter((file) => file.change === "added").length;
  const removed = files.filter((file) => file.change === "removed").length;
  const parts: string[] = [];
  if (modified > 0) parts.push(`修改 ${modified} 个文件`);
  if (added > 0) parts.push(`新增 ${added} 个`);
  if (removed > 0) parts.push(`删除 ${removed} 个`);
  return parts.length === 0 ? "没有变化。" : `${parts.join("、")}。`;
}

/** 工具技能的更新确认 (M94 → M96; UI 稿 9.4 tools-update): preview the diff and
 * the backup, then replace the directory in place. M96 backs it up first. */
export function ToolUpdateDialog({
  vaultId,
  toolId,
  toolName,
  skillName,
  spec,
  remote,
  onClose,
  onChanged,
  onResult,
}: {
  readonly vaultId: string;
  readonly toolId: string;
  readonly toolName: string;
  readonly skillName: string;
  readonly spec: string;
  readonly remote: SkillRemote;
  readonly onClose: () => void;
  readonly onChanged: () => void;
  readonly onResult?: (lines: string[]) => void;
}) {
  const [preview, setPreview] = useState<SkillUpdatePreviewResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const stageRef = useRef<string | null>(null);
  const appliedRef = useRef(false);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    skillsApi
      .updatePreview(vaultId, { kind: "tool", toolId, path: spec })
      .then((value) => {
        if (!alive) return;
        setPreview(value);
        stageRef.current = value.stageId ?? null;
      })
      .catch((caught) => {
        if (alive) setError(errorText(caught));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
      const stage = stageRef.current;
      if (stage && !appliedRef.current) void skillsApi.discardStage(stage).catch(() => undefined);
    };
  }, [vaultId, toolId, spec]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || busy) return;
      event.stopPropagation();
      onClose();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [busy, onClose]);

  const update = async () => {
    if (!preview?.stageId) return;
    setBusy(true);
    setError(null);
    try {
      const value = await skillsApi.updateTool(preview.stageId);
      appliedRef.current = true;
      const detail = value.items[0]?.detail;
      onResult?.([`已更新 ${toolName} 里的 ${skillName}${detail ? `（${detail}）` : ""}`]);
      onChanged();
      onClose();
    } catch (caught) {
      setError(errorText(caught));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="skills-subview" data-testid="tool-update-dialog">
      <div className="skills-subview-card">
        <header className="skills-subview-head">
          <span className="skills-subview-title">
            更新 {toolName} 里的 {skillName}
          </span>
          <button type="button" className="icon-button" aria-label="取消更新" onClick={onClose} disabled={busy}>
            ×
          </button>
        </header>

        <div className="skills-dialog-body">
          {loading ? <p className="skills-dialog-line">正在读取新版本…</p> : null}

          {error ? (
            <section className="install-note" data-tone="fault" role="alert" data-testid="tool-update-error">
              <WarningCircle size={16} aria-hidden />
              <div className="install-note-body">
                <b>不能更新这个技能</b>
                <p>{error}</p>
              </div>
            </section>
          ) : null}

          {!loading && preview ? (
            <>
              <div className="install-origin">
                <GithubLogo size={15} aria-hidden />
                <b>{sourceOf(remote)}</b>
                <span>{remote.path || "仓库根目录"}</span>
                <code>{shortSha(preview.from)} → {shortSha(preview.to)}</code>
                <span className="install-faint">本地没有改过</span>
              </div>
              <p className="skills-dialog-line">
                {counts(preview.files ?? [])}
                {(preview.files?.length ?? 0) > 0 ? "本地没有改过。" : ""}
              </p>
              <div className="install-origin">
                <FolderOpen size={15} aria-hidden />
                <b>原目录先备份</b>
                <span>整个目录移到备份区，可在操作记录里恢复。</span>
              </div>
              {preview.backupPath ? <code className="update-backup">{preview.backupPath}</code> : null}
            </>
          ) : null}
        </div>

        <footer className="skills-subview-foot">
          <span />
          <div className="skills-subview-actions">
            <button type="button" className="btn-quiet" onClick={onClose} disabled={busy}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={busy || loading || !preview?.stageId || Boolean(error)}
              onClick={() => void update()}
              data-testid="tool-update-submit"
            >
              更新
            </button>
          </div>
        </footer>
      </div>
    </div>
  );
}
