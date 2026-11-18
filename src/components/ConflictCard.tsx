type Props = {
  diskText: string;
  mineText: string;
  onKeepMine: () => void;
  onTakeDisk: () => void;
  onDismiss: () => void;
};

/** Save conflict (PRD FR-DOC): the disk moved on, nothing was written. The
 * three choices mirror the legacy card; "compare" shows both texts first. */
export function ConflictCard({ diskText, mineText, onKeepMine, onTakeDisk, onDismiss }: Props) {
  return (
    <div className="surface-card conflict-card" data-tone="warn" role="alertdialog" aria-label="保存冲突">
      <div className="status-card-head">磁盘上的版本已变化</div>
      <div className="status-card-body">
        保存被拦下了，磁盘文件没有被修改。选择如何处理：保留你的版本覆盖磁盘，或采用磁盘版本。
      </div>
      <details className="conflict-compare">
        <summary>并排比较</summary>
        <div className="conflict-compare-body">
          <div>
            <p className="eyebrow">磁盘</p>
            <pre>{diskText}</pre>
          </div>
          <div>
            <p className="eyebrow">你的编辑</p>
            <pre>{mineText}</pre>
          </div>
        </div>
      </details>
      <div className="status-card-actions">
        <button type="button" className="btn-accent" onClick={onKeepMine}>
          保留我的版本
        </button>
        <button type="button" className="btn-quiet" onClick={onTakeDisk}>
          采用磁盘版本
        </button>
        <button type="button" className="btn-quiet" onClick={onDismiss}>
          稍后处理
        </button>
      </div>
    </div>
  );
}
