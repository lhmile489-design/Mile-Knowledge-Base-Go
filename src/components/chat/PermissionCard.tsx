import { useState } from "react";
import { ShieldCheck } from "@phosphor-icons/react";
import type { PermFrame, WritePreview } from "../../mygo";
import { AGENT_META, type AgentId } from "../../lib/agentMeta";
import { useAgentStore } from "../../stores/agentStore";

const TOOL_VERB: Record<string, string> = {
  write_note: "写入",
  edit_note: "修改",
  read_note: "读取",
  search_notes: "检索",
  list_dir: "列出目录",
  handoff_task: "交接任务",
  set_aliases: "修改别名",
};

function PreviewBody({ preview }: { readonly preview: WritePreview }) {
  if (preview.kind === "summary") {
    const label =
      preview.reason === "unchanged"
        ? "内容与磁盘当前版本相同"
        : preview.reason === "not-text"
          ? "目标不是文本文件"
          : "内容过大，无法展示逐行差异";
    return (
      <p className="perm-preview-summary" data-testid="perm-preview-summary">
        {label}
        {preview.newBytes > 0 ? `（新内容 ${preview.newBytes} 字节）` : ""}
      </p>
    );
  }

  const isCreate = preview.kind === "created";
  return (
    <div data-testid="perm-preview">
      <p className="perm-preview-summary">
        {isCreate
          ? `将新建文件，内容 ${preview.newLineCount} 行。`
          : `将覆盖已有文件（旧内容 ${preview.oldLineCount} 行），以下为差异：`}
      </p>
      <div className="perm-preview-lines" data-testid="perm-preview-lines">
        {(preview.lines ?? []).map((line, index) => {
          const sign = line.kind === "added" ? "+" : line.kind === "removed" ? "-" : " ";
          return (
            <div
              key={index}
              className="perm-preview-line"
              data-kind={line.kind}
              data-testid="perm-preview-line"
            >
              <span className="perm-preview-sign" aria-hidden>
                {sign}{" "}
              </span>
              <span className="perm-preview-text">{line.text === "" ? " " : line.text}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/** Approval card (T14): tool verb + target + detail, the diff preview the Go
 * side computed, and allow/reject. The card leaves the list only when Go
 * pushes the approved/rejected/expired frame — never on the IPC reply. */
export function PermissionCard({
  agent,
  permission,
}: {
  readonly agent: AgentId;
  readonly permission: PermFrame;
}) {
  const approve = useAgentStore((state) => state.approve);
  const reject = useAgentStore((state) => state.reject);
  const [submitting, setSubmitting] = useState(false);

  const verb = TOOL_VERB[permission.toolName] ?? permission.toolName;
  const detail = permission.detail;

  const submit = async (action: "approve" | "reject"): Promise<void> => {
    if (submitting) return;
    setSubmitting(true);
    if (action === "approve") await approve(permission.requestId);
    else await reject(permission.requestId);
    // The Go side decides the outcome; a failed call leaves the card until
    // the expired frame arrives, so the spinner simply stops.
    setSubmitting(false);
  };

  return (
    <div
      className="surface-card permission-card"
      data-tone="warn"
      role="group"
      aria-label={`工具审批：${verb}`}
      data-testid="permission-card"
    >
      <div className="perm-head">
        <ShieldCheck size={13} weight="bold" aria-hidden />
        <span>需要批准工具调用 · {AGENT_META[agent].name}</span>
      </div>
      <p className="perm-detail">
        {verb}
        {permission.targetPath ? ` · ${permission.targetPath}` : ""}
        {detail && detail !== permission.toolName ? ` · ${detail}` : ""}
      </p>
      {permission.preview ? <PreviewBody preview={permission.preview} /> : null}
      <div className="perm-actions">
        {permission.options
          .filter((option) => option.id.startsWith("allow"))
          .map((option) => (
            <button
              key={option.id}
              type="button"
              className="btn-primary"
              disabled={submitting}
              onClick={() => void submit("approve")}
            >
              {option.label}
            </button>
          ))}
        <button
          type="button"
          className="btn-quiet"
          disabled={submitting}
          onClick={() => void submit("reject")}
        >
          拒绝
        </button>
      </div>
    </div>
  );
}
