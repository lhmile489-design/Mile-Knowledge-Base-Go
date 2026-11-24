import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Archive, FileZip, FolderOpen, GithubLogo, WarningCircle } from "@phosphor-icons/react";
import { events, type SkillStageProgressEvent } from "../../mygo";
import {
  TransportError,
  skillsApi,
  type SkillLogRecord,
  type SkillPreviewItem,
  type SkillStage,
  type SkillTool,
} from "../../lib/transport";
import { categoryProblem } from "../../lib/skillImport";
import { installTargets, tailOf } from "../../lib/skillsView";
import { SkillImportWizard } from "../SkillImportWizard";
import { SkillsModal } from "./SkillsModal";
import { ToolIcon } from "./ToolIcon";

const OP_LABEL: Record<string, string> = {
  distribute: "分发",
  overwrite: "覆盖",
  uninstall: "卸载",
  adopt: "收编",
  restore: "恢复",
  clean: "清理",
  install: "安装",
  update: "更新",
};

const CANDIDATE_STATE: Record<string, { text: string; tone: "idle" | "warn" }> = {
  exists: { text: "已存在", tone: "idle" },
  "too-large": { text: "过大", tone: "warn" },
  duplicate: { text: "重复", tone: "idle" },
  "invalid-name": { text: "名字不合法", tone: "warn" },
};

const PHASE_LABEL: Record<string, string> = {
  resolving: "解析仓库",
  downloading: "下载中",
  unpacking: "解压中",
  scanning: "扫描中",
};

