import { useEffect, useState } from "react";
import { skillsApi, type SkillUninstallPreview } from "../../lib/transport";

/** 卸载确认 (M4 T11; UI 稿 uninstall-*): a second-level card that says what
 * will happen — links only lose the link, real directories go to the backup
 * area first, builtin skills never reach here. */
export function UninstallDialog({
  toolId,
  path,
  skillName,
  onClose,
  onDone,
  onResult,
}: {
  readonly toolId: string;
  readonly path: string;
  readonly skillName: string;
  readonly onClose: () => void;
  readonly onDone: () => void;
  readonly onResult?: (lines: string[]) => void;
}) {
  const [preview, setPreview] = useState<SkillUninstallPreview | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    skillsApi
      .uninstallPreview(toolId, path)
      .then((value) => {
        if (alive) setPreview(value.preview);
      })
      .catch((err: unknown) => {
        if (alive) setError(err instanceof Error ? err.message : String(err));
      });
    return () => {
      alive = false;
    };
  }, [toolId, path]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      onClose();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  const confirm = async () => {
    setBusy(true);
    try {
      const value = await skillsApi.uninstall(toolId, path);
      const item = value.items[0];
      const line =
        item?.result === "removed"
          ? item.backupId
            ? `已卸载 ${skillName}：移到备份区（${item.backupId}）`
            : `已卸载 ${skillName}：已删除链接，知识库未改动`
          : `卸载失败：${item?.detail || "未知原因"}`;
      onResult?.([line]);
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="skills-subview" data-testid="uninstall-dialog">
      <div className="skills-subview-card">
        <header className="skills-subview-head">
          <span className="skills-subview-title">卸载「{skillName}」</span>
          <button type="button" className="icon-button" aria-label="返回列表" onClick={onClose}>
            ←
          </button>
        </header>
        <div className="skills-dialog-body">
          {error ? <p className="skills-dialog-line">{error}</p> : null}
          {preview ? (
            <>
              {preview.kind === "link" ? (
                <p className="skills-dialog-line">只删除链接；知识库里的技能不动。</p>
              ) : (
                <p className="skills-dialog-line">整个目录会移到备份区，可从操作记录恢复。</p>
              )}
              {preview.target ? (
                <p className="skills-dialog-line" data-testid="uninstall-target">
                  链接目标：{preview.target}
                </p>
              ) : null}
              {preview.affectedTools && preview.affectedTools.length > 0 ? (
                <p className="skills-dialog-line">
                  会影响 {preview.affectedTools.length} 个已检测到的工具：{preview.affectedTools.join("、")}
                </p>
              ) : null}
            </>
          ) : (
            <p className="skills-dialog-line">读取中…</p>
          )}
        </div>
        <footer className="skills-subview-foot">
          <span />
          <div className="skills-subview-actions">
            <button type="button" className="btn-quiet" onClick={onClose} disabled={busy}>
              取消
            </button>
            <button
              type="button"
              className="btn-danger"
              onClick={() => void confirm()}
              disabled={busy || !preview}
              data-testid="uninstall-confirm"
            >
              {busy ? "卸载中…" : "卸载"}
            </button>
          </div>
        </footer>
      </div>
    </div>
  );
}
