import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { events } from "../../mygo";
import { skillsApi, type KbSkill, type SkillRestorePreview, type SkillUpdateCheckResult } from "../../lib/transport";
import { useAppStore } from "../../stores/appStore";
import { useDocStore } from "../../stores/docStore";
import { useMutationStore } from "../../stores/mutationStore";
import { useSkillsStore } from "../../stores/skillsStore";
import { useTreeStore } from "../../stores/treeStore";
import { DistributeDialog } from "./DistributeDialog";
import { InstallView } from "./InstallView";
import { KbSkillsPage } from "./KbSkillsPage";
import { RestoreView } from "./RestoreView";
import { SkillDetailDrawer } from "./SkillDetailDrawer";
import { SkillsNav, type SkillsSelection } from "./SkillsNav";
import { TargetsView } from "./TargetsView";
import { ToolSkillsPage } from "./ToolSkillsPage";
import { UpdateView } from "./UpdateView";

type PendingUpdate = { vaultId: string; stageId: string; planId: string; name: string };

// The entrance animation is ~1.2 s for 24 staggered cards (520ms + 24 × 38ms
// plus a small settle margin); non-scan requests wait for it (PLAN-M10.1 R2).
const ENTER_SETTLE_MS = 1500;

function subscribeSkillsChanged(listener: () => void): () => void {
  try {
    return events.skillsChanged.on(listener);
  } catch {
    return () => undefined;
  }
}

/** The standalone skills page (M8 T03 shell, T04 knowledge-base view). It is
 * mounted only while `page === "skills"`, so every entry starts fresh and the
 * scan is refreshed then. Tool views and the full-page subview shells land in
 * T05–T07; the existing dialogs are reused unchanged until then. */
