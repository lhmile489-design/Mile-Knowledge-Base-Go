import { useMutationStore } from "../stores/mutationStore";

const STATUS_TEXT: Record<string, string> = {
  completed: "操作已完成",
  partial: "操作部分完成",
  failed: "操作失败",
};

/** Report after a plan ran (PRD FR-MUT, screenshot 30). */
export function MutationReportCard() {
  const report = useMutationStore((state) => state.report);
  const error = useMutationStore((state) => state.error);
  const dismiss = useMutationStore((state) => state.dismissReport);
  const prepareRestore = useMutationStore((state) => state.prepareRestore);

  if (!report && !error) return null;

  return (
    <div className="surface-card report-card" role="status">
      <div className="status-card-head">
        {error ? "操作未执行" : STATUS_TEXT[report?.status ?? ""] ?? "操作结束"}
      </div>
      {report ? (
        <div className="status-card-body">
          已完成 {report.completed?.length ?? 0} · 失败 {report.failed?.length ?? 0} · 未执行{" "}
          {report.remaining?.length ?? 0}
        </div>
      ) : (
        <div className="status-card-body">{error}</div>
      )}
      {report?.failed?.length ? (
        <ul className="report-failures">
          {report.failed.slice(0, 5).map((record) => (
            <li key={record.id}>
              {record.kind} {record.target || record.source}：{record.errorMessage}
            </li>
          ))}
        </ul>
      ) : null}
      <div className="status-card-actions">
        {report?.transactionId ? (
          <button type="button" className="btn-quiet" onClick={() => void prepareRestore(report.transactionId!)}>
            恢复到操作前
          </button>
        ) : null}
        <button type="button" className="btn-accent" onClick={dismiss}>
          知道了
        </button>
      </div>
    </div>
  );
}
