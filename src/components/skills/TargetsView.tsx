import { useEffect, useState } from "react";
import { ArrowLeft, FolderOpen, Pencil, Plus, Trash } from "@phosphor-icons/react";
import { skillsApi, type SkillTarget, type SkillTargetDir } from "../../lib/transport";

type Kind = "custom" | "project";

type FormState = {
  id?: string;
  kind: Kind;
  name: string;
  path: string; // masked display only
  token: string;
  dirs: SkillTargetDir[];
  installDir: string;
};

const KIND_LABEL: Record<Kind, string> = { custom: "自定义", project: "项目" };

function failureText(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

/** 管理目录 (M6 T07; UI 稿 9.5, DESIGN.md 11.6): registered custom tools and
 * projects — list, add/edit forms and the removal confirmation. Removal only
 * drops the registration and the records that point at it; files stay. */
export function TargetsView({
  vaultId,
  onBack,
  onChanged,
  onResult,
}: {
  readonly vaultId: string;
  readonly onBack: () => void;
  readonly onChanged: () => void;
  readonly onResult?: (lines: string[]) => void;
}) {
  const [items, setItems] = useState<SkillTarget[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [form, setForm] = useState<FormState | null>(null);
  const [confirmTarget, setConfirmTarget] = useState<SkillTarget | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = async () => {
    try {
      const value = await skillsApi.listTargets();
      setItems(value.items);
      setError(null);
    } catch (err) {
      setError(failureText(err));
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  // Esc closes the innermost layer: confirm, then form, then the subview.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (confirmTarget) {
        event.stopPropagation();
        setConfirmTarget(null);
        return;
      }
      if (form) {
        event.stopPropagation();
        setForm(null);
        return;
      }
      event.stopPropagation();
      onBack();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [confirmTarget, form, onBack]);

  const atCap = (items?.length ?? 0) >= 100;

  const openCreate = (kind: Kind) => {
    setError(null);
    setForm({ kind, name: "", path: "", token: "", dirs: [], installDir: kind === "project" ? ".claude/skills" : "" });
  };

  const openEdit = (target: SkillTarget) => {
    setError(null);
    setForm({
      id: target.id,
      kind: target.kind === "project" ? "project" : "custom",
      name: target.name,
      path: target.path,
      token: "",
      dirs: target.dirs ?? [],
      installDir: target.installDir ?? ".claude/skills",
    });
  };

  const pickFolder = async () => {
    if (!form) return;
    try {
      const picked = await skillsApi.pickTargetFolder();
      if (picked.cancelled) return;
      const dirs = picked.dirs ?? [];
      setForm((current) =>
        current
          ? {
              ...current,
              token: picked.token ?? "",
              path: picked.path ?? "",
              dirs,
              installDir:
                current.kind === "project"
                  ? dirs[0]?.rel ?? ".claude/skills"
                  : current.installDir,
            }
          : current,
      );
      setError(null);
    } catch (err) {
      setError(failureText(err));
    }
  };

  const save = async () => {
    if (!form || busy) return;
    setBusy(true);
    setError(null);
    try {
      const value = await skillsApi.saveTarget({
        id: form.id,
        kind: form.kind,
        name: form.name,
        token: form.token || undefined,
        installDir: form.kind === "project" ? form.installDir : undefined,
      });
      const name = value.target?.name ?? form.name;
      setForm(null);
      await reload();
      onChanged();
      onResult?.([form.id ? `已更新登记「${name}」` : `已登记「${name}」`]);
    } catch (err) {
      setError(failureText(err));
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!confirmTarget || busy) return;
    setBusy(true);
    try {
      const value = await skillsApi.removeTarget(confirmTarget.id);
      const name = confirmTarget.name;
      setConfirmTarget(null);
      await reload();
      onChanged();
      onResult?.([
        `已移除登记「${name}」：删除 ${value.distributionsRemoved} 条分发记录、${value.sourcesRemoved} 条来源记录；目录里的文件保留`,
      ]);
    } catch (err) {
      setError(failureText(err));
      setConfirmTarget(null);
    } finally {
      setBusy(false);
    }
  };

  const installOptions = form
    ? form.dirs.length > 0
      ? form.dirs.map((dir) => dir.rel)
      : [".claude/skills"]
    : [];

  return (
    <div className="targets-view" data-testid="targets-view">
      <header className="install-head">
        <button
          type="button"
          className="icon-button"
          aria-label={form ? "返回目录列表" : "返回工具技能"}
          onClick={() => (form ? setForm(null) : onBack())}
          data-testid="targets-back"
        >
          <ArrowLeft size={14} aria-hidden />
        </button>
        <strong>
          {form
            ? form.id
              ? `编辑「${form.name || "未命名"}」`
              : form.kind === "project"
                ? "登记项目"
                : "新增自定义工具"
            : "管理目录"}
        </strong>
      </header>

      {error ? (
        <p className="install-fault" data-testid="targets-error">
          {error}
        </p>
      ) : null}

      {form ? (
        <div className="targets-form">
          <label className="skills-dialog-field">
            名称
            <input
              className="field"
              value={form.name}
              maxLength={40}
              placeholder={form.kind === "project" ? "例如：acme-site" : "例如：Droid Skills"}
              onChange={(event) => setForm({ ...form, name: event.target.value })}
              data-testid="target-name"
            />
          </label>

          <div className="skills-dialog-field">
            {form.kind === "project" ? "项目根目录" : "目录"}
            <div className="targets-pick">
              <button type="button" className="btn-quiet" onClick={() => void pickFolder()} data-testid="target-pick">
                <FolderOpen size={13} aria-hidden />
                选择文件夹…
              </button>
              <span className="targets-path" data-testid="target-picked">
                {form.token || form.path ? form.path || "已选择" : "还没有选择目录"}
              </span>
            </div>
            <span className="field-hint">只支持本机磁盘路径，不支持网络路径；目录里每个含 SKILL.md 的子目录就是一个技能。</span>
          </div>

          {form.kind === "project" ? (
            <>
              <div className="skills-dialog-field">
                检测到的约定子目录
                <div className="targets-chips" data-testid="target-dirs">
                  {form.dirs.length > 0 ? (
                    form.dirs.map((dir) => (
                      <span key={dir.rel} className="skm-chip" aria-readonly>
                        {dir.rel}
                        <span className="tool-chip-count">{dir.skills}</span>
                      </span>
                    ))
                  ) : (
                    <span className="field-hint">这个目录里还没有约定子目录。</span>
                  )}
                </div>
              </div>
              <div className="skills-dialog-field">
                分发安装到
                <div className="targets-install" role="radiogroup" aria-label="分发安装子目录">
                  {installOptions.map((rel) => (
                    <button
                      key={rel}
                      type="button"
                      role="radio"
                      aria-checked={form.installDir === rel}
                      className={form.installDir === rel ? "skm-chip targets-install-on" : "skm-chip"}
                      onClick={() => setForm({ ...form, installDir: rel })}
                      data-testid={`target-install-${rel}`}
                    >
                      {rel}
                    </button>
                  ))}
                </div>
                <span className="field-hint">分发到这个项目的技能写进这里；项目里只有这些子目录会被读取。</span>
              </div>
            </>
          ) : null}

          <div className="install-note" data-tone="info">
            <span>
              登记的目录不能在知识库里面，也不能和内置工具目录、其他已登记目录互相包含。Agent 不能自己登记。
            </span>
          </div>

          <footer className="install-foot targets-form-foot">
            <span className="spacer" />
            <button type="button" className="btn-quiet" onClick={() => setForm(null)} data-testid="targets-cancel">
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={busy || !form.name.trim() || (!form.id && !form.token)}
              onClick={() => void save()}
              data-testid="target-save"
            >
              {busy ? "保存中…" : "保存"}
            </button>
          </footer>
        </div>
      ) : (
        <>
          <div className="targets-toolbar">
            <button
              type="button"
              className="btn-quiet"
              disabled={atCap}
              title={atCap ? "最多 100 个" : undefined}
              onClick={() => openCreate("custom")}
              data-testid="targets-add-custom"
            >
              <Plus size={13} aria-hidden />
              新增自定义工具
            </button>
            <button
              type="button"
              className="btn-quiet"
              disabled={atCap}
              title={atCap ? "最多 100 个" : undefined}
              onClick={() => openCreate("project")}
              data-testid="targets-add-project"
            >
              <Plus size={13} aria-hidden />
              登记项目
            </button>
            <span className="spacer" />
            <span className="tool-chip-count" data-testid="targets-count">
              自定义 {items?.filter((item) => item.kind === "custom").length ?? 0} · 项目{" "}
              {items?.filter((item) => item.kind === "project").length ?? 0}
            </span>
          </div>

          {items === null ? (
            <p className="field-hint" data-testid="targets-loading">
              读取中…
            </p>
          ) : items.length === 0 ? (
            <div className="targets-empty" data-testid="targets-empty">
              <p>还没有登记目录。新增自定义工具或登记一个项目后，它们会和内置工具一起出现在工具技能里。</p>
            </div>
          ) : (
            <div className="targets-rows" role="list">
              {items.map((target) => (
                <div className="targets-row" role="listitem" key={target.id} data-testid={`target-row-${target.id}`}>
                  <span className="pill" data-tone={target.kind === "project" ? "info" : "idle"}>
                    {KIND_LABEL[target.kind === "project" ? "project" : "custom"]}
                  </span>
                  <div className="targets-main">
                    <span className="targets-name">
                      {target.name}
                      {target.exists ? null : (
                        <span className="pill" data-tone="warn">
                          目录不存在
                        </span>
                      )}
                    </span>
                    <span className="targets-path">{target.path}</span>
                  </div>
                  <span className="targets-stat">
                    {target.skillCount} 个技能
                    {target.kind === "project" && target.dirs
                      ? ` · ${target.dirs.filter((dir) => dir.exists).length} 个子目录`
                      : ""}
                  </span>
                  <span className="targets-ops">
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={`编辑 ${target.name}`}
                      title="编辑"
                      onClick={() => openEdit(target)}
                      data-testid={`target-edit-${target.id}`}
                    >
                      <Pencil size={13} aria-hidden />
                    </button>
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={`移除登记 ${target.name}`}
                      title="移除登记"
                      onClick={() => setConfirmTarget(target)}
                      data-testid={`target-remove-${target.id}`}
                    >
                      <Trash size={13} aria-hidden />
                    </button>
                  </span>
                </div>
              ))}
            </div>
          )}

          <div className="install-note" data-tone="info">
            <span>移除登记只删除这里的记录，不会删除目录里的文件；重新登记同一目录后，留下的链接与拷贝会重新被识别。</span>
          </div>
        </>
      )}

      {confirmTarget ? (
        <div className="skills-subview" data-testid="target-remove-dialog">
          <div className="skills-subview-card">
            <header className="skills-subview-head">
              <span className="skills-subview-title">移除登记「{confirmTarget.name}」</span>
            </header>
            <div className="skills-dialog-body">
              <p className="skills-dialog-line">
                指向它的分发记录与来源记录会一起删除，这些记录只用于在知识库技能卡片上显示「已分发」。
              </p>
              <p className="skills-dialog-line">
                <strong>目录里的文件不会删除</strong>：{confirmTarget.path} 原样保留。
              </p>
            </div>
            <footer className="skills-subview-foot">
              <span />
              <div className="skills-subview-actions">
                <button type="button" className="btn-quiet" onClick={() => setConfirmTarget(null)} disabled={busy}>
                  取消
                </button>
                <button
                  type="button"
                  className="btn-danger"
                  onClick={() => void remove()}
                  disabled={busy}
                  data-testid="target-remove-confirm"
                >
                  {busy ? "移除中…" : "移除登记"}
                </button>
              </div>
            </footer>
          </div>
        </div>
      ) : null}
    </div>
  );
}
