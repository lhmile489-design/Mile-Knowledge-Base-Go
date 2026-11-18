import { useCallback, useEffect, useState } from "react";
import { ArrowCounterClockwise, X } from "@phosphor-icons/react";
import {
  events,
  mutationApi,
  type AuditListResult,
  type TransactionSummary,
} from "../lib/transport";
import { useAppStore } from "../stores/appStore";
import { useMutationStore } from "../stores/mutationStore";

const KIND_LABEL: Record<string, string> = {
  create: "新建",
  createDirectory: "新建目录",
  edit: "写入",
  move: "移动",
  copy: "复制",
  archive: "归档",
  trash: "回收站",
  permanentDelete: "永久删除",
};

const STATUS_LABEL: Record<string, string> = {
  planned: "已计划",
  approved: "已批准",
  completed: "已完成",
  failed: "失败",
  reverted: "已恢复",
  conflicted: "冲突",
  skipped: "跳过",
};

function formatTime(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}/${pad(date.getMonth() + 1)}/${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Operation log at the bottom of the document column (PRD FR-MUT, screenshot 32). */
export function AuditPanel() {
  const open = useAppStore((state) => state.auditOpen);
  const toggle = useAppStore((state) => state.toggleAudit);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const report = useMutationStore((state) => state.report);
  const prepareRestore = useMutationStore((state) => state.prepareRestore);

  const [data, setData] = useState<AuditListResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!vaultId) return;
    try {
      setError(null);
      setData(await mutationApi.auditList(vaultId, "", 200));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  }, [vaultId]);

  useEffect(() => {
    if (!open) return;
    void load();
    const off = events.auditChanged.on((event) => {
      if (event.vaultId === vaultId) void load();
    });
    return () => off();
  }, [open, load, vaultId]);

  useEffect(() => {
    if (open && report) void load();
  }, [report, open, load]);

  if (!open) return null;

  const transactions = new Map<string, TransactionSummary>();
  for (const summary of data?.transactions ?? []) transactions.set(summary.id, summary);

  return (
    <section className="audit-panel" aria-label="操作日志">
      <div className="audit-head">
        <span className="eyebrow">操作日志</span>
        <span className="audit-count">LEDGER {data?.records?.length ?? 0}</span>
        <button type="button" className="icon-btn audit-close" aria-label="关闭操作日志" onClick={toggle}>
          <X size={13} aria-hidden />
        </button>
      </div>
      <div className="audit-body">
        {error ? (
          <p className="audit-empty" role="alert">
            {error}
          </p>
        ) : !data || data.records.length === 0 ? (
          <p className="audit-empty muted">还没有记录</p>
        ) : (
          data.records.map((record) => {
            const summary = record.transactionId ? transactions.get(record.transactionId) : undefined;
            const canRestore =
              summary?.status === "committed" && record.status === "completed" && record.transactionId;
            return (
              <div key={record.id} className="audit-row">
                <span className="audit-status" data-status={record.status}>
                  {STATUS_LABEL[record.status] ?? record.status}
                </span>
                <span className="audit-kind">{KIND_LABEL[record.kind] ?? record.kind}</span>
                <span className="audit-path" title={`${record.source ?? ""} ${record.target ?? ""}`}>
                  {record.source ?? ""}
                  {record.source && record.target ? " → " : ""}
                  {record.target ?? ""}
                </span>
                <span className="audit-time">{formatTime(record.completedAt || record.createdAt)}</span>
                {canRestore ? (
                  <button
                    type="button"
                    className="icon-btn audit-restore"
                    aria-label="恢复到操作前"
                    title="恢复到操作前"
                    onClick={() => void prepareRestore(record.transactionId!)}
                  >
                    <ArrowCounterClockwise size={13} aria-hidden />
                  </button>
                ) : null}
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}
