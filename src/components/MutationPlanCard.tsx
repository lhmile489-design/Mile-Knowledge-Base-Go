import { useEffect, useState } from "react";
import { Warning } from "@phosphor-icons/react";
import { docApi } from "../lib/transport";
import { previewDiff, type DiffLine } from "../lib/lineDiff";
import { useAppStore } from "../stores/appStore";
import { useMutationStore } from "../stores/mutationStore";

const KIND_LABEL: Record<string, string> = {
  create: "新建文件",
  createDirectory: "新建目录",
  edit: "写入",
  importFile: "导入文件",
  move: "移动 / 重命名",
  copy: "复制",
  archive: "归档",
  trash: "移到回收站",
  permanentDelete: "永久删除",
};

const RISK_LABEL: Record<string, string> = { low: "低风险", medium: "中风险", high: "高风险" };

/** Confirm card for a pending change plan (PRD FR-MUT; screenshot 29/31). */
export function MutationPlanCard() {
  const plan = useMutationStore((state) => state.pending);
  const title = useMutationStore((state) => state.title);
  const running = useMutationStore((state) => state.running);
  const error = useMutationStore((state) => state.error);
  const cancel = useMutationStore((state) => state.cancel);
  const confirm = useMutationStore((state) => state.confirm);

  const [acknowledged, setAcknowledged] = useState(false);
  const [diff, setDiff] = useState<DiffLine[] | "summary" | null>(null);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const editAction = plan?.actions.find((action) => action.kind === "edit" && action.text !== undefined);

  useEffect(() => {
    setAcknowledged(false);
    setDiff(null);
  }, [plan?.id]);

  const showDiff = async () => {
    if (!vaultId || !editAction?.target || editAction.text === undefined) return;
    try {
      const current = await docApi.read(vaultId, editAction.target);
      const lines = previewDiff(editAction.text, current.doc?.text ?? "");
      setDiff(lines ?? "summary");
    } catch {
      setDiff("summary");
    }
  };

  useEffect(() => {
    if (!plan) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !running) cancel();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [plan, running, cancel]);

  if (!plan) return null;

  const destructive = plan.actions.some((action) => action.kind === "permanentDelete");
  const shown = plan.actions.slice(0, 12);
  const hidden = plan.actions.length - shown.length;

  return (
    <div className="plan-backdrop" role="presentation">
      <section className="plan-card" role="alertdialog" aria-label="变更计划确认">
        <header className="plan-head">
          <span className="eyebrow">变更计划</span>
          <span className="plan-risk" data-risk={plan.risk}>
            {RISK_LABEL[plan.risk] ?? plan.risk}
          </span>
          <span className="plan-count">{plan.actions.length} 项</span>
        </header>
        {title ? <p className="plan-title">{title}</p> : null}

        <ul className="plan-list">
          {shown.map((action) => (
            <li key={action.id} className="plan-item">
              <span className="plan-kind">{KIND_LABEL[action.kind] ?? action.kind}</span>
              <span className="plan-path">
                {action.source ? action.source : ""}
                {action.source && action.target ? " → " : ""}
                {action.target ?? ""}
              </span>
            </li>
          ))}
          {hidden > 0 ? <li className="plan-more">另有 {hidden} 项未显示</li> : null}
        </ul>

        {editAction ? (
          <div className="plan-diff">
            <button type="button" className="btn-quiet" onClick={() => void showDiff()} disabled={diff !== null}>
              查看改动
            </button>
            {diff === "summary" ? (
              <p className="plan-note">差异过大，确认后将直接写入，可在操作日志里恢复。</p>
            ) : diff ? (
              <pre className="plan-diff-lines">
                {diff.map((line, index) => (
                  <span key={index} data-kind={line.kind}>
                    {line.kind === "added" ? "+ " : line.kind === "removed" ? "- " : "  "}
                    {line.text}
                    {"\n"}
                  </span>
                ))}
              </pre>
            ) : null}
          </div>
        ) : null}

        <p className="plan-note">
          {destructive
            ? "永久删除无法撤销。执行后可在操作日志里查看记录，但删除本身不提供恢复。"
            : "执行前不会改动任何文件；破坏性操作会写入事务，可在操作日志里恢复。"}
        </p>

        {destructive ? (
          <label className="plan-ack">
            <input
              type="checkbox"
              checked={acknowledged}
              onChange={(event) => setAcknowledged(event.target.checked)}
            />
            <Warning size={13} aria-hidden /> 我了解风险
          </label>
        ) : null}

        {error ? (
          <p className="plan-error" role="alert">
            {error}
          </p>
        ) : null}

        <div className="plan-actions">
          <button type="button" className="btn-quiet" onClick={cancel} disabled={running}>
            取消
          </button>
          <button
            type="button"
            className="btn-accent"
            disabled={running || (destructive && !acknowledged)}
            onClick={() => void confirm(destructive && acknowledged)}
          >
            {running ? "执行中…" : "执行"}
          </button>
        </div>
      </section>
    </div>
  );
}
