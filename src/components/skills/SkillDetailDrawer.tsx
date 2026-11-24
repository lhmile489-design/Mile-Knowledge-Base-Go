import { useEffect, useRef, useState } from "react";
import { Crosshair, FileText, FolderSimple, GithubLogo, PaperPlaneTilt, PencilSimple, X } from "@phosphor-icons/react";
import { docApi, TransportError, type KbSkill, type SkillTool } from "../../lib/transport";
import { remotePill, shortSha, sourceOf } from "../../lib/skillRemote";
import { distributionLabel, skillStatus } from "../../lib/skillsView";
import { useAppStore } from "../../stores/appStore";
import { ReaderView } from "../ReaderView";
import { ToolIcon } from "./ToolIcon";

type View =
  | { state: "loading" }
  | { state: "ready"; path: string; revision: number; hash: string }
  | { state: "missing" }
  | { state: "large" }
  | { state: "error"; message: string };

/** 技能详情抽屉 (M9 T04; UI 稿 12.2 detail-*, DESIGN 14, tech design 20.3). A
 * read-only look at one knowledge-base skill: where it was distributed, where it
 * came from, and its SKILL.md rendered like reading mode. The list behind it
 * stays usable; choosing another card swaps the content in place. */
export function SkillDetailDrawer({
  vaultId,
  skill,
  tools,
  onClose,
  onDistribute,
  onUpdate,
  onEdit,
  onLocate,
  onRescan,
}: {
  readonly vaultId: string;
  readonly skill: KbSkill;
  readonly tools: SkillTool[];
  readonly onClose: () => void;
  readonly onDistribute: () => void;
  readonly onUpdate: () => void;
  readonly onEdit: () => void;
  readonly onLocate: () => void;
  readonly onRescan: () => void;
}) {
  const showPage = useAppStore((state) => state.showPage);
  const [view, setView] = useState<View>({ state: "loading" });
  const [attempt, setAttempt] = useState(0);
  const closeRef = useRef<HTMLButtonElement | null>(null);
  const filePath = `${skill.path}/SKILL.md`;

  // Read the file; a newer choice (or a retry) makes an older answer irrelevant.
  useEffect(() => {
    let alive = true;
    setView({ state: "loading" });
    docApi
      .read(vaultId, filePath)
      .then((result) => {
        if (!alive) return;
        const doc = result.doc;
        if (!doc) {
          setView({ state: "error", message: "读取失败" });
        } else if (doc.degraded === "large-source-only" || doc.degraded === "large-read-only") {
          setView({ state: "large" });
        } else {
          setView({ state: "ready", path: filePath, revision: doc.revision, hash: doc.hash });
        }
      })
      .catch((error: unknown) => {
        if (!alive) return;
        if (error instanceof TransportError && (error.code === "not-found" || error.code === "not-a-file")) {
          setView({ state: "missing" });
        } else {
          setView({ state: "error", message: error instanceof Error ? error.message : String(error) });
        }
      });
    return () => {
      alive = false;
    };
  }, [vaultId, filePath, attempt]);

  useEffect(() => {
    closeRef.current?.focus();
  }, []);

  // Esc closes the drawer, but not while a dialog above it is open (that Esc
  // belongs to the dialog) and not while an input method is composing.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.isComposing || event.defaultPrevented) return;
      if (document.querySelector('[data-testid="skills-modal"]')) return;
      onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const status = skillStatus(skill);
  const remote = remotePill(skill.remote);
  const updatable = skill.remote?.updateState === "update-available";
  const distributed = skill.distributed ?? [];

  return (
    <aside className="skills-drawer" role="dialog" aria-label="技能详情" data-testid="skill-drawer" data-state={view.state}>
      <header className="skills-drawer-head">
        <div className="skills-drawer-title">
          <span className="kb-card-glyph" aria-hidden>
            <FolderSimple size={16} />
          </span>
          <span className="skills-drawer-heading">
            <span className="skills-drawer-name">{skill.name}</span>
            <span className="skills-drawer-cat">{skill.category}</span>
          </span>
          <button
            ref={closeRef}
            type="button"
            className="icon-btn"
            aria-label="关闭详情"
            title="关闭（Esc）"
            data-testid="skill-drawer-close"
            onClick={onClose}
          >
            <X size={14} aria-hidden />
          </button>
        </div>
        <div className="skills-drawer-path" data-testid="skill-drawer-path">
          {filePath}
        </div>
        <div className="skills-drawer-pills">
          <span className="pill" data-tone={status.tone}>
            {status.label}
          </span>
          {remote ? (
            <span className="pill" data-tone={remote.tone} title={remote.title}>
              {remote.text}
            </span>
          ) : null}
        </div>
      </header>

      <div className="skills-drawer-body">
        {view.state === "missing" ? (
          <div className="skills-empty" data-testid="skill-drawer-missing">
            <FileText size={28} aria-hidden />
            <h4>找不到这个技能文件</h4>
            <p>它可能刚被移动、改名或删除。重新扫描后列表会更新。</p>
            <button
              type="button"
              className="btn-primary"
              data-testid="skill-drawer-rescan"
              onClick={() => {
                onRescan();
                setAttempt((value) => value + 1);
              }}
            >
              重新扫描
            </button>
          </div>
        ) : (
          <>
            <section>
              <h5 className="skills-drawer-sec">分发情况</h5>
              <div className="skills-drawer-dist" data-testid="skill-drawer-dist">
                {distributed.length === 0 ? (
                  <div className="skills-drawer-none">未分发</div>
                ) : (
                  distributed.map((item) => {
                    const tool = tools.find((candidate) => candidate.id === item.toolId);
                    const label = distributionLabel(item);
                    return (
                      <div key={item.toolId} className="skills-drawer-dist-row">
                        <ToolIcon tool={tool ?? { id: item.toolId, name: item.toolId, iconKey: "" }} size={18} />
                        <span>{tool?.name ?? item.toolId}</span>
                        <span className="skills-drawer-how">{label.how}</span>
                        <span className="pill" data-tone={label.tone}>
                          {label.sync}
                        </span>
                      </div>
                    );
                  })
                )}
              </div>
            </section>

            {skill.remote ? (
              <section>
                <h5 className="skills-drawer-sec">来源</h5>
                <div className="skills-drawer-source" data-testid="skill-drawer-source">
                  <GithubLogo size={14} aria-hidden />
                  <span>{sourceOf(skill.remote)}</span>
                  {remote ? (
                    <span className="pill" data-tone={remote.tone}>
                      {remote.text}
                    </span>
                  ) : (
                    <span />
                  )}
                  <span className="skills-drawer-source-sub">
                    {skill.remote.path ? `子目录 ${skill.remote.path}` : "仓库根目录"}
                    {skill.remote.pathCommit ? ` · 安装时 ${shortSha(skill.remote.pathCommit)}` : ""}
                    {skill.remote.latestPathCommit ? ` · 最新 ${shortSha(skill.remote.latestPathCommit)}` : ""}
                  </span>
                </div>
              </section>
            ) : null}

            <section>
              <h5 className="skills-drawer-sec">SKILL.md</h5>
              {view.state === "loading" ? (
                <div className="skills-drawer-skeleton" data-testid="skill-drawer-skeleton" aria-label="正在读取">
                  <span />
                  <span />
                  <span />
                </div>
              ) : view.state === "ready" ? (
                <div className="skills-drawer-preview">
                  <ReaderView
                    vaultId={vaultId}
                    path={view.path}
                    revision={view.revision}
                    hash={view.hash}
                    onBeforeNavigate={() => showPage("work")}
                  />
                </div>
              ) : view.state === "large" ? (
                <p className="skills-dialog-line" data-testid="skill-drawer-large">
                  文件较大，请在工作台打开。
                </p>
              ) : (
                <div className="install-note" data-tone="fault" role="alert" data-testid="skill-drawer-error">
                  <div className="install-note-body">
                    <b>读取失败</b>
                    <p>{view.state === "error" ? view.message : ""}</p>
                    <button
                      type="button"
                      className="btn-quiet"
                      data-testid="skill-drawer-retry"
                      onClick={() => setAttempt((value) => value + 1)}
                    >
                      重试
                    </button>
                  </div>
                </div>
              )}
            </section>
          </>
        )}
      </div>

      <footer className="skills-drawer-foot">
        <button type="button" className="btn-primary" data-testid="skill-drawer-distribute" onClick={onDistribute}>
          <PaperPlaneTilt size={13} aria-hidden />
          分发…
        </button>
        {updatable ? (
          <button type="button" className="btn-quiet" data-testid="skill-drawer-update" onClick={onUpdate}>
            更新…
          </button>
        ) : null}
        <span className="spacer" />
        <button
          type="button"
          className="icon-btn"
          aria-label="在侧栏定位"
          title="在侧栏定位"
          data-testid="skill-drawer-locate"
          onClick={onLocate}
        >
          <Crosshair size={14} aria-hidden />
        </button>
        <button type="button" className="btn-quiet" data-testid="skill-drawer-edit" onClick={onEdit}>
          <PencilSimple size={13} aria-hidden />
          在工作台编辑
        </button>
      </footer>
    </aside>
  );
}
