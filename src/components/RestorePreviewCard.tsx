import { useMutationStore } from "../stores/mutationStore";

/** Preview before restoring a transaction (M32); entries changed afterwards
 *  are shown as conflicts and will not be overwritten on confirm. */
export function RestorePreviewCard() {
  const preview = useMutationStore((state) => state.preview);
  const running = useMutationStore((state) => state.running);
  const cancel = useMutationStore((state) => state.cancelRestore);
  const confirm = useMutationStore((state) => state.confirmRestore);

  if (!preview) return null;
  const entries = preview.entries ?? [];
  const blocked = entries.filter((entry) => !entry.restorable).length;

  return (
    <div className="plan-backdrop" role="presentation">
      <section className="plan-card" role="alertdialog" aria-label="恢复确认">
        <header className="plan-head">
          <span className="eyebrow">恢复操作前状态</span>
          <span className="plan-count">{entries.length} 项</span>
        </header>
        {blocked > 0 ? (
          <p className="plan-note" data-tone="warn">
            其中 {blocked} 项在事务之后被外部改动，确认后会被跳过并标为冲突。
          </p>
        ) : null}
        <ul className="plan-list">
          {entries.slice(0, 12).map((entry) => (
            <li key={entry.path} className="plan-item">
              <span className="plan-kind">{entry.restorable ? "可恢复" : "冲突"}</span>
              <span className="plan-path">{entry.path}</span>
              {entry.note ? <span className="plan-more">{entry.note}</span> : null}
            </li>
          ))}
          {entries.length > 12 ? <li className="plan-more">另有 {entries.length - 12} 项</li> : null}
        </ul>
        <div className="plan-actions">
          <button type="button" className="btn-quiet" onClick={cancel} disabled={running}>
            取消
          </button>
          <button type="button" className="btn-accent" onClick={() => void confirm()} disabled={running}>
            {running ? "恢复中…" : "恢复"}
          </button>
        </div>
      </section>
    </div>
  );
}
