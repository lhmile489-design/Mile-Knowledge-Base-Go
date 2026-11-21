import { useEffect, useRef, useState } from "react";
import { ArrowLeft, GithubLogo, Info, WarningCircle } from "@phosphor-icons/react";
import {
  TransportError,
  skillsApi,
  type MutationPlan,
  type SkillUpdatePreviewResult,
} from "../../lib/transport";
import { changeSummary, formatBytes, shortSha } from "../../lib/skillRemote";

const CHANGE: Record<string, { text: string; tone: "live" | "idle" | "fault" }> = {
  added: { text: "新增", tone: "live" },
  modified: { text: "修改", tone: "idle" },
  removed: { text: "删除", tone: "fault" },
};

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  return error instanceof Error ? error.message : String(error);
}

/** 知识库技能的更新预览 (M94; UI 稿 9.4 update-preview): diff first, then the
 * plan card writes the vault. The stage lives until the plan is proposed or
 * the view closes. */
export function UpdateView({
  vaultId,
  skillPath,
  skillName,
  onClose,
  onProposed,
}: {
  readonly vaultId: string;
  readonly skillPath: string;
  readonly skillName: string;
  readonly onClose: () => void;
  readonly onProposed: (plan: MutationPlan, stageId: string) => void;
}) {
  const [preview, setPreview] = useState<SkillUpdatePreviewResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const stageRef = useRef<string | null>(null);
  const proposedRef = useRef(false);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    setError(null);
    setPreview(null);
    stageRef.current = null;
    skillsApi
      .updatePreview(vaultId, { kind: "kb", skillPath })
      .then((value) => {
        if (!alive) return;
        setPreview(value);
        stageRef.current = value.stageId ?? null;
      })
      .catch((caught) => {
        if (alive) setError(errorText(caught));
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
      const stage = stageRef.current;
      if (stage && !proposedRef.current) void skillsApi.discardStage(stage).catch(() => undefined);
    };
  }, [vaultId, skillPath]);

  const propose = () => {
    if (!preview?.plan || !preview.stageId) return;
    proposedRef.current = true;
    onProposed(preview.plan, preview.stageId);
  };

  const files = preview?.files ?? [];
  const remote = preview?.from || preview?.to;

  return (
    <div className="install-view update-view" data-testid="update-view">
      <header className="install-head">
        <button type="button" className="icon-button" aria-label="返回技能列表" onClick={onClose}>
          <ArrowLeft size={14} aria-hidden />
        </button>
        <span className="update-crumb">知识库技能 / 更新「{skillName}」</span>
      </header>

      {loading ? <p className="skills-dialog-line">正在读取新版本…</p> : null}

      {error ? (
        <section className="install-note" data-tone="fault" role="alert" data-testid="update-error">
          <WarningCircle size={16} aria-hidden />
          <div className="install-note-body">
            <b>不能更新这个技能</b>
            <p>{error}</p>
          </div>
        </section>
      ) : null}

      {!loading && !error && preview ? (
        <>
          <div className="install-origin" data-testid="update-origin">
            <GithubLogo size={16} aria-hidden />
            <b>{skillName}</b>
            <code>{skillPath}</code>
            {remote ? <code>{shortSha(preview?.from)} → {shortSha(preview?.to)}</code> : null}
            <span className="install-faint">本地没有改过</span>
          </div>

          {files.length === 0 ? (
            <section className="install-note" data-tone="info" data-testid="update-empty">
              <Info size={16} aria-hidden />
              <span>已经是最新的，没有要写入的文件。</span>
            </section>
          ) : (
            <>
              <div className="install-label">
                会改动的文件
                <span className="install-faint">{files.length} 项，知识库 {skillPath}</span>
              </div>
              <ul className="update-diff" data-testid="update-files">
                {files.map((file) => {
                  const change = CHANGE[file.change] ?? { text: file.change, tone: "idle" as const };
                  return (
                    <li key={`${file.change}:${file.path}`} className="update-diff-row">
                      <span className="pill" data-tone={change.tone}>{change.text}</span>
                      <code title={file.path}>{file.path}</code>
                      <span className="update-diff-size">
                        {file.change === "removed" ? "移到回收站" : formatBytes(file.bytes)}
                        {file.binary ? " · 二进制" : ""}
                      </span>
                    </li>
                  );
                })}
              </ul>
              <section className="install-note" data-tone="info">
                <Info size={16} aria-hidden />
                <span>
                  点「生成变更计划」后，主界面会出现计划卡；确认后才写入知识库。写入经过事务，可以在操作日志里恢复。已分发成拷贝的工具会显示「知识库有更新」。
                </span>
              </section>
            </>
          )}

          <footer className="install-foot">
            <span data-testid="update-summary">{changeSummary(files)}</span>
            <span className="spacer" />
            <button type="button" className="btn-quiet" onClick={onClose} data-testid="update-cancel">
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={!preview.plan || files.length === 0}
              onClick={propose}
              data-testid="update-propose"
            >
              生成变更计划
            </button>
          </footer>
        </>
      ) : null}
    </div>
  );
}
