import { useEffect, useMemo, useState } from "react";
import { skillsApi, type KbSkill, type SkillPreviewItem, type SkillTool } from "../../lib/transport";
import { ToolIcon } from "./ToolIcon";

type Mode = "link" | "copy";

/** 分发子视图 (M4 T10; UI 稿 distribute-*): mode, target grid, overwrite,
 * preview hints, per-target results. One dialog handles single and batch. */
export function DistributeDialog({
  vaultId,
  skills,
  tools,
  onClose,
  onDone,
}: {
  readonly vaultId: string;
  readonly skills: KbSkill[];
  readonly tools: SkillTool[];
  readonly onClose: () => void;
  readonly onDone: () => void;
}) {
  const [mode, setMode] = useState<Mode>("link");
  const [selected, setSelected] = useState<Set<string>>(() => {
    const first = skills[0];
    return new Set((first?.distributed ?? []).map((item) => item.toolId));
  });
  const [overwrite, setOverwrite] = useState<Set<string>>(new Set());
  const [preview, setPreview] = useState<SkillPreviewItem[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<string[] | null>(null);

  const targets = useMemo(() => {
    const list = tools.filter(
      (tool) => tool.enabled && (tool.id === "agents" || (tool.status === "detected" && Boolean(tool.installDir))),
    );
    return list.sort((a, b) => (a.id === "agents" ? -1 : b.id === "agents" ? 1 : a.name.localeCompare(b.name)));
  }, [tools]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      onClose();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [onClose]);

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
    const lines: string[] = [];
    try {
      for (const skill of skills) {
        const inputs = [...selected].map((toolId) => ({ targetId: toolId, allowOverwrite: overwrite.has(toolId) }));
        const value = await skillsApi.distribute(vaultId, skill.path, inputs, mode);
        for (const item of value.items) {
          const tool = targets.find((candidate) => candidate.id === item.targetId);
          const verb =
            item.result === "linked"
              ? "已联接"
              : item.result === "copied"
                ? "已拷贝"
                : item.result === "skipped"
                  ? "跳过"
                  : "失败";
          lines.push(`${skill.name} → ${tool?.name ?? item.toolId}：${verb}${item.detail ? `（${item.detail}）` : ""}`);
        }
      }
      onDone();
      setResult(lines);
    } catch (error) {
      setResult([`分发失败：${error instanceof Error ? error.message : String(error)}`]);
    } finally {
      setBusy(false);
    }
  };

  const creates = preview?.filter((item) => item.action === "create").length ?? selected.size;

  return (
    <div className="skills-subview" data-testid="distribute-dialog">
      <div className="skills-subview-card">
        <header className="skills-subview-head">
          <span className="skills-subview-title">
            分发{skills.length === 1 ? `「${skills[0]?.name}」` : ` ${skills.length} 个技能`}
          </span>
          <button type="button" className="icon-button" aria-label="返回列表" onClick={onClose}>
            ←
          </button>
        </header>

        {result ? (
          <div className="skills-result" data-testid="distribute-result">
            <ul>
              {result.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
            <div className="skills-subview-actions">
              <button type="button" className="btn-primary" onClick={onClose} data-testid="distribute-close">
                完成
              </button>
            </div>
          </div>
        ) : (
          <>
            <div className="skills-subview-row">
              <span>方式</span>
              <div className="segmented" role="radiogroup" aria-label="分发方式">
                <button
                  type="button"
                  role="radio"
                  aria-checked={mode === "link"}
                  className="segmented-item"
                  onClick={() => setMode("link")}
                >
                  目录联接（推荐）
                </button>
                <button
                  type="button"
                  role="radio"
                  aria-checked={mode === "copy"}
                  className="segmented-item"
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
                const checked = selected.has(tool.id);
                return (
                  <div
                    key={tool.id}
                    className="skills-target"
                    role="checkbox"
                    aria-checked={checked}
                    aria-disabled={blocked && !checked}
                    tabIndex={0}
                    data-testid={`distribute-target-${tool.id}`}
                    onClick={() => {
                      if (blocked && !checked) return;
                      toggle(tool.id);
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      if (blocked && !checked) return;
                      toggle(tool.id);
                    }}
                  >
                    <ToolIcon tool={tool} size={18} />
                    <span className="skills-target-name">{tool.name}</span>
                    {item ? <span className="skills-target-note">{item.detail || (item.action === "overwrite" ? "将覆盖" : "")}</span> : null}
                    {blocked ? (
                      <label className="skills-target-overwrite" onClick={(event) => event.stopPropagation()}>
                        <input
                          type="checkbox"
                          checked={overwrite.has(tool.id)}
                          onChange={(event) => {
                            setOverwrite((current) => {
                              const next = new Set(current);
                              if (event.target.checked) next.add(tool.id);
                              else next.delete(tool.id);
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

            <footer className="skills-subview-foot">
              <span data-testid="distribute-summary">
                已选 {selected.size} 个 · 新增 {Math.min(creates, selected.size)} 处
              </span>
              <div className="skills-subview-actions">
                <button type="button" className="btn-quiet" onClick={onClose} disabled={busy}>
                  取消
                </button>
                <button type="button" className="btn-primary" disabled={busy || selected.size === 0} onClick={() => void submit()} data-testid="distribute-submit">
                  {busy ? "分发中…" : "分发"}
                </button>
              </div>
            </footer>
          </>
        )}
      </div>
    </div>
  );
}
