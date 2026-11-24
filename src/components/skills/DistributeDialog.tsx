import { useEffect, useMemo, useState } from "react";
import { Check, FolderSimple, PaperPlaneTilt, WarningCircle } from "@phosphor-icons/react";
import { skillsApi, type KbSkill, type SkillPreviewItem, type SkillTool } from "../../lib/transport";
import { ToolIcon } from "./ToolIcon";
import { installTargets, tailOf } from "../../lib/skillsView";
import { SkillsModal } from "./SkillsModal";

type Mode = "link" | "copy";
type Step = "pick" | "result";

type ResultRow = {
  key: string;
  skill: string;
  target: string;
  how: string;
  verb: string;
  tone: "live" | "idle" | "fault";
  detail?: string;
  ok: boolean;
};

/** 分发弹窗 (M4 T10, M8 T06, M9 T03; UI 稿 12.2 distribute-*): mode, target
 * grid, overwrite confirmation card above the dialog, and the per-target result
 * inside the same dialog. One dialog handles single and batch. 完成 (or any way
 * of closing from the result) hands `onDone(succeeded)` to the page: refresh the
 * scan and, after a batch with at least one success, leave batch selection. */
export function DistributeDialog({
  vaultId,
  skills: openedWith,
  tools,
  onClose,
  onDone,
}: {
  readonly vaultId: string;
  readonly skills: KbSkill[];
  readonly tools: SkillTool[];
  readonly onClose: () => void;
  readonly onDone: (succeeded: boolean) => void;
}) {
  // The scan refreshes behind the dialog (the write itself triggers one); the
  // dialog keeps the skills it was opened with, so the title and the result
  // rows do not change underneath the user.
  const [skills] = useState(openedWith);
  const [mode, setMode] = useState<Mode>("link");
  const [selected, setSelected] = useState<Set<string>>(() => {
    const first = skills[0];
    return new Set((first?.distributed ?? []).map((item) => item.toolId));
  });
  const [overwrite, setOverwrite] = useState<Set<string>>(new Set());
  const [confirming, setConfirming] = useState<string | null>(null);
  const [preview, setPreview] = useState<SkillPreviewItem[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [step, setStep] = useState<Step>("pick");
  const [rows, setRows] = useState<ResultRow[]>([]);

  const targets = useMemo(() => installTargets(tools), [tools]);

  // Preview every target so existing/conflicting cells can be disabled; a
  // batch just executes and reports.
  useEffect(() => {
    if (skills.length !== 1) {
      setPreview(null);
      return;
    }
    let alive = true;
    const inputs = targets.map((tool) => ({ targetId: tool.id, allowOverwrite: overwrite.has(tool.id) }));
    if (inputs.length === 0) return;
    skillsApi
      .distributePreview(vaultId, skills[0]?.path ?? "", inputs, mode)
      .then((value) => {
        if (alive) setPreview(value.items);
      })
      .catch(() => {
        if (alive) setPreview(null);
      });
    return () => {
      alive = false;
    };
  }, [skills, mode, targets, overwrite, vaultId]);

  const toggle = (toolId: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(toolId)) next.delete(toolId);
      else next.add(toolId);
      return next;
    });
  };

  const targetFor = (toolId: string) => preview?.find((item) => item.targetId === toolId);

  const submit = async () => {
    if (selected.size === 0) return;
    setBusy(true);
    const collected: ResultRow[] = [];
    try {
      for (const skill of skills) {
        const inputs = [...selected].map((toolId) => ({ targetId: toolId, allowOverwrite: overwrite.has(toolId) }));
        try {
          const value = await skillsApi.distribute(vaultId, skill.path, inputs, mode);
          for (const item of value.items) {
            const tool = targets.find((candidate) => candidate.id === item.targetId);
            const ok = item.result === "linked" || item.result === "copied";
            collected.push({
              key: `${skill.path}:${item.targetId}`,
              skill: skill.name,
              target: tool?.name ?? item.toolId,
              how: item.result === "linked" ? "联接" : item.result === "copied" ? "拷贝" : "",
              verb: item.result === "linked" ? "已联接" : item.result === "copied" ? "已拷贝" : item.result === "skipped" ? "跳过" : "失败",
              tone: ok ? "live" : item.result === "skipped" ? "idle" : "fault",
              detail: item.detail || undefined,
              ok,
            });
          }
        } catch (error) {
          // The rows written so far stay in the result; later skills are not tried.
          collected.push({
            key: `${skill.path}:error`,
            skill: skill.name,
            target: "",
            how: "",
            verb: "失败",
            tone: "fault",
            detail: error instanceof Error ? error.message : String(error),
            ok: false,
          });
          break;
        }
      }
    } finally {
      setRows(collected);
      setStep("result");
      setBusy(false);
    }
  };

  const succeeded = rows.filter((row) => row.ok).length;
  const skipped = rows.filter((row) => row.tone === "idle").length;
  const failed = rows.filter((row) => row.tone === "fault").length;

  const finish = () => {
    onDone(succeeded > 0);
    onClose();
  };

  const requestOverwrite = (toolId: string) => setConfirming(toolId);
  const confirmingTool = confirming ? targets.find((tool) => tool.id === confirming) : undefined;
  const confirmingItem = confirming ? targetFor(confirming) : undefined;

  const creates = preview?.filter((item) => item.action === "create").length ?? selected.size;
  const overwrites = [...selected].filter((toolId) => overwrite.has(toolId)).length;
  const subject = skills.length === 1 ? `「${skills[0]?.name}」` : ` ${skills.length} 个技能`;
  const title = `分发${subject}`;

  const tags = (
    <div className="skills-chips" data-testid="distribute-skills">
      {skills.map((item) => (
        <span key={item.path} className="chip" title={item.path}>
          <FolderSimple size={12} aria-hidden />
          {item.name}
        </span>
      ))}
    </div>
  );

  const pickBody = (
    <>
      <div className="skills-subview-row">
        <span>方式</span>
        <div className="segmented" role="radiogroup" aria-label="分发方式">
          <button
            type="button"
            role="radio"
            aria-checked={mode === "link"}
            className="segmented-item"
            disabled={busy}
            onClick={() => setMode("link")}
          >
            目录联接（推荐）
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={mode === "copy"}
            className="segmented-item"
            disabled={busy}
            onClick={() => setMode("copy")}
          >
            拷贝
          </button>
        </div>
      </div>

      <div className="skills-targets" data-testid="distribute-targets">
        {targets.map((tool) => {
          const item = targetFor(tool.id);
          const blocked = item?.action === "exists" || item?.action === "conflict";
          const missing = Boolean(tool.missing);
          const checked = selected.has(tool.id);
          const locked = busy || missing || (blocked && !checked);
          const note = missing
            ? "目录不存在"
            : item
              ? item.detail || (item.action === "overwrite" ? "将覆盖" : "")
              : tool.kind === "project"
                ? `写入 ${tool.installRel ?? ""}`
                : tool.kind === "custom"
                  ? tailOf(tool.installDir)
                  : "";
          return (
            <div
              key={tool.id}
              className="skills-target"
              role="checkbox"
              aria-checked={checked}
              aria-disabled={locked}
              tabIndex={0}
              data-testid={`distribute-target-${tool.id}`}
              onClick={() => {
                if (locked) return;
                toggle(tool.id);
              }}
              onKeyDown={(event) => {
                if (event.key !== "Enter" && event.key !== " ") return;
                event.preventDefault();
                if (locked) return;
                toggle(tool.id);
              }}
            >
              <ToolIcon tool={tool} size={18} />
              <span className="skills-target-name">{tool.name}</span>
              {note ? <span className="skills-target-note">{note}</span> : null}
              {blocked ? (
                <label className="skills-target-overwrite" onClick={(event) => event.stopPropagation()}>
                  <input
                    type="checkbox"
                    checked={overwrite.has(tool.id)}
                    disabled={busy}
                    onChange={(event) => {
                      if (event.target.checked) {
                        // Overwriting needs an explicit yes on the card above the dialog.
                        requestOverwrite(tool.id);
                        return;
                      }
                      setOverwrite((current) => {
                        const next = new Set(current);
                        next.delete(tool.id);
                        return next;
                      });
                    }}
                  />
                  覆盖
                </label>
              ) : null}
            </div>
          );
        })}
      </div>

      {busy ? (
        <div className="install-note" data-tone="info" role="status" data-testid="distribute-busy">
          <span className="kb-spin" aria-hidden />
          <span>正在分发，完成前不能关闭。</span>
        </div>
      ) : null}
    </>
  );

  const resultBody = (
    <>
      <div className="skills-dist-rows" data-testid="distribute-result">
        {rows.map((row) => (
          <div key={row.key} className="skills-dist-row" data-tone={row.tone}>
            {row.tone === "fault" ? <WarningCircle size={14} aria-hidden /> : row.tone === "live" ? <Check size={14} aria-hidden /> : <span />}
            <span className="skills-dist-what">{row.target ? `${row.skill} → ${row.target}` : row.skill}</span>
            {row.how ? <span className="skills-dist-how">{row.how}</span> : <span />}
            <span className="pill" data-tone={row.tone}>
              {row.verb}
            </span>
            {row.detail ? <span className="skills-dist-detail">{row.detail}</span> : null}
          </div>
        ))}
      </div>
      {overwrite.size > 0 && succeeded > 0 ? (
        <p className="field-hint">覆盖前的原目录已备份，可在操作记录里恢复。</p>
      ) : null}
    </>
  );

  const overlay = confirmingTool ? (
    <div className="skills-confirm" role="alertdialog" aria-modal="true" aria-label="覆盖确认" data-testid="distribute-overwrite-confirm">
      <h3>
        覆盖 {confirmingTool.name} 里的 {skills[0]?.name}
      </h3>
      <div className="skills-confirm-item">
        <ToolIcon tool={confirmingTool} size={18} />
        <b>{confirmingItem?.targetPath ?? confirmingTool.name}</b>
        <span>目标里已有同名目录，不是这个知识库分发过去的。覆盖前会把它整个备份。</span>
      </div>
      <div className="skills-confirm-item">
        <b>可以恢复</b>
        <span>备份永久保留，可在操作记录里恢复。</span>
      </div>
      <div className="skills-confirm-btns">
        <button type="button" className="btn-quiet" onClick={() => setConfirming(null)} data-testid="distribute-overwrite-keep">
          不覆盖这一处
        </button>
        <button
          type="button"
          className="btn-danger"
          onClick={() => {
            setOverwrite((current) => new Set(current).add(confirmingTool.id));
            setSelected((current) => new Set(current).add(confirmingTool.id));
            setConfirming(null);
          }}
          data-testid="distribute-overwrite-go"
        >
          备份后覆盖
        </button>
      </div>
    </div>
  ) : undefined;

  const footer =
    step === "result" ? (
      <>
        <span data-testid="distribute-summary">
          成功 {succeeded} 处{skipped > 0 ? `，跳过 ${skipped} 处` : ""}
          {failed > 0 ? `，失败 ${failed} 处` : ""}
        </span>
        <span className="spacer" />
        <button type="button" className="btn-primary" onClick={finish} data-testid="distribute-done">
          完成
        </button>
      </>
    ) : (
      <>
        <span data-testid="distribute-summary">
          已选 {selected.size} 个 · 新增 {Math.min(creates, selected.size)} 处{overwrites > 0 ? `，覆盖 ${overwrites} 处` : ""}
        </span>
        <span className="spacer" />
        <button type="button" className="btn-quiet" onClick={onClose} disabled={busy} data-testid="distribute-cancel">
          取消
        </button>
        <button
          type="button"
          className="btn-primary"
          disabled={busy || selected.size === 0}
          onClick={() => void submit()}
          data-testid="distribute-submit"
        >
          {busy ? <span className="kb-spin" aria-hidden /> : <PaperPlaneTilt size={13} aria-hidden />}
          {busy ? "分发中…" : "分发"}
        </button>
      </>
    );

  return (
    <SkillsModal
      title={title}
      step={step === "result" ? "结果" : busy ? "分发中" : "选择方式与目标"}
      busy={busy}
      onClose={step === "result" ? finish : onClose}
      onBack={() => {
        if (!confirming) return false;
        setConfirming(null);
        return true;
      }}
      head={tags}
      footer={footer}
      focusKey={step}
      overlay={overlay}
    >
      <div className="skills-modal-flow" data-testid="distribute-dialog">
        {step === "result" ? resultBody : pickBody}
      </div>
    </SkillsModal>
  );
}
