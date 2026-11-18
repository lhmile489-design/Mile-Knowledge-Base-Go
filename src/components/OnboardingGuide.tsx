import { useState } from "react";
import { FolderOpen } from "@phosphor-icons/react";
import { vaultApi, TransportError, type PickFolderResult } from "../lib/transport";
import { useAppStore } from "../stores/appStore";

type Props = {
  canDismiss: boolean;
  onDismiss?: () => void;
  onDone?: () => void;
};

/** Two-step new-vault wizard (PRD FR-VLT): pick a folder, confirm what will be
 * created. A non-empty folder must be confirmed explicitly. */
export function OnboardingGuide({ canDismiss, onDismiss, onDone }: Props) {
  const refreshVaults = useAppStore((state) => state.refreshVaults);
  const [pick, setPick] = useState<PickFolderResult | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const step = pick ? 2 : 1;

  const chooseFolder = async () => {
    setBusy(true);
    setError(null);
    try {
      const result = await vaultApi.pickFolder();
      if (result.cancelled) return;
      setPick(result);
      setConfirmed(result.empty);
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  const create = async () => {
    if (!pick?.token) return;
    setBusy(true);
    setError(null);
    try {
      const result = await vaultApi.createVault(pick.token, confirmed);
      if (result.cancelled) return;
      await refreshVaults();
      onDone?.();
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="surface-card onboarding" aria-label="新建知识库向导">
      <p className="eyebrow">新建知识库 · 第 {step} 步 / 共 2 步</p>

      {step === 1 ? (
        <>
          <h1 className="welcome-title">选择知识库目录</h1>
          <p className="overview-note">
            可以选一个已有的 Markdown 目录，也可以选一个空目录，由应用建好规范的子目录。
          </p>
          <div className="welcome-actions">
            <button type="button" className="btn-accent" onClick={() => void chooseFolder()} disabled={busy}>
              <FolderOpen size={15} weight="bold" aria-hidden />
              {busy ? "正在选择" : "选择文件夹"}
            </button>
            {canDismiss ? (
              <button type="button" className="btn-quiet" onClick={onDismiss} disabled={busy}>
                返回
              </button>
            ) : null}
          </div>
        </>
      ) : (
        <>
          <h1 className="welcome-title">确认</h1>
          <p className="overview-note">
            目录：{pick?.displayName}
            {pick?.empty ? "（空目录）" : `（已有 ${pick?.entryCount ?? 0} 项内容）`}
          </p>
          {pick && pick.willCreate.length > 0 ? (
            <p className="overview-note">将创建目录：{pick.willCreate.join("、")}</p>
          ) : (
            <p className="overview-note">目录已包含规范结构，不会新建任何目录。</p>
          )}
          {pick && !pick.empty ? (
            <label className="field-check">
              <input type="checkbox" checked={confirmed} onChange={(e) => setConfirmed(e.target.checked)} />
              我知道这是一个非空目录，仍要用它作为知识库
            </label>
          ) : null}
          <div className="welcome-actions">
            <button
              type="button"
              className="btn-accent"
              onClick={() => void create()}
              disabled={busy || !confirmed}
            >
              {busy ? "正在创建" : "创建并打开"}
            </button>
            <button type="button" className="btn-quiet" onClick={() => setPick(null)} disabled={busy}>
              重新选择
            </button>
            {canDismiss ? (
              <button type="button" className="btn-quiet" onClick={onDismiss} disabled={busy}>
                返回
              </button>
            ) : null}
          </div>
        </>
      )}

      {error ? (
        <p className="welcome-alert" data-tone="fault" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}