export function SkillsPage() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const showPage = useAppStore((state) => state.showPage);
  const chooseVault = useAppStore((state) => state.chooseVault);
  const setLayout = useAppStore((state) => state.setLayout);
  const kbCollapsed = useAppStore((state) => state.settings?.layout?.skillsKbCollapsed ?? false);
  const toolsCollapsed = useAppStore((state) => state.settings?.layout?.skillsToolsCollapsed ?? false);
  const pushNotice = useAppStore((state) => state.pushNotice);
  const scan = useSkillsStore((state) => state.scan);
  const tools = useSkillsStore((state) => state.tools);
  const [selection, setSelection] = useState<SkillsSelection>({ kind: "kb", category: null });
  const [distributePaths, setDistributePaths] = useState<string[] | null>(null);
  // Set by a batch distribute; run once the batch finished so selection ends.
  const batchFinished = useRef<(() => void) | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  // The open detail drawer (M9 T04): the path picks the skill; the last scan
  // entry is kept so a rescan that drops it does not blank the drawer.
  const [detailPath, setDetailPath] = useState<string | null>(null);
  const detailSkillRef = useRef<KbSkill | null>(null);
  const [updatePath, setUpdatePath] = useState<string | null>(null);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restore, setRestore] = useState<SkillRestorePreview | null>(null);
  const [resultLines, setResultLines] = useState<string[] | null>(null);
  const [pendingUpdate, setPendingUpdate] = useState<PendingUpdate | null>(null);
  const [updateInfo, setUpdateInfo] = useState<SkillUpdateCheckResult | null>(null);
  const shellRef = useRef<HTMLElement | null>(null);
  const pendingPlan = useMutationStore((state) => state.pending);
  const mutationRunning = useMutationStore((state) => state.running);
  const mutationReport = useMutationStore((state) => state.report);

  const refreshScan = () => {
    void useSkillsStore.getState().refresh(vaultId);
  };

  // Q-SKM-2: entering the page checks for updates (cached for 60 minutes).
  // The footer shows the result and offers a manual check.
  const refreshUpdates = async (force: boolean) => {
    if (!vaultId) return;
    try {
      setUpdateInfo(await skillsApi.checkUpdates(vaultId, force));
    } catch {
      // Keep the previous footer state when a check fails.
    }
  };

  useEffect(() => {
    refreshScan();
    setDetailPath(null);
    // The entrance animation comes first: the update check and the restore
    // preview are deferred so their extra renders cannot interrupt it
    // (PLAN-M10.1 R2). The scan itself starts at once, it is needed to paint.
    const defer = window.setTimeout(() => {
      void refreshUpdates(false);
      setRestoreEnter(true);
    }, ENTER_SETTLE_MS);
    // A vault switch can drop the selected category or tool; fall back to all.
    setSelection((current) =>
      current.kind === "kb" && current.category ? { kind: "kb", category: null } : current,
    );
    return subscribeSkillsChanged(() => {
      refreshScan();
      void refreshUpdates(false);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vaultId]);

  // The settings entry (and the panel's transitional entry) asks for the
  // target manager; the request is consumed so a later visit starts on the list.
  const targetsRequest = useAppStore((state) => state.targetsRequest);
  useEffect(() => {
    if (targetsRequest === 0) return;
    setSelection({ kind: "targets" });
    useAppStore.getState().consumeTargetsRequest();
  }, [targetsRequest]);

  useEffect(() => {
    if (!scan) return;
    setSelection((current) => {
      if (current.kind === "kb" && current.category && !scan.kb.some((group) => group.category === current.category)) {
        return { kind: "kb", category: null };
      }
      if (current.kind === "tools" && current.toolId && !scan.tools.some((group) => group.toolId === current.toolId)) {
        return { kind: "tools", toolId: null };
      }
      return current;
    });
  }, [scan]);

  const [restoreEnter, setRestoreEnter] = useState(false);
  // Manifest banner (M7; UI 稿 11.2 puts it at the top of the page content).
  // Fires only after the entrance settled, and reloads only when the scan
  // payload itself changes identity (the merge in skillsStore keeps it stable
  // for unchanged scans, so the entrance is no longer re-interrupted).
  useEffect(() => {
    if (!vaultId || !restoreEnter) {
      setRestore(null);
      return;
    }
    let alive = true;
    void skillsApi
      .restorePreview(vaultId)
      .then((value) => {
        if (alive) setRestore(value.preview ?? null);
      })
      .catch(() => {
        if (alive) setRestore(null);
      });
    return () => {
      alive = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vaultId, restoreEnter, scan]);

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
        if (target.vaultId) void useSkillsStore.getState().loadScan(target.vaultId);
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

  // Entering the page puts focus on the current navigation item (UI 稿 11.1);
  // the page unmounts on leave, so mount is the right moment.
  useLayoutEffect(() => {
    shellRef.current?.querySelector<HTMLElement>('[aria-current="page"]')?.focus();
  }, []);

  const updateSkillName = useMemo(() => {
    if (!updatePath) return "";
    for (const group of scan?.kb ?? []) {
      for (const skill of group.skills) if (skill.path === updatePath) return skill.name;
    }
    return updatePath.split("/").pop() ?? updatePath;
  }, [scan, updatePath]);

  const updateRemote = useMemo(() => {
    if (!updatePath) return undefined;
    for (const group of scan?.kb ?? []) {
      for (const skill of group.skills) if (skill.path === updatePath) return skill.remote;
    }
    return undefined;
  }, [scan, updatePath]);

  const detailSkill = useMemo(() => {
    if (!detailPath) return null;
    for (const group of scan?.kb ?? []) {
      for (const skill of group.skills) if (skill.path === detailPath) return skill;
    }
    return null;
  }, [scan, detailPath]);
  if (detailPath === null) detailSkillRef.current = null;
  else if (detailSkill) detailSkillRef.current = detailSkill;
  const shownDetail = detailPath ? (detailSkill ?? (detailSkillRef.current?.path === detailPath ? detailSkillRef.current : null)) : null;

  // Closing from the keyboard or the button hands focus back to the card.
  const closeDetail = () => {
    const path = detailPath;
    setDetailPath(null);
    if (!path) return;
    requestAnimationFrame(() => {
      const cards = shellRef.current?.querySelectorAll<HTMLElement>("[data-testid^='kb-card-']") ?? [];
      for (const card of cards) {
        if (card.dataset.path === path) {
          card.focus();
          return;
        }
      }
    });
  };

  const groups = selection.kind === "kb" && selection.category
    ? (scan?.kb ?? []).filter((group) => group.category === selection.category)
    : (scan?.kb ?? []);

  const openSkill = useCallback(
    (path: string) => {
      showPage("work");
      if (vaultId) void useDocStore.getState().open(vaultId, `${path}/SKILL.md`);
    },
    [showPage, vaultId],
  );

  const locateSkill = useCallback(
    (path: string) => {
      showPage("work");
      void useTreeStore.getState().reveal(path);
    },
    [showPage],
  );

  const restoreBanner = (() => {
    if (!restore) return null;
    if (restore.status === "ok" && restore.pending > 0) {
      return (
        <div className="restore-banner" data-testid="restore-banner">
          <span className="restore-banner-text">知识库清单里有 {restore.pending} 项尚未在本机还原</span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={() => setRestoreOpen(true)} data-testid="restore-banner-open">
            查看并还原
          </button>
        </div>
      );
    }
    if (restore.status === "unreadable" || restore.status === "too-large") {
      return (
        <div className="restore-banner" data-tone="warn" data-testid="restore-banner">
          <span className="restore-banner-text">分发清单无法读取，之后的分发不会写入清单</span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={() => setRestoreOpen(true)} data-testid="restore-banner-open">
            用本机记录重建
          </button>
        </div>
      );
    }
    if (restore.status === "version-unknown") {
      return (
        <div className="restore-banner" data-tone="warn" data-testid="restore-banner">
          <span className="restore-banner-text">分发清单来自更新版本的应用，请升级后再还原</span>
        </div>
      );
    }
    return null;
  })();

  return (
    <section className="skills-page" data-testid="skills-page" ref={shellRef}>
      <SkillsNav
        selection={selection}
        onSelect={(next) => {
          setDetailPath(null);
          setDistributePaths(null);
          setInstallOpen(false);
          setUpdatePath(null);
          setRestoreOpen(false);
          setSelection(next);
        }}
        scan={scan}
        tools={tools}
        hasVault={Boolean(vaultId)}
        collapsed={{ kb: kbCollapsed, tools: toolsCollapsed }}
        onSetCollapsed={(group, value) => {
          void setLayout(group === "kb" ? { skillsKbCollapsed: value } : { skillsToolsCollapsed: value }).catch(() => undefined);
        }}
        update={updateInfo}
        onCheckUpdates={() => void refreshUpdates(true)}
      />

      <main className="skills-main" data-testid="skills-main" aria-label="技能内容">
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
              if (!vaultId) return;
              setPendingUpdate({ vaultId, stageId, planId: plan.id, name: updateSkillName });
              useMutationStore.getState().propose(plan, `更新技能「${updateSkillName}」`);
              setUpdatePath(null);
              pushNotice("已生成变更计划，回到工作台确认");
            }}
          />
        ) : restoreOpen && vaultId ? (
          <RestoreView
            vaultId={vaultId}
            onBack={() => setRestoreOpen(false)}
            onChanged={() => refreshScan()}
            onResult={(lines) => setResultLines(lines)}
          />
        ) : (
          <>
            {restoreBanner}
            {selection.kind === "kb" ? (
              vaultId ? (
                <KbSkillsPage
                  groups={groups}
                  tools={tools}
                  scanning={scan?.scanning ?? false}
                  onOpenSkill={openSkill}
                  onLocate={locateSkill}
                  onDistribute={(paths, onFinished) => {
                    batchFinished.current = onFinished ?? null;
                    setDistributePaths(paths);
                  }}
                  onInstall={() => {
                    setUpdatePath(null);
                    setRestoreOpen(false);
                    setInstallOpen(true);
                  }}
                  onUpdate={(skill: KbSkill) => {
                    setInstallOpen(false);
                    setRestoreOpen(false);
                    setUpdatePath(skill.path);
                  }}
                  onOpenDetail={setDetailPath}
                  activePath={detailPath}
                />
              ) : (
                <div className="skills-empty" data-testid="skills-no-vault">
                  <p>还没有打开知识库。</p>
                  <button type="button" className="btn-accent" onClick={() => void chooseVault()}>
                    打开知识库…
                  </button>
                </div>
              )
            ) : selection.kind === "targets" ? (
              vaultId ? (
                <TargetsView
                  vaultId={vaultId}
                  onBack={() => setSelection({ kind: "tools", toolId: null })}
                  onChanged={refreshScan}
                  onResult={(lines) => setResultLines(lines)}
                />
              ) : (
                <div className="skills-empty" data-testid="skills-no-vault">
                  <p>还没有打开知识库。</p>
                  <button type="button" className="btn-accent" onClick={() => void chooseVault()}>
                    打开知识库…
                  </button>
                </div>
              )
            ) : selection.kind === "tools" ? (
              <ToolSkillsPage
                vaultId={vaultId}
                groups={
                  selection.toolId
                    ? (scan?.tools ?? []).filter((group) => group.toolId === selection.toolId)
                    : (scan?.tools ?? [])
                }
                tools={tools}
                title={
                  selection.kind === "tools" && selection.toolId
                    ? (tools.find((tool) => tool.id === selection.toolId)?.name ?? selection.toolId)
                    : "全部工具"
                }
                onChanged={refreshScan}
                onResult={(lines) => setResultLines(lines)}
                onInstall={vaultId ? () => {
                  setDistributePaths(null);
                  setUpdatePath(null);
                  setRestoreOpen(false);
                  setInstallOpen(true);
                } : undefined}
              />
            ) : null}
          </>
        )}

        {shownDetail && vaultId && selection.kind === "kb" && !updatePath && !restoreOpen ? (
          <SkillDetailDrawer
            vaultId={vaultId}
            skill={shownDetail}
            tools={tools}
            onClose={closeDetail}
            onDistribute={() => setDistributePaths([shownDetail.path])}
            onUpdate={() => {
              setInstallOpen(false);
              setRestoreOpen(false);
              setUpdatePath(shownDetail.path);
            }}
            onEdit={() => openSkill(shownDetail.path)}
            onLocate={() => locateSkill(shownDetail.path)}
            onRescan={refreshScan}
          />
        ) : null}
      </main>

      {distributePaths && vaultId ? (
        <DistributeDialog
          vaultId={vaultId}
          skills={(scan?.kb ?? [])
            .flatMap((group) => group.skills)
            .filter((skill) => distributePaths.includes(skill.path))}
          tools={tools}
          onClose={() => {
            batchFinished.current = null;
            setDistributePaths(null);
          }}
          onDone={(succeeded) => {
            void useSkillsStore.getState().loadScan(vaultId);
            if (succeeded) batchFinished.current?.();
          }}
        />
      ) : null}

      {installOpen && vaultId ? (
        <InstallView
          vaultId={vaultId}
          tools={tools}
          onBack={() => setInstallOpen(false)}
          onChanged={() => void useSkillsStore.getState().loadScan(vaultId)}
        />
      ) : null}
    </section>
  );
}
