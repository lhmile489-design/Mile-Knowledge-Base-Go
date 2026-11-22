import { useEffect, useMemo, useRef, useState } from "react";
import { Plus, X } from "@phosphor-icons/react";
import { events } from "../../mygo";
import { skillsApi, type SkillUpdateCheckResult } from "../../lib/transport";
import { checkedLabel, clockOf } from "../../lib/skillRemote";
import { useAppStore } from "../../stores/appStore";
import { useSkillsStore } from "../../stores/skillsStore";
import { totals } from "../../lib/skillsView";
import { useDocStore } from "../../stores/docStore";
import { useTreeStore } from "../../stores/treeStore";
import { useMutationStore } from "../../stores/mutationStore";
import { KbSkillsPage } from "./KbSkillsPage";
import { DistributeDialog } from "./DistributeDialog";
import { ToolSkillsPage } from "./ToolSkillsPage";
import { InstallView } from "./InstallView";
import { UpdateView } from "./UpdateView";
import { TargetsView } from "./TargetsView";
import { RestoreView } from "./RestoreView";

type Page = "kb" | "tools";

type PendingUpdate = { vaultId: string; stageId: string; planId: string; name: string };

function subscribeSkillsChanged(listener: () => void): () => void {
  try {
    return events.skillsChanged.on(listener);
  } catch {
    return () => undefined;
  }
}

/** The skills panel shell (M4 T09; UI 稿 9.1–9.2). T10/T11 fill the two
 * pages; this task delivers the layer, the two tabs, the install entry, the
 * badge refresh, and the keyboard/focus rules. */