const ERROR_HINT: Record<string, string> = {
  "unsafe-archive": "压缩包里有不安全的路径，整包已拒绝。知识库、工具目录和临时区都没有写入任何文件。",
  "too-large": "超过了 200 MiB 上限，整包已拒绝，没有写入任何文件。",
  "network-failure": "连不上 GitHub，检查网络或代理后重试。",
  "rate-limited": "GitHub 匿名额度已用完，稍后再试。",
  "not-found": "仓库或路径不存在；私有仓库在未登录时也显示为不存在。",
};

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function messageOf(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function failureOf(error: unknown): { code: string; message: string; detail?: string; size?: number; limit?: number } {
  if (error instanceof TransportError) {
    return { code: error.code, message: error.message, detail: error.detail };
  }
  return { code: "unknown", message: messageOf(error) };
}

type Reading = { source: "github" | "zip"; label: string };
type ErrorCard = { title: string; code: string; message: string; detail?: string; source: "github" | "zip" | "install" };

/** E13 is wired by the desktop runtime; unit tests run without it, so a
 * missing bridge degrades to no progress instead of crashing the view. */
function subscribeProgress(listener: (event: SkillStageProgressEvent) => void): () => void {
  try {
    return events.skillStageProgress.on(listener);
  } catch {
    return () => undefined;
  }
}

type Source = "github" | "zip" | "local";

const SOURCE_TABS: { id: Source; label: string; testId: string; icon: ReactNode }[] = [
  { id: "github", label: "GitHub", testId: "install-github", icon: <GithubLogo size={13} aria-hidden /> },
  { id: "zip", label: "zip 文件", testId: "install-zip", icon: <FileZip size={13} aria-hidden /> },
  { id: "local", label: "本地文件夹", testId: "install-local", icon: <FolderOpen size={13} aria-hidden /> },
];

type Finished = { lines: string[]; summary: string; hint: string };

/** 安装弹窗 (M5 T10 content, M9 T02 shell; UI 稿 12.2): one dialog for the
 * three sources, the candidate step with a vault or direct-tool target, and a
 * result step. Esc steps back (read → source, candidates → source) and closes
 * on the source step; while an install or import writes, nothing can close it
 * (PRD Q-UX-6). The operation records stay at the bottom of the source step so
 * a backup can still be restored from there. */
export function InstallView({
  vaultId,
  tools,
  onBack,
  onChanged,
}: {
  readonly vaultId: string;
  readonly tools: SkillTool[];
  readonly onBack: () => void;
  readonly onChanged: () => void;
}) {
  const [records, setRecords] = useState<SkillLogRecord[] | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [wizardBusy, setWizardBusy] = useState(false);

  const [source, setSource] = useState<Source>("github");
  const [input, setInput] = useState("");
  const [inputProblem, setInputProblem] = useState<string | null>(null);
  const [reading, setReading] = useState<Reading | null>(null);
  const [progress, setProgress] = useState<{ phase: string; received: number; total: number } | null>(null);
  const [error, setError] = useState<ErrorCard | null>(null);
  const [stage, setStage] = useState<SkillStage | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [targetKind, setTargetKind] = useState<"kb" | "tools">("kb");
  const [category, setCategory] = useState("");
  const [toolTargets, setToolTargets] = useState<Set<string>>(new Set());
  const [overwrite, setOverwrite] = useState<Set<string>>(new Set());
  const [preview, setPreview] = useState<SkillPreviewItem[] | null>(null);
  const [resultLines, setResultLines] = useState<string[] | null>(null);
  const [finished, setFinished] = useState<Finished | null>(null);
  const abortRef = useRef<AbortController | null>(null);
  const stageRef = useRef<SkillStage | null>(null);
  stageRef.current = stage;

  const load = async () => {
    try {
      const value = await skillsApi.log(30);
      setRecords(value.records);
    } catch {
      setRecords([]);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  // Closing the dialog cancels a GitHub read and drops a staged download the
  // user never installed.
  useEffect(
    () => () => {
      abortRef.current?.abort();
      const held = stageRef.current;
      if (held) void skillsApi.discardStage(held.stageId).catch(() => undefined);
    },
    [],
  );

  // E13: one frame per 200 ms with the download's byte count.
  useEffect(() => {
    if (reading?.source !== "github") return;
    return subscribeProgress((event) => {
      setProgress({ phase: event.phase, received: event.received, total: event.total ?? 0 });
    });
  }, [reading]);

  const eligibleTools = useMemo(() => installTargets(tools), [tools]);

  // Tool targets: preview tells which tiles exist (and need 覆盖).
  useEffect(() => {
    if (!stage || targetKind !== "tools" || selected.size === 0 || eligibleTools.length === 0) {
      setPreview(null);
      return;
    }
    let alive = true;
    const target = {
      kind: "tools" as const,
      targets: eligibleTools.map((tool) => ({ targetId: tool.id, allowOverwrite: overwrite.has(tool.id) })),
    };
    skillsApi
      .installPreview(vaultId, stage.stageId, [...selected], target)
      .then((value) => {
        if (alive) setPreview(value.items);
      })
      .catch(() => {
        if (alive) setPreview(null);
      });
    return () => {
      alive = false;
    };
  }, [stage, targetKind, selected, eligibleTools, overwrite, vaultId]);

  const restore = async (record: SkillLogRecord) => {
    if (!record.backupId) return;
    setBusy(true);
    setNotice(null);
    try {
      await skillsApi.restore(record.backupId);
      setNotice(`已恢复 ${record.skill || "技能"}`);
      await load();
      onChanged();
    } catch (caught) {
      setNotice(messageOf(caught));
    } finally {
      setBusy(false);
    }
  };

  const resetStage = () => {
    setStage(null);
    setSelected(new Set());
    setPreview(null);
    setTargetKind("kb");
    setCategory("");
    setToolTargets(new Set());
    setOverwrite(new Set());
  };

  // Back to the source step; the chosen source tab stays.
  const leaveStage = () => {
    if (stage) void skillsApi.discardStage(stage.stageId).catch(() => undefined);
    resetStage();
  };

  const selectSource = (next: Source) => {
    if (stage) leaveStage();
    setResultLines(null);
    setError(null);
    setInputProblem(null);
    setNotice(null);
    setSource(next);
  };

  // 继续安装 (Q-M9-2): same source type, address and candidates cleared.
  const installAnother = () => {
    setFinished(null);
    setResultLines(null);
    setInput("");
    setInputProblem(null);
    setError(null);
    setNotice(null);
  };

  const enterStage = (value: SkillStage) => {
    setStage(value);
    setSelected(new Set(value.candidates.filter((candidate) => candidate.state === "ok").map((candidate) => candidate.id)));
    setPreview(null);
    setTargetKind("kb");
    setCategory("");
    setToolTargets(new Set());
    setOverwrite(new Set());
    setError(null);
    setResultLines(null);
  };

  const readGithub = async () => {
    const address = input.trim();
    if (!address) {
      setInputProblem("请粘贴 GitHub 地址或 owner/repo");
      return;
    }
    setInputProblem(null);
    setError(null);
    setProgress(null);
    setReading({ source: "github", label: address });
    const controller = new AbortController();
    abortRef.current = controller;
    try {
      const value = await skillsApi.readGithub(vaultId, address, controller.signal);
      if (value.cancelled) {
        setNotice("已取消读取");
        return;
      }
      if (value.stage) enterStage(value.stage);
    } catch (caught) {
      if (controller.signal.aborted) {
        setNotice("已取消读取");
        return;
      }
      const failure = failureOf(caught);
      if (failure.code === "invalid-input") {
        setInputProblem(failure.message);
        return;
      }
      setError({ title: "读取失败", code: failure.code, message: failure.message, detail: failure.detail, source: "github" });
    } finally {
      abortRef.current = null;
      setReading(null);
      setProgress(null);
    }
  };

  const pickZip = async () => {
    setError(null);
    setNotice(null);
    setReading({ source: "zip", label: "zip 文件" });
    try {
      const value = await skillsApi.pickZip(vaultId);
      if (value.cancelled) return;
      if (value.stage) enterStage(value.stage);
    } catch (caught) {
      const failure = failureOf(caught);
      setError({ title: "读取失败", code: failure.code, message: failure.message, detail: failure.detail, source: "zip" });
    } finally {
      setReading(null);
    }
  };

  const install = async () => {
    if (!stage || selected.size === 0) return;
    let target;
    if (targetKind === "kb") {
      const problem = categoryProblem(category);
      if (problem) {
        setError({ title: "分类名不可用", code: "invalid-input", message: problem, source: "install" });
        return;
      }
      target = { kind: "kb" as const, category: category.trim() };
    } else {
      if (toolTargets.size === 0) return;
      target = {
        kind: "tools" as const,
        targets: [...toolTargets].map((toolId) => ({ targetId: toolId, allowOverwrite: overwrite.has(toolId) })),
      };
    }
    setBusy(true);
    setError(null);
    try {
      const value = await skillsApi.install(vaultId, stage.stageId, [...selected], target);
      const lines = value.items.map((item) => {
        const candidate = stage.candidates.find((entry) => entry.id === item.targetId);
        const tool = tools.find((entry) => entry.id === item.toolId);
        const verb =
          item.result === "copied" || item.result === "linked"
            ? "已安装"
            : item.result === "skipped"
              ? "跳过"
              : "失败";
        const where = item.toolId === "kb" ? "知识库" : (tool?.name ?? item.toolId);
        const label = candidate?.name ?? item.name ?? item.targetId;
        return `${label} → ${where}：${verb}${item.detail ? `（${item.detail}）` : ""}`;
      });
      onChanged();
      const installed = value.items.filter((item) => item.result === "copied" || item.result === "linked").length;
      if (installed > 0) {
        const skipped = value.items.filter((item) => item.result === "skipped").length;
        const failed = value.items.length - installed - skipped;
        const parts = [`已安装 ${installed} 个`];
        if (skipped > 0) parts.push(`跳过 ${skipped} 个`);
        if (failed > 0) parts.push(`失败 ${failed} 个`);
        const hint =
          targetKind === "kb" ? `已装进知识库「${category.trim()}」分类。` : "已拷贝到所选工具，不经过知识库。";
        resetStage();
        setResultLines(null);
        setFinished({ lines, summary: parts.join("，"), hint });
        void load();
      } else {
        // Nothing written: stay on the candidates so the user can adjust.
        setResultLines(lines);
      }
    } catch (caught) {
      const failure = failureOf(caught);
      setError({ title: "安装失败", code: failure.code, message: failure.message, detail: failure.detail, source: "install" });
    } finally {
      setBusy(false);
    }
  };

  const toggle = (set: Set<string>, id: string) => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  };

  const modalBusy = busy || wizardBusy || reading?.source === "zip";

  // Esc: cancel a GitHub read, then candidates → source; otherwise let the
  // dialog close.
  const handleBack = (): boolean => {
    if (reading?.source === "github") {
      abortRef.current?.abort();
      return true;
    }
    if (stage) {
      leaveStage();
      setResultLines(null);
      return true;
    }
    return false;
  };

  const categoryIssue = targetKind === "kb" ? categoryProblem(category) : null;
  const canSubmit =
    Boolean(stage) && selected.size > 0 && !busy && (targetKind === "kb" ? categoryIssue === null : toolTargets.size > 0);
  const percent = progress && progress.total > 0 ? Math.max(0, Math.min(100, Math.round((progress.received * 100) / progress.total))) : null;

  const sourceLocked = modalBusy || reading !== null;
  const tabs = (
    <div className="segmented" role="tablist" aria-label="安装来源">
      {SOURCE_TABS.map((tab) => (
        <button
          key={tab.id}
          type="button"
          role="tab"
          aria-selected={source === tab.id}
          className="segmented-item"
          disabled={sourceLocked}
          onClick={() => selectSource(tab.id)}
          data-testid={tab.testId}
        >
          {tab.icon}
          {tab.label}
        </button>
      ))}
    </div>
  );

  let stepLabel = "选择来源";
  let body: ReactNode;
  let footer: ReactNode = null;
  let focusKey = `source-${source}`;

  if (finished) {
    stepLabel = "结果";
    focusKey = "result";
    body = (
      <div className="skills-modal-flow" data-testid="install-finished">
        <div className="install-note" data-tone="ok" data-testid="install-result">
          <ul>
            {finished.lines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </div>
        <p className="field-hint">{finished.hint}</p>
      </div>
    );
    footer = (
      <>
        <span data-testid="install-summary">{finished.summary}</span>
        <span className="spacer" />
        <button type="button" className="btn-quiet" onClick={installAnother} data-testid="install-again">
          继续安装
        </button>
        <button type="button" className="btn-primary" onClick={onBack} data-testid="install-done">
          完成
        </button>
      </>
    );
  } else if (stage) {
    stepLabel = busy ? "安装中" : "选择要安装的技能";
    focusKey = "stage";
    const eligible = stage.candidates.filter((candidate) => candidate.state === "ok").length;
    const skippedLinks = (stage.skipped ?? []).filter((item) => item.reason === "symlink" || item.reason === "special").length;
    const capped = (stage.skipped ?? []).some((item) => item.reason === "capped");
    const targets = eligibleTools;
    const creates = preview ? preview.filter((item) => item.action === "create").length : selected.size * toolTargets.size;

    body = (
      <div className="skills-modal-flow" data-testid="install-stage">
        {busy ? (
          <div className="install-note" data-tone="info" role="status" data-testid="install-busy">
            <span className="kb-spin" aria-hidden />
            <span>正在安装，完成前不能关闭。</span>
          </div>
        ) : null}

        <div className="install-origin" data-testid="install-origin">
          {stage.kind === "github" ? <GithubLogo size={16} aria-hidden /> : <FileZip size={16} aria-hidden />}
          <b>{stage.origin ? `${stage.origin.owner}/${stage.origin.repo}` : stage.label}</b>
          {stage.origin ? (
            <>
              <span>分支 {stage.origin.ref}</span>
              <code>提交 {stage.origin.commit.slice(0, 7)}</code>
            </>
          ) : null}
          {targetKind === "tools" && selected.size > 0 ? (
            <span className="install-faint">
              已选 {stage.candidates.filter((candidate) => selected.has(candidate.id)).map((candidate) => candidate.name).join("、")}
            </span>
          ) : (
            <span className="install-faint">找到 {stage.candidates.length} 个技能</span>
          )}
        </div>

        {skippedLinks > 0 || capped ? (
          <p className="field-hint" data-testid="install-skipped">
            {skippedLinks > 0 ? `跳过 ${skippedLinks} 个链接或特殊文件。` : ""}
            {capped ? "候选超过 50 个，其余未列出。" : ""}
          </p>
        ) : null}

        <div className="install-label">
          选择要安装的技能
          <span className="install-faint">向下找 3 层，跳过点号目录与 node_modules</span>
          <span className="spacer" />
          <button
            type="button"
            className="btn-quiet"
            disabled={busy || eligible === 0}
            onClick={() =>
              setSelected(new Set(stage.candidates.filter((candidate) => candidate.state === "ok").map((candidate) => candidate.id)))
            }
            data-testid="install-select-all"
          >
            全选可装的
          </button>
        </div>
        {stage.candidates.length === 0 ? (
          <p className="skills-empty">没有找到含 SKILL.md 的目录（向下找了 3 层）。</p>
        ) : (
          <ul className="install-candidates" data-testid="install-candidates">
            {stage.candidates.map((candidate) => {
              const state = CANDIDATE_STATE[candidate.state];
              const usable = candidate.state === "ok";
              return (
                <li key={candidate.id} className="install-candidate" data-state={candidate.state}>
                  <label className="install-candidate-check">
                    <input
                      type="checkbox"
                      checked={selected.has(candidate.id)}
                      disabled={!usable || busy}
                      onChange={() => setSelected((current) => toggle(current, candidate.id))}
                      data-testid={`install-candidate-${candidate.id}`}
                    />
                    <span className="install-candidate-name">{candidate.name}</span>
                  </label>
                  <span className="install-candidate-path" title={candidate.id}>
                    {candidate.id || "仓库根目录"}
                  </span>
                  <span className="install-candidate-desc">{candidate.description || "—"}</span>
                  <span className="install-candidate-size">
                    {formatBytes(candidate.bytes)} · {candidate.files} 个文件
                  </span>
                  {state ? <span className="pill" data-tone={state.tone}>{state.text}</span> : <span />}
                </li>
              );
            })}
          </ul>
        )}

        <div className="install-label">装到哪里</div>
        <div className="segmented" role="radiogroup" aria-label="安装位置">
          <button
            type="button"
            role="radio"
            aria-checked={targetKind === "kb"}
            className="segmented-item"
            disabled={busy}
            data-testid="install-target-kb"
            onClick={() => setTargetKind("kb")}
          >
            装进知识库
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={targetKind === "tools"}
            className="segmented-item"
            disabled={busy}
            data-testid="install-target-tools"
            onClick={() => setTargetKind("tools")}
          >
            直接装进工具
          </button>
        </div>

        {targetKind === "kb" ? (
          <div className="install-category">
            <input
              className="field install-category-input"
              value={category}
              placeholder="分类，例如 文档"
              disabled={busy}
              onChange={(event) => setCategory(event.target.value)}
              data-testid="install-category"
            />
            <span className="field-hint">
              将复制到 知识库/skills/{category.trim() || "<分类>"}/；目标已存在时跳过，不覆盖。
            </span>
          </div>
        ) : targets.length === 0 ? (
          <p className="skills-empty">没有检测到已启用的工具。装好工具后重新打开技能页即可看到。</p>
        ) : (
          <>
            <div className="skills-targets" role="group" aria-label="安装目标" data-testid="install-tool-targets">
              {targets.map((tool) => {
                const item = preview?.find((entry) => entry.targetId === tool.id);
                const blocked = item?.action === "exists" || item?.action === "conflict";
                const missing = Boolean(tool.missing);
                const checked = toolTargets.has(tool.id);
                const locked = busy || missing || (blocked && !overwrite.has(tool.id));
                return (
                  <div
                    key={tool.id}
                    className="skills-target"
                    role="checkbox"
                    aria-checked={checked}
                    aria-disabled={locked}
                    tabIndex={0}
                    data-testid={`install-tool-${tool.id}`}
                    onClick={() => {
                      if (locked) return;
                      setToolTargets((current) => toggle(current, tool.id));
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      if (locked) return;
                      setToolTargets((current) => toggle(current, tool.id));
                    }}
                  >
                    <ToolIcon tool={tool} size={18} />
                    <span className="skills-target-name">{tool.name}</span>
                    <span className="skills-target-note">
                      {missing
                        ? "目录不存在"
                        : item
                          ? item.detail || (item.action === "overwrite" ? "将覆盖" : item.action === "create" ? "新增" : "")
                          : tool.kind === "project"
                            ? `写入 ${tool.installRel ?? ""}`
                            : tool.kind === "custom"
                              ? tailOf(tool.installDir)
                              : tool.id === "agents"
                                ? "共用目录"
                                : "已检测到"}
                    </span>
                    {blocked ? (
                      <label className="skills-target-overwrite" onClick={(event) => event.stopPropagation()}>
                        <input
                          type="checkbox"
                          checked={overwrite.has(tool.id)}
                          disabled={busy}
                          onChange={() => {
                            setOverwrite((current) => toggle(current, tool.id));
                            setToolTargets((current) => {
                              const next = new Set(current);
                              next.add(tool.id);
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
            <div className="install-note" data-tone="info">
              <span>直接装进工具的是拷贝，不经过知识库，也不会出现在知识库技能页。想多个工具共用一份，选「装进知识库」后再分发。</span>
            </div>
          </>
        )}

        {resultLines ? (
          <div className="install-note" data-tone="ok" data-testid="install-result">
            <ul>
              {resultLines.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        ) : null}

        {error ? (
          <ErrorNote card={error} testid="install-error" onRetry={targetKind === "kb" ? () => void install() : undefined} />
        ) : null}
      </div>
    );
    footer = (
      <>
        <span data-testid="install-summary">
          {targetKind === "kb"
            ? categoryIssue
              ? `已选 ${selected.size} 个 · 先填分类再安装`
              : `已选 ${selected.size} 个 · 写入知识库 ${Math.min(eligible, selected.size)} 个目录`
            : `${selected.size} 个技能 × ${toolTargets.size} 个工具，新增 ${Math.min(creates, selected.size * toolTargets.size)} 处`}
        </span>
        <span className="spacer" />
        <button
          type="button"
          className="btn-quiet"
          disabled={busy}
          onClick={() => {
            leaveStage();
            setResultLines(null);
          }}
          data-testid="install-stage-back"
        >
          换来源
        </button>
        <button
          type="button"
          className="btn-primary"
          disabled={!canSubmit}
          onClick={() => void install()}
          title={targetKind === "kb" ? (categoryIssue ?? undefined) : undefined}
          data-testid="install-submit"
        >
          {busy ? <span className="kb-spin" aria-hidden /> : <Archive size={13} aria-hidden />}
          {busy ? "安装中…" : targetKind === "kb" ? `安装 ${selected.size} 个` : "安装"}
        </button>
      </>
    );
  } else {
    if (source === "local") stepLabel = "导入本地文件夹";
    body = (
      <div className="skills-modal-flow" data-testid="install-view">
        {source === "github" ? (
          <section className="install-github" data-testid="install-github-form">
            <div className="install-urlrow">
              <input
                className="field"
                value={input}
                placeholder="https://github.com/owner/repo 或 owner/repo"
                spellCheck={false}
                aria-label="GitHub 地址"
                onChange={(event) => {
                  setInput(event.target.value);
                  setInputProblem(null);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && !reading) void readGithub();
                }}
                data-testid="install-github-input"
              />
              <button type="button" className="btn-quiet" disabled={reading !== null} onClick={() => void readGithub()} data-testid="install-github-read">
                <GithubLogo size={13} aria-hidden />
                读取
              </button>
            </div>
            <p className="field-hint">支持 https://github.com/owner/repo、带 /tree/分支/子目录 的地址，或 owner/repo。只匿名访问 GitHub，每小时约 60 次请求。</p>
            {inputProblem ? (
              <p className="install-fault" data-testid="install-input-problem">
                {inputProblem}
              </p>
            ) : null}
          </section>
        ) : null}

        {source === "zip" ? (
          <section className="install-github" data-testid="install-zip-panel">
            <p className="field-hint">选择一个含技能的 zip 文件，大小不超过 200 MiB。解压前会检查路径，不安全的压缩包整包拒绝。</p>
            <div>
              <button type="button" className="btn-primary" disabled={reading !== null} onClick={() => void pickZip()} data-testid="install-zip-pick">
                <FileZip size={13} aria-hidden />
                选择 zip 文件…
              </button>
            </div>
          </section>
        ) : null}

        {source === "local" ? (
          <SkillImportWizard
            embedded
            onClose={() => setSource("github")}
            onBusyChange={setWizardBusy}
            onImported={onChanged}
            onFinish={() => {
              void load();
              onChanged();
              onBack();
            }}
          />
        ) : null}

        {reading ? (
          <section className="install-progress-box" data-testid="install-progress">
            <div className="install-label">
              {reading.source === "zip" ? "解压中" : (PHASE_LABEL[progress?.phase ?? "resolving"] ?? "读取中")}
              <span className="install-faint">{reading.label}</span>
            </div>
            <div
              className="install-progress"
              data-indeterminate={percent === null ? "true" : undefined}
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={percent ?? undefined}
              aria-label="读取进度"
            >
              <i style={percent === null ? undefined : { width: `${percent}%` }} />
            </div>
            <div className="install-progress-line">
              {percent !== null && progress ? (
                <span>
                  {PHASE_LABEL[progress.phase] ?? progress.phase} · {formatBytes(progress.received)} / {formatBytes(progress.total)}
                </span>
              ) : (
                <span>{reading.source === "zip" ? "读取压缩包…" : "连接 GitHub…"}</span>
              )}
              <span className="spacer" />
              {reading.source === "github" ? (
                <button
                  type="button"
                  className="btn-quiet"
                  onClick={() => abortRef.current?.abort()}
                  data-testid="install-cancel-read"
                >
                  取消
                </button>
              ) : null}
            </div>
          </section>
        ) : null}

        {error ? (
          <ErrorNote
            card={error}
            testid="install-error"
            retryLabel={error.source === "zip" ? "换一个文件…" : "重试"}
            onRetry={
              ["network-failure", "rate-limited", "unsafe-archive", "too-large"].includes(error.code)
                ? () => (error.source === "zip" ? void pickZip() : void readGithub())
                : undefined
            }
          />
        ) : null}

        {source !== "local" ? (
          <section className="install-recent">
            <h3 className="kb-group-title">最近记录</h3>
            {records === null ? (
              <p className="skills-dialog-line">读取中…</p>
            ) : records.length === 0 ? (
              <p className="skills-empty">还没有技能操作记录。</p>
            ) : (
              <ul className="install-records" data-testid="install-records">
                {records.map((record) => (
                  <li key={record.id} className="install-record">
                    <span className="install-record-op">{OP_LABEL[record.op] ?? record.op}</span>
                    <span className="install-record-skill">{record.skill || "—"}</span>
                    <span className="install-record-loc" title={record.location}>
                      {record.location}
                    </span>
                    <span className="install-record-time">{record.at}</span>
                    {record.restorable && record.backupId ? (
                      <button
                        type="button"
                        className="btn-quiet"
                        disabled={busy}
                        onClick={() => void restore(record)}
                        data-testid={`install-restore-${record.id}`}
                      >
                        <Archive size={12} aria-hidden />
                        恢复
                      </button>
                    ) : (
                      <span />
                    )}
                  </li>
                ))}
              </ul>
            )}
            {notice ? (
              <p className="skills-dialog-line" data-testid="install-notice">
                {notice}
              </p>
            ) : null}
          </section>
        ) : null}
      </div>
    );
  }

  return (
    <SkillsModal
      title="安装技能"
      step={stepLabel}
      busy={modalBusy}
      onClose={onBack}
      onBack={handleBack}
      head={finished ? undefined : tabs}
      footer={footer}
      focusKey={focusKey}
    >
      {body}
    </SkillsModal>
  );
}

function ErrorNote({
  card,
  testid,
  onRetry,
  retryLabel = "重试",
}: {
  readonly card: ErrorCard;
  readonly testid: string;
  readonly onRetry?: () => void;
  readonly retryLabel?: string;
}) {
  const hint = ERROR_HINT[card.code];
  const archiveLevel = card.code === "unsafe-archive" || card.code === "too-large";
  return (
    <section className="install-note" data-tone="fault" role="alert" data-testid={testid}>
      <WarningCircle size={16} aria-hidden />
      <div className="install-note-body">
        <b>{hint ? hint.split("。")[0] + "。" : card.title}</b>
        {card.detail ? <code>{card.detail}</code> : null}
        <p>{hint ?? card.message}</p>
        {archiveLevel ? (
          <p className="install-faint">
            其他会整包拒绝的情况：绝对路径或盘符、两个条目只差大小写、Windows 保留名（如 CON）。符号链接不算错误，直接跳过并在结果里列出。
          </p>
        ) : null}
        {onRetry ? (
          <button type="button" className="btn-quiet" onClick={onRetry}>
            {retryLabel}
          </button>
        ) : null}
      </div>
    </section>
  );
}
