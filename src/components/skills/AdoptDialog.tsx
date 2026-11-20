import { useEffect, useState } from "react";
import { skillsApi, type SkillToolRef, type ToolSkill } from "../../lib/transport";

/** 收进知识库 (M4 T11; UI 稿 adopt-*): category + optional link-back; other
 * tools with the same name are merged only when their content matches. */
export function AdoptDialog({
  vaultId,
  skill,
  builtin,
  extras,
  onClose,
  onDone,
}: {
  readonly vaultId: string;
  readonly skill: ToolSkill;
  readonly builtin: boolean;
  readonly extras: SkillToolRef[];
  readonly onClose: () => void;
  readonly onDone: (warnings: string[]) => void;
}) {
  const [category, setCategory] = useState("");
  const [linkBack, setLinkBack] = useState(!builtin);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ vaultPath: string; warnings: string[] } | null>(null);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      onClose();
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  const submit = async () => {
    if (!category.trim()) {
      setError("请填写分类");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const value = await skillsApi.adopt(vaultId, skill.toolId, skill.name, category.trim(), linkBack, extras);
      setResult({ vaultPath: value.vaultPath, warnings: value.warnings ?? [] });
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="skills-subview" data-testid="adopt-dialog">
      <div className="skills-subview-card">
        <header className="skills-subview-head">
          <span className="skills-subview-title">收进知识库「{skill.name}」</span>
          <button type="button" className="icon-button" aria-label="返回列表" onClick={onClose}>
            ←
          </button>
        </header>
        <div className="skills-dialog-body">
          {result ? (
            <>
              <p className="skills-dialog-line" data-testid="adopt-result">
                已复制到 {result.vaultPath}
              </p>
              {result.warnings.map((warning) => (
                <p key={warning} className="skills-dialog-line">
                  {warning}
                </p>
              ))}
            </>
          ) : (
            <>
              <label className="skills-dialog-field">
                <span>分类</span>
                <input
                  className="kb-search"
                  value={category}
                  placeholder="例如：前端"
                  aria-label="收编分类"
                  data-testid="adopt-category"
                  onChange={(event) => setCategory(event.target.value)}
                />
              </label>
              <label className="skills-dialog-check">
                <input
                  type="checkbox"
                  checked={linkBack}
                  disabled={builtin}
                  onChange={(event) => setLinkBack(event.target.checked)}
                  data-testid="adopt-linkback"
                />
                <span>复制后把工具里的原目录换成链接（先备份{builtin ? "；内置技能不可换" : ""}）</span>
              </label>
              {extras.length > 0 ? (
                <p className="skills-dialog-line">
                  同名的其他工具副本 {extras.length} 个：内容一致会一起换成链接，不一致会跳过。
                </p>
              ) : null}
              {error ? <p className="skills-dialog-line" data-tone="fault">{error}</p> : null}
            </>
          )}
        </div>
        <footer className="skills-subview-foot">
          <span />
          <div className="skills-subview-actions">
            {result ? (
              <button type="button" className="btn-primary" onClick={() => onDone(result.warnings)} data-testid="adopt-done">
                完成
              </button>
            ) : (
              <>
                <button type="button" className="btn-quiet" onClick={onClose} disabled={busy}>
                  取消
                </button>
                <button type="button" className="btn-primary" onClick={() => void submit()} disabled={busy} data-testid="adopt-submit">
                  {busy ? "收编中…" : "收进知识库"}
                </button>
              </>
            )}
          </div>
        </footer>
      </div>
    </div>
  );
}