export function SkillsPanel() {
  const open = useAppStore((state) => state.skillsPanelOpen);
  const closePanel = useAppStore((state) => state.closeSkillsPanel);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const chooseVault = useAppStore((state) => state.chooseVault);
  const scan = useSkillsStore((state) => state.scan);
  const tools = useSkillsStore((state) => state.tools);
  const [page, setPage] = useState<Page>("kb");
  const [distributePaths, setDistributePaths] = useState<string[] | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const [updatePath, setUpdatePath] = useState<string | null>(null);
  const [updateInfo, setUpdateInfo] = useState<SkillUpdateCheckResult | null>(null);
  const [targetsOpen, setTargetsOpen] = useState(false);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const targetsRequest = useAppStore((state) => state.targetsRequest);
  const [pendingUpdate, setPendingUpdate] = useState<PendingUpdate | null>(null);
  const [resultLines, setResultLines] = useState<string[] | null>(null);
  const [box, setBox] = useState<{ right: number; width: number; height: number } | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const pendingPlan = useMutationStore((state) => state.pending);
  const mutationRunning = useMutationStore((state) => state.running);
  const mutationReport = useMutationStore((state) => state.report);

  // Position under the title-bar button, never crossing the sidebar (9.1).
  useEffect(() => {
    if (!open) return;
    const compute = () => {
      const button = document.querySelector('[data-testid="open-skills"]');
      const buttonRight = button
        ? button.getBoundingClientRect().right
        : window.innerWidth - 220;
      const sidebar = useAppStore.getState().settings?.layout?.sidebarWidth ?? 232;
      const right = Math.max(12, Math.round(window.innerWidth - buttonRight));
      const width = Math.max(360, Math.min(760, window.innerWidth - sidebar - 24 - right));
      const top = 46;
      const height = Math.max(280, Math.min(560, window.innerHeight - top - 8));
      setBox({ right, width, height });
    };
    compute();
    window.addEventListener("resize", compute);
    return () => window.removeEventListener("resize", compute);
  }, [open]);

  // Another page (settings → 关于与诊断) can ask for the target manager (M6 T08).
  useEffect(() => {
    if (targetsRequest === 0) return;
    setPage("tools");
    setInstallOpen(false);
    setUpdatePath(null);
    setTargetsOpen(true);
  }, [targetsRequest]);

  // A panel opened without a vault never fetched the registry; do it here so
  // the tools page works everywhere (PRD FR-SKM-PANEL).
  useEffect(() => {
    if (!open) return;
    if (useSkillsStore.getState().tools.length === 0) {
      void useSkillsStore.getState().refresh(vaultId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, vaultId]);

  const refreshUpdates = async (vault: string, force: boolean) => {
    try {
      setUpdateInfo(await skillsApi.checkUpdates(vault, force));
    } catch {
      // A failed check leaves the previous footer state in place.
    }
  };

  // Q-SKM-2: opening the panel asks M93; E12 refreshes the scan and the state.
  useEffect(() => {
    if (!open) return;
    void refreshUpdates(vaultId, false);
    const off = subscribeSkillsChanged(() => {
      if (vaultId) void useSkillsStore.getState().loadScan(vaultId);
      void refreshUpdates(vaultId, false);
    });
    return off;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, vaultId]);

  // After the plan card runs, M95 moves the source record only when the tree
  // matches the staged version; closing the card drops the stage instead.
  useEffect(() => {
    if (!pendingUpdate) return;
    if (mutationReport && mutationReport.planId === pendingUpdate.planId) {
      const target = pendingUpdate;
      setPendingUpdate(null);
      void (async () => {
        try {
          const value = await skillsApi.confirmKbUpdate(target.vaultId, target.stageId);
          setResultLines([
            value.updated
              ? `已更新「${target.name}」`
              : `「${target.name}」没有全部写入${value.failure ? `：${value.failure.message}` : ""}，可以重新检查更新`,
          ]);
        } catch (error) {
          setResultLines([`更新确认失败：${error instanceof Error ? error.message : String(error)}`]);
        }
        if (target.vaultId) {
          void useSkillsStore.getState().loadScan(target.vaultId);
          void refreshUpdates(target.vaultId, false);
        }
      })();
      return;
    }
    if (!pendingPlan && !mutationRunning) {
      const target = pendingUpdate;
      setPendingUpdate(null);
      void skillsApi.discardStage(target.stageId).catch(() => undefined);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mutationReport, pendingPlan, mutationRunning, pendingUpdate]);

  const updateSkillName = useMemo(() => {
    if (!updatePath) return "";
    for (const group of scan?.kb ?? []) {
      for (const skill of group.skills) {
        if (skill.path === updatePath) return skill.name;
      }
    }
    return updatePath.split("/").pop() ?? updatePath;
  }, [scan, updatePath]);

  const updateRemote = useMemo(() => {
    if (!updatePath) return undefined;
    for (const group of scan?.kb ?? []) {
      for (const skill of group.skills) {
        if (skill.path === updatePath) return skill.remote;
      }
    }
    return undefined;
  }, [scan, updatePath]);

  useEffect(() => {
    if (!open) return;
    const first = panelRef.current?.querySelector<HTMLElement>("[data-skills-tab]");
    first?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        // A subview (distribute/uninstall/adopt/wizard) handles its own Esc.
        if (
          document.querySelector(
            '[data-testid="distribute-dialog"], [data-testid="uninstall-dialog"], [data-testid="adopt-dialog"], [data-testid="skill-wizard"], [data-testid="targets-view"], [data-testid="restore-view"]',
          )
        ) {
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        close();
        return;
      }
      if (event.key !== "Tab" || !panelRef.current) return;
      const focusables = panelRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      );
      if (focusables.length === 0) return;
      const first = focusables[0];
      const last = focusables[focusables.length - 1];
      if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      } else if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      }
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
    // close is stable enough for this effect
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const close = () => {
    closePanel();
    window.requestAnimationFrame(() => {
      document.querySelector<HTMLButtonElement>('[data-testid="open-skills"]')?.focus();
    });
  };

  if (!open) return null;

  const counts = totals(scan);
  const scanning = scan?.scanning ?? false;

  return (
    <div className="skills-layer" data-testid="skills-layer">
      <div
        className="skills-backdrop"
        aria-hidden
        onPointerDown={() => close()}
      />
      <div
        ref={panelRef}
        id="skills-panel"
        className="skills-panel"
        role="dialog"
        aria-label="技能"
        data-testid="skills-panel"
        style={box ? { right: box.right, width: box.width, height: box.height } : undefined}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <header className="skills-head">
          <div className="segmented skills-tabs" role="tablist" aria-label="技能页">
            {(["kb", "tools"] as const).map((id, index) => (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={page === id}
                className="segmented-item"
                tabIndex={page === id || index === 0 ? 0 : -1}
                data-skills-tab
                onClick={() => {
                  setInstallOpen(false);
                  setUpdatePath(null);
                  setTargetsOpen(false);
                  setPage(id);
                }}
                onKeyDown={(event) => {
                  if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
                  event.preventDefault();
                  const next: Page = page === "kb" ? "tools" : "kb";
                  setPage(next);
                  const target = panelRef.current?.querySelectorAll<HTMLElement>("[data-skills-tab]")[
                    next === "kb" ? 0 : 1
                  ];
                  target?.focus();
                }}
              >
                {id === "kb" ? "知识库技能" : "工具技能"}
                <span className="skills-count">{id === "kb" ? counts.kb : counts.tools}</span>
              </button>
            ))}
          </div>
          <button
            type="button"
            className="btn-quiet skills-install"
            onClick={() => {
              setDistributePaths(null);
              setUpdatePath(null);
              setTargetsOpen(false);
              setInstallOpen(true);
            }}
          >
            <Plus size={13} aria-hidden />
            安装
          </button>
          <button type="button" className="icon-button" aria-label="关闭技能浮层" onClick={() => close()} data-testid="skills-close">
            <X size={14} aria-hidden />
          </button>
        </header>

        <div className="skills-body scroll-y" data-testid="skills-body">
          {resultLines ? (
            <div className="skills-banner" role="status" data-testid="skills-banner">
              <ul>
                {resultLines.map((line) => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
              <button type="button" className="btn-quiet" onClick={() => setResultLines(null)} data-testid="skills-banner-close">
                关闭
              </button>
            </div>
          ) : null}
          {updatePath && vaultId ? (
            <UpdateView
              vaultId={vaultId}
              skillPath={updatePath}
              skillName={updateSkillName}
              remote={updateRemote}
              onClose={() => setUpdatePath(null)}
              onProposed={(plan, stageId) => {
                setPendingUpdate({ vaultId, stageId, planId: plan.id, name: updateSkillName });
                useMutationStore.getState().propose(plan, `更新技能「${updateSkillName}」`);
                setUpdatePath(null);
              }}
            />
          ) : restoreOpen && vaultId ? (
            <RestoreView
              vaultId={vaultId}
              onBack={() => setRestoreOpen(false)}
              onChanged={() => {
                if (vaultId) void useSkillsStore.getState().refresh(vaultId);
              }}
              onResult={(lines) => setResultLines(lines)}
            />
          ) : targetsOpen ? (
            <TargetsView
              vaultId={vaultId}
              onBack={() => setTargetsOpen(false)}
              onChanged={() => {
                void useSkillsStore.getState().refresh(vaultId);
              }}
              onResult={(lines) => setResultLines(lines)}
            />
          ) : installOpen && vaultId ? (
            <InstallView
              vaultId={vaultId}
              tools={tools}
              onBack={() => setInstallOpen(false)}
              onChanged={() => {
                if (vaultId) void useSkillsStore.getState().loadScan(vaultId);
              }}
            />
          ) : !scan && vaultId ? (
            <div className="skills-skeleton" aria-label="正在扫描" data-testid="skills-skeleton">
              {[0, 1, 2, 3, 4].map((row) => (
                <span key={row} className="skills-skeleton-row" />
              ))}
            </div>
          ) : page === "kb" ? (
            vaultId ? (
              <KbSkillsPage
                groups={scan?.kb ?? []}
                tools={tools}
                onOpenSkill={(path) => {
                  void useDocStore.getState().open(vaultId, `${path}/SKILL.md`);
                  close();
                }}
                onLocate={(path) => {
                  void useTreeStore.getState().reveal(path);
                  close();
                }}
                onDistribute={(paths) => setDistributePaths(paths)}
                onInstall={() => {
                  setUpdatePath(null);
                  setInstallOpen(true);
                }}
                onUpdate={(skill) => {
                  setInstallOpen(false);
                  setUpdatePath(skill.path);
                }}
              />
            ) : (
              <div className="skills-empty">
                <p>还没有打开知识库。</p>
                <button type="button" className="btn-accent" onClick={() => void chooseVault()}>
                  打开知识库…
                </button>
              </div>
            )
          ) : (
            <ToolSkillsPage
              vaultId={vaultId}
              groups={scan?.tools ?? []}
              tools={tools}
              onChanged={() => {
                if (vaultId) void useSkillsStore.getState().loadScan(vaultId);
              }}
              onResult={(lines) => setResultLines(lines)}
              onManageTargets={() => {
                setInstallOpen(false);
                setUpdatePath(null);
                setRestoreOpen(false);
                setTargetsOpen(true);
              }}
              onOpenRestore={() => {
                setInstallOpen(false);
                setUpdatePath(null);
                setTargetsOpen(false);
                setRestoreOpen(true);
              }}
            />
          )}
        </div>

        {distributePaths && vaultId ? (
          <DistributeDialog
            vaultId={vaultId}
            skills={(scan?.kb ?? [])
              .flatMap((group) => group.skills)
              .filter((skill) => distributePaths.includes(skill.path))}
            tools={tools}
            onClose={() => setDistributePaths(null)}
            onResult={(lines) => setResultLines(lines)}
            onDone={() => {
              void useSkillsStore.getState().loadScan(vaultId);
            }}
          />
        ) : null}

        <footer className="skills-foot">
          <span data-testid="skills-status">
            {scanning ? "后台扫描中…" : scan ? `上次扫描 ${scan.cachedAt}` : "尚未扫描"}
          </span>
          <span className="spacer" />
          {updateInfo && updateInfo.unchecked > 0 ? (
            <span className="skills-foot-warn" data-testid="skills-unchecked">
              {updateInfo.unchecked} 个来源这次没查
            </span>
          ) : null}
          {updateInfo?.rateLimitResetAt ? (
            <span className="skills-foot-warn" data-testid="skills-rate-limit">
              额度用完，{clockOf(updateInfo.rateLimitResetAt)} 后再试
            </span>
          ) : null}
          <span data-testid="skills-checked">
            {updateInfo?.checking ? "检查中…" : checkedLabel(updateInfo?.checkedAt) || "尚未检查"}
          </span>
          <button
            type="button"
            className="btn-quiet"
            disabled={updateInfo?.checking ?? false}
            onClick={() => void refreshUpdates(vaultId, true)}
            data-testid="skills-check-updates"
          >
            检查更新
          </button>
        </footer>
      </div>
    </div>
  );
}
