import { useEffect, useState } from "react";
import { ArrowClockwise, ClockCounterClockwise, FolderOpen, Trash, Warning } from "@phosphor-icons/react";
import { OnboardingGuide } from "./OnboardingGuide";
import { useAppStore } from "../stores/appStore";
import { TransportError } from "../lib/transport";

const MAX_RECENT_TILES = 6;

/** Welcome page: the only place with the accent action when no vault is open
 * (PRD FR-VLT, DESIGN.md). All registered vaults being unavailable also lands
 * here (D-006), with fault cards instead of the workbench shell. */
export function WelcomeView() {
  const vaults = useAppStore((state) => state.vaults);
  const unavailable = useAppStore((state) => state.unavailable);
  const chooseVault = useAppStore((state) => state.chooseVault);
  const openVault = useAppStore((state) => state.openVault);
  const forgetVault = useAppStore((state) => state.forgetVault);
  const retryVault = useAppStore((state) => state.retryVault);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [guideOpen, setGuideOpen] = useState(false);

  const firstRun = vaults.length === 0 && unavailable.length === 0;
  const shown = vaults.slice(0, MAX_RECENT_TILES);

  const run = async (work: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await work();
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  // 向导是覆盖层（旧版结构）：欢迎页始终在下面，Esc 只在允许返回时关闭。
  useEffect(() => {
    if (!guideOpen || firstRun) return;
    const onKey = (event: KeyboardEvent): void => {
      if (event.key === "Escape") setGuideOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [guideOpen, firstRun]);

  return (
    <>
      <main className="welcome-shell" aria-label="欢迎">
        <div className="welcome-grid">
          <section className="surface-card overview-tile welcome-hero" aria-labelledby="welcome-heading">
            <h1 id="welcome-heading" className="welcome-title">
              选择一个知识库
            </h1>
            <p className="overview-note">
              选择一个本地目录作为知识库根目录。普通 Markdown 文件和附件始终是唯一事实源，不导入专有格式，也不改变你用文件管理器或 Git 访问它们的方式。
            </p>
            <div className="welcome-actions">
              <button
                type="button"
                className="btn-accent"
                onClick={() => void run(chooseVault)}
                disabled={busy}
              >
                <FolderOpen size={15} weight="bold" aria-hidden />
                {busy ? "正在打开" : "选择目录"}
              </button>
              <button type="button" className="btn-quiet" onClick={() => setGuideOpen(true)} disabled={busy}>
                新建知识库向导
              </button>
            </div>
            {error ? (
              <p className="welcome-alert" data-tone="fault" role="alert">
                {error}
              </p>
            ) : null}
          </section>

          {shown.map((vault) => (
            <article key={vault.id} className="surface-card overview-tile" aria-label={vault.displayName}>
              <div className="welcome-tile-head">
                <span className="overview-icon" aria-hidden>
                  <ClockCounterClockwise size={14} weight="bold" />
                </span>
                <span className="overview-sub">最近使用</span>
                <button
                  type="button"
                  className="icon-btn"
                  aria-label={`移除 ${vault.displayName} 的记录`}
                  onClick={() => void run(() => forgetVault(vault.id))}
                >
                  <Trash size={13} aria-hidden />
                </button>
              </div>
              <button type="button" className="welcome-vault" onClick={() => void run(() => openVault(vault.id))}>
                <span className="welcome-vault-name">{vault.displayName}</span>
                <span className="welcome-vault-path">{vault.root}</span>
              </button>
            </article>
          ))}

          {unavailable.map((vault) => (
            <article
              key={vault.id}
              className="surface-card overview-tile welcome-unavailable"
              data-tone="fault"
              aria-label={vault.displayName}
            >
              <div className="welcome-tile-head">
                <span className="overview-icon" aria-hidden>
                  <Warning size={14} weight="bold" />
                </span>
                <span className="overview-sub">已断开</span>
              </div>
              <p className="welcome-vault-name">{vault.displayName}</p>
              <p className="welcome-vault-path">
                {vault.root} · {vault.reason === "missing" ? "目录不存在" : "没有权限"}
              </p>
              <div className="welcome-unavailable-actions">
                <button type="button" className="btn-quiet" onClick={() => void run(() => retryVault(vault.id))}>
                  <ArrowClockwise size={13} aria-hidden /> 重试
                </button>
                <button type="button" className="btn-quiet" onClick={() => void run(() => forgetVault(vault.id))}>
                  <Trash size={13} aria-hidden /> 移除记录
                </button>
              </div>
            </article>
          ))}
        </div>
        <p className="welcome-footnote">移除记录只影响本应用的列表，不会移动或删除任何文件。</p>
      </main>
      {guideOpen ? (
        <div className="welcome-guide-backdrop" role="presentation">
          <OnboardingGuide
            canDismiss={!firstRun}
            onDismiss={() => setGuideOpen(false)}
            onDone={() => setGuideOpen(false)}
          />
        </div>
      ) : null}
    </>
  );
}
