import { useEffect, useState } from "react";
import { skillsApi, TransportError, type SkillBackup } from "../../lib/transport";
import { SettingRow } from "./SettingRow";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Settings → 关于与诊断 → 维护 → 技能备份 (技术设计 15.6, Q-SKM-3): backups
 * are kept forever; this row lists them and deletes the ones the user picks. */
export function SkillBackupsRow() {
  const [items, setItems] = useState<SkillBackup[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  const load = () => {
    skillsApi
      .backups()
      .then((value) => {
        setItems(value.items);
        setError(null);
      })
      .catch((err) => setError(err instanceof TransportError ? err.message : String(err)));
  };

  useEffect(load, []);

  const total = (items ?? []).reduce((sum, item) => sum + item.size, 0);
  const count = items?.length ?? 0;

  const toggle = (id: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const clean = async () => {
    if (selected.size === 0) return;
    setBusy(true);
    try {
      const value = await skillsApi.cleanBackups([...selected]);
      setItems(value.items);
      setSelected(new Set());
      setError(null);
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <SettingRow
        label="技能备份"
        help="卸载与覆盖前的原目录，永久保留；在这里勾选删除。删除后对应记录不能再恢复。"
        testid="skill-backups-row"
        control={
          <span className="settings-storage">
            <span className="settings-widths" data-testid="skill-backups-count">
              {count} 个 · {formatSize(total)}
            </span>
            <button
              type="button"
              className="btn-quiet"
              onClick={() => {
                setOpen(true);
                setSelected(new Set());
                load();
              }}
              data-testid="skill-backups-manage"
            >
              管理…
            </button>
          </span>
        }
      />
      {error ? (
        <p className="settings-error" role="alert">
          {error}
        </p>
      ) : null}

      {open ? (
        <div className="settings-dialog-backdrop" role="presentation">
          <div className="settings-dialog" role="dialog" aria-modal="true" aria-labelledby="skill-backups-title" data-testid="skill-backups-dialog">
            <div className="settings-dialog-head">
              <strong id="skill-backups-title">技能备份</strong>
            </div>
            {count === 0 ? (
              <p className="field-hint">还没有备份。</p>
            ) : (
              <ul className="backup-list">
                {(items ?? []).map((item) => (
                  <li key={item.id} className="backup-row">
                    <label className="backup-check">
                      <input
                        type="checkbox"
                        checked={selected.has(item.id)}
                        onChange={() => toggle(item.id)}
                        data-testid={`backup-row-${item.id}`}
                      />
                      <span className="backup-kind">{item.kind}</span>
                      <span className="backup-path" title={item.originalPath}>
                        {item.originalPath}
                      </span>
                      <span className="backup-meta">
                        {formatSize(item.size)} · {item.createdAt}
                      </span>
                    </label>
                  </li>
                ))}
              </ul>
            )}
            <div className="settings-dialog-actions">
              <button type="button" className="btn-quiet" onClick={() => setOpen(false)} disabled={busy}>
                关闭
              </button>
              <button
                type="button"
                className="btn-danger"
                disabled={busy || selected.size === 0}
                onClick={() => void clean()}
                data-testid="skill-backups-clean"
              >
                {busy ? "删除中…" : `删除所选（${selected.size}）`}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </>
  );
}
