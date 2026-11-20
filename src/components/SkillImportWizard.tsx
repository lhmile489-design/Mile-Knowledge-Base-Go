import { useEffect, useMemo, useRef, useState } from "react";
import { X } from "@phosphor-icons/react";
import type { ImportSkillsResult, SkillCandidate } from "../mygo";
import {
  categoryProblem,
  EMPTY_SOURCE_HINT,
  importSummary,
  SKILL_NAME_MAX,
  skipLabel,
} from "../lib/skillImport";
import { TransportError, vaultApi } from "../lib/transport";
import { useAppStore } from "../stores/appStore";
import { useTreeStore } from "../stores/treeStore";

type Step = "source" | "select" | "done";

/** Skill import wizard (M3 T08; PRD FR-SKILL). Three steps: pick a source
 * folder, choose skills and a category, then the result. The overlay starts
 * below the title bar, Esc closes it and focus returns to the sidebar entry. */
export function SkillImportWizard({
  embedded = false,
  onClose,
}: {
  readonly embedded?: boolean;
  readonly onClose?: () => void;
} = {}) {
  const storeOpen = useAppStore((state) => state.skillWizardOpen);
  const closeWizard = useAppStore((state) => state.closeSkillWizard);
  const open = embedded || storeOpen;
  const close = () => (embedded ? onClose?.() : closeWizard());
  const vaultId = useAppStore((state) => state.activeVaultId);
  const invalidateTree = useTreeStore((state) => state.invalidate);
  const [step, setStep] = useState<Step>("source");
  const [token, setToken] = useState("");
  const [candidates, setCandidates] = useState<SkillCandidate[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [category, setCategory] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ImportSkillsResult | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);

  // 每次打开都从第一步开始，避免上次的残留状态。
  useEffect(() => {
    if (!open) return;
    setStep("source");
    setToken("");
    setCandidates([]);
    setSelected(new Set());
    setCategory("");
    setError(null);
    setResult(null);
    window.requestAnimationFrame(() => panelRef.current?.focus());
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopPropagation();
      close();
      window.requestAnimationFrame(() =>
        document.querySelector<HTMLButtonElement>('[data-testid="open-skills"]')?.focus(),
      );
    };
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [open, embedded, onClose]);

  const problem = useMemo(() => categoryProblem(category), [category]);
  const sorted = useMemo(
    () => [...candidates].sort((a, b) => a.name.localeCompare(b.name)),
    [candidates],
  );

  if (!open) return null;

  const pickSource = async (): Promise<void> => {
    setBusy(true);
    setError(null);
    try {
      const picked = await vaultApi.pickSkillSource();
      if (picked.cancelled) return;
      if (!picked.candidates || picked.candidates.length === 0) {
        setError(EMPTY_SOURCE_HINT);
        return;
      }
      setToken(picked.token ?? "");
      setCandidates(picked.candidates);
      setSelected(new Set(picked.candidates.map((item) => item.name)));
      setStep("select");
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const runImport = async (): Promise<void> => {
    if (!vaultId || !token || problem || selected.size === 0) return;
    setBusy(true);
    setError(null);
    try {
      const outcome = await vaultApi.importSkills(vaultId, token, category.trim(), [...selected]);
      setResult(outcome);
      setStep("done");
      if ((outcome.imported ?? []).length > 0) {
        void invalidateTree(["skills", `skills/${category.trim()}`]);
      }
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const toggleName = (name: string, checked: boolean): void => {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(name);
      else next.delete(name);
      return next;
    });
  };

  const panel = (
      <div
        ref={panelRef}
        className={embedded ? "skill-wizard skill-wizard-embedded" : "skill-wizard"}
        role="dialog"
        aria-modal={embedded ? undefined : "true"}
        aria-labelledby="skill-wizard-title"
        tabIndex={-1}
        data-testid="skill-wizard"
      >
        <div className="skill-wizard-head">
          <strong id="skill-wizard-title">
            导入技能
            <span className="skill-wizard-step">
              {step === "source" ? "第 1 步 / 共 3 步" : step === "select" ? "第 2 步 / 共 3 步" : "第 3 步 / 共 3 步"}
            </span>
          </strong>
          <button
            type="button"
            className="icon-button"
            aria-label="关闭导入向导"
            onClick={() => close()}
          >
            <X size={15} aria-hidden />
          </button>
        </div>

        {error ? (
          <p className="settings-error" role="alert" data-testid="skill-wizard-error">
            {error}
          </p>
        ) : null}

        {step === "source" ? (
          <div className="skill-wizard-body">
            <p className="field-hint">
              选择一个包含技能的文件夹：可以直接选技能本身，也可以选装着多个技能的上一级目录。
            </p>
            <div className="skill-wizard-actions">
              <button
                type="button"
                className="btn-primary"
                onClick={() => void pickSource()}
                disabled={busy}
                data-testid="skill-wizard-pick"
              >
                {busy ? "正在读取…" : "选择来源文件夹"}
              </button>
            </div>
          </div>
        ) : null}

        {step === "select" ? (
          <div className="skill-wizard-body">
            <div className="skill-wizard-field">
              <label className="setting-row-label" htmlFor="skill-category">
                分类名
              </label>
              <input
                id="skill-category"
                className="field"
                value={category}
                maxLength={SKILL_NAME_MAX}
                placeholder="例如：前端"
                onChange={(event) => setCategory(event.target.value)}
                data-testid="skill-wizard-category"
                aria-invalid={problem ? "true" : undefined}
              />
              <p className="field-hint" data-testid="skill-wizard-category-problem">
                {problem ?? "技能会导入到 skills/<分类>/<技能名>/，同名不覆盖。"}
              </p>
            </div>

            <div className="skill-wizard-list" role="group" aria-label="可选技能">
              {sorted.map((item) => (
                <label key={item.name} className="skill-row">
                  <input
                    type="checkbox"
                    className="field-check"
                    checked={selected.has(item.name)}
                    onChange={(event) => toggleName(item.name, event.target.checked)}
                    data-testid={`skill-wizard-item-${item.name}`}
                  />
                  <span className="skill-name">{item.name}</span>
                  <span className="skill-path">{item.fileCount} 个文件</span>
                </label>
              ))}
            </div>

            <div className="skill-wizard-actions">
              <button type="button" className="btn line" onClick={() => setStep("source")} disabled={busy}>
                上一步
              </button>
              <button
                type="button"
                className="btn-primary"
                onClick={() => void runImport()}
                disabled={busy || !!problem || selected.size === 0}
                data-testid="skill-wizard-import"
              >
                {busy ? "正在导入…" : `导入所选（${selected.size}）`}
              </button>
            </div>
          </div>
        ) : null}

        {step === "done" && result ? (
          <div className="skill-wizard-body">
            <p className="setting-row-label" data-testid="skill-wizard-summary">
              {importSummary(result.imported ?? [])}
            </p>
            {(result.imported ?? []).length > 0 ? (
              <ul className="skill-wizard-done-list">
                {(result.imported ?? []).map((name) => (
                  <li key={name}>{name}</li>
                ))}
              </ul>
            ) : null}
            {(result.skipped ?? []).length > 0 ? (
              <div className="skill-wizard-skipped" data-testid="skill-wizard-skipped">
                <strong>跳过 {result.skipped.length} 个：</strong>
                <ul>
                  {result.skipped.map((item) => (
                    <li key={item.name}>
                      {item.name} — {skipLabel(item.reason)}
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}
            <div className="skill-wizard-actions">
              <button
                type="button"
                className="btn line"
                onClick={() => {
                  setCategory("");
                  setStep("select");
                }}
                data-testid="skill-wizard-again"
              >
                换个分类再导入
              </button>
              <button type="button" className="btn-primary" onClick={() => close()} data-testid="skill-wizard-done">
                完成
              </button>
            </div>
          </div>
        ) : null}
      </div>
  );
  if (embedded) return panel;
  return (
    <div className="skill-wizard-backdrop" role="presentation">
      {panel}
    </div>
  );
}
