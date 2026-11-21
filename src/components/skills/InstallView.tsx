import { useEffect, useMemo, useRef, useState } from "react";
import { Archive, ArrowLeft, FileZip, FolderOpen, GithubLogo, WarningCircle } from "@phosphor-icons/react";
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
import { SkillImportWizard } from "../SkillImportWizard";
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

/** 安装视图 (M5 T10; UI 稿 9.4 install-*): three sources, candidate checkboxes
 * with disabled reasons, knowledge base or direct-tool target, progress with
 * cancel, fault cards, and the existing local-folder wizard. */
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
  const [wizard, setWizard] = useState(false);
  const [records, setRecords] = useState<SkillLogRecord[] | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [source, setSource] = useState<"github" | null>(null);
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

  // Leaving the view drops a staged download the user never installed.
  useEffect(
    () => () => {
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

  const eligibleTools = useMemo(
    () =>
      tools
        .filter((tool) => tool.enabled && (tool.id === "agents" || (tool.status === "detected" && Boolean(tool.installDir))))
        .sort((a, b) => (a.id === "agents" ? -1 : b.id === "agents" ? 1 : a.name.localeCompare(b.name))),
    [tools],
  );

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

  const leaveStage = () => {
    if (stage) void skillsApi.discardStage(stage.stageId).catch(() => undefined);
    resetStage();
    setSource(null);
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
        return `${candidate?.name ?? item.targetId} → ${where}：${verb}${item.detail ? `（${item.detail}）` : ""}`;
      });
      setResultLines(lines);
      onChanged();
      const installedAny = value.items.some((item) => item.result === "copied" || item.result === "linked");
      if (installedAny) {
        resetStage();
        setSource(null);
        void load();
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

  if (wizard) {
    return (
      <SkillImportWizard
        embedded
        onClose={() => {
          setWizard(false);
          void load();
          onChanged();
        }}
      />
    );
  }

  const categoryIssue = targetKind === "kb" ? categoryProblem(category) : null;
  const canSubmit =
    Boolean(stage) && selected.size > 0 && !busy && (targetKind === "kb" ? categoryIssue === null : toolTargets.size > 0);
  const percent = progress && progress.total > 0 ? Math.max(0, Math.min(100, Math.round((progress.received * 100) / progress.total))) : null;

  if (stage) {
    const eligible = stage.candidates.filter((candidate) => candidate.state === "ok").length;
    const skippedLinks = (stage.skipped ?? []).filter((item) => item.reason === "symlink" || item.reason === "special").length;
    const capped = (stage.skipped ?? []).some((item) => item.reason === "capped");
    const targets = eligibleTools;
    const creates = preview ? preview.filter((item) => item.action === "create").length : selected.size * toolTargets.size;

    return (
      <div className="install-view" data-testid="install-stage">
        <header className="install-head">
          <button
            type="button"
            className="icon-button"
            aria-label="返回安装来源"
            onClick={() => {
              leaveStage();
              setResultLines(null);
            }}
            data-testid="install-stage-back"
          >
            <ArrowLeft size={14} aria-hidden />
          </button>
          <strong>安装</strong>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={onBack}>
            关闭
          </button>
        </header>

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
                      disabled={!usable}
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
              onChange={(event) => setCategory(event.target.value)}
              data-testid="install-category"
            />
            <span className="field-hint">
              将复制到 知识库/skills/{category.trim() || "<分类>"}/；目标已存在时跳过，不覆盖。
            </span>
          </div>
        ) : targets.length === 0 ? (
          <p className="skills-empty">没有检测到已启用的工具。装好工具后重开浮层即可看到。</p>
        ) : (
          <>
            <div className="skills-targets" role="group" aria-label="安装目标" data-testid="install-tool-targets">
              {targets.map((tool) => {
                const item = preview?.find((entry) => entry.targetId === tool.id);
                const blocked = item?.action === "exists" || item?.action === "conflict";
                const checked = toolTargets.has(tool.id);
                return (
                  <div
                    key={tool.id}
                    className="skills-target"
                    role="checkbox"
                    aria-checked={checked}
                    aria-disabled={blocked && !overwrite.has(tool.id)}
                    tabIndex={0}
                    data-testid={`install-tool-${tool.id}`}
                    onClick={() => {
                      if (blocked && !overwrite.has(tool.id)) return;
                      setToolTargets((current) => toggle(current, tool.id));
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      if (blocked && !overwrite.has(tool.id)) return;
                      setToolTargets((current) => toggle(current, tool.id));
                    }}
                  >
                    <ToolIcon tool={tool} size={18} />
                    <span className="skills-target-name">{tool.name}</span>
                    <span className="skills-target-note">
                      {item
                        ? item.detail || (item.action === "overwrite" ? "将覆盖" : item.action === "create" ? "新增" : "")
                        : tool.id === "agents"
                          ? "共用目录"
                          : "已检测到"}
                    </span>
                    {blocked ? (
                      <label className="skills-target-overwrite" onClick={(event) => event.stopPropagation()}>
                        <input
                          type="checkbox"
                          checked={overwrite.has(tool.id)}
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

        <footer className="install-foot">
          <span data-testid="install-summary">
            {targetKind === "kb"
              ? categoryIssue
                ? `已选 ${selected.size} 个 · 先填分类再安装`
                : `已选 ${selected.size} 个 · 写入知识库 ${Math.min(eligible, selected.size)} 个目录`
              : `${selected.size} 个技能 × ${toolTargets.size} 个工具，新增 ${Math.min(creates, selected.size * toolTargets.size)} 处`}
          </span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={() => { leaveStage(); setResultLines(null); }} data-testid="install-cancel">
            取消
          </button>
          <button
            type="button"
            className="btn-primary"
            disabled={!canSubmit}
            onClick={() => void install()}
            title={targetKind === "kb" ? (categoryIssue ?? undefined) : undefined}
            data-testid="install-submit"
          >
            <Archive size={13} aria-hidden />
            {targetKind === "kb" ? `安装 ${selected.size} 个` : "安装"}
          </button>
        </footer>
      </div>
    );
  }

  return (
    <div className="install-view" data-testid="install-view">
      <header className="install-head">
        <button type="button" className="icon-button" aria-label="返回技能列表" onClick={onBack}>
          <ArrowLeft size={14} aria-hidden />
        </button>
        <strong>安装</strong>
      </header>
      <p className="field-hint">选择一个来源。装进知识库，或直接装进工具。</p>

      {resultLines ? (
        <div className="install-note" data-tone="ok" data-testid="install-result">
          <ul>
            {resultLines.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
          <button type="button" className="btn-quiet" onClick={() => setResultLines(null)}>
            关闭
          </button>
        </div>
      ) : null}

      <div className="install-sources">
        <button type="button" className="install-source" onClick={() => setWizard(true)} data-testid="install-local">
          <FolderOpen size={20} aria-hidden />
          <span>本地文件夹</span>
        </button>
        <button
          type="button"
          className="install-source"
          data-active={source === "github"}
          onClick={() => {
            setSource(source === "github" ? null : "github");
            setError(null);
            setInputProblem(null);
          }}
          data-testid="install-github"
        >
          <GithubLogo size={20} aria-hidden />
          <span>GitHub</span>
        </button>
        <button type="button" className="install-source" onClick={() => void pickZip()} disabled={reading !== null} data-testid="install-zip">
          <FileZip size={20} aria-hidden />
          <span>zip 文件</span>
        </button>
      </div>

      {source === "github" ? (
        <section className="install-github" data-testid="install-github-form">
          <div className="install-urlrow">
            <input
              className="field"
              value={input}
              placeholder="https://github.com/owner/repo 或 owner/repo"
              spellCheck={false}
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
    </div>
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
