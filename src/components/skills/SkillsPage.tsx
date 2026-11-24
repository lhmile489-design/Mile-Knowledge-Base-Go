import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { events } from "../../mygo";
import { skillsApi, type KbSkill, type SkillRestorePreview } from "../../lib/transport";
import { useAppStore } from "../../stores/appStore";
import { useDocStore } from "../../stores/docStore";
import { useMutationStore } from "../../stores/mutationStore";
import { useSkillsStore } from "../../stores/skillsStore";
import { useTreeStore } from "../../stores/treeStore";
import { DistributeDialog } from "./DistributeDialog";
import { InstallView } from "./InstallView";
import { KbSkillsPage } from "./KbSkillsPage";
import { RestoreView } from "./RestoreView";
import { SkillsNav, type SkillsSelection } from "./SkillsNav";
import { UpdateView } from "./UpdateView";

type PendingUpdate = { vaultId: string; stageId: string; planId: string; name: string };

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
  const pushNotice = useAppStore((state) => state.pushNotice);
  const scan = useSkillsStore((state) => state.scan);
  const tools = useSkillsStore((state) => state.tools);
  const [selection, setSelection] = useState<SkillsSelection>({ kind: "kb", category: null });
  const [distributePaths, setDistributePaths] = useState<string[] | null>(null);
  const [installOpen, setInstallOpen] = useState(false);
  const [updatePath, setUpdatePath] = useState<string | null>(null);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restore, setRestore] = useState<SkillRestorePreview | null>(null);
  const [resultLines, setResultLines] = useState<string[] | null>(null);
  const [pendingUpdate, setPendingUpdate] = useState<PendingUpdate | null>(null);
  const shellRef = useRef<HTMLElement | null>(null);
  const pendingPlan = useMutationStore((state) => state.pending);
  const mutationRunning = useMutationStore((state) => state.running);
  const mutationReport = useMutationStore((state) => state.report);

  const refreshScan = () => {
    void useSkillsStore.getState().refresh(vaultId);
  };

  useEffect(() => {
    refreshScan();
    // A vault switch can drop the selected category or tool; fall back to all.
    setSelection((current) =>
      current.kind === "kb" && current.category ? { kind: "kb", category: null } : current,
    );
    return subscribeSkillsChanged(() => refreshScan());
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vaultId]);

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

  // Manifest banner (M7; UI 稿 11.2 puts it at the top of the page content).
  useEffect(() => {
    if (!vaultId) {
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
  }, [vaultId, scan?.cachedAt]);

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

  const groups = selection.kind === "kb" && selection.category
    ? (scan?.kb ?? []).filter((group) => group.category === selection.category)
    : (scan?.kb ?? []);

  const openSkill = (path: string) => {
    showPage("work");
    if (vaultId) void useDocStore.getState().open(vaultId, `${path}/SKILL.md`);
  };

  const locateSkill = (path: string) => {
    showPage("work");
    void useTreeStore.getState().reveal(path);
  };

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
          setInstallOpen(false);
          setUpdatePath(null);
          setRestoreOpen(false);
          setSelection(next);
        }}
        scan={scan}
        tools={tools}
        hasVault={Boolean(vaultId)}
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

        {installOpen && vaultId ? (
          <InstallView
            vaultId={vaultId}
            tools={tools}
            onBack={() => setInstallOpen(false)}
            onChanged={() => void useSkillsStore.getState().loadScan(vaultId)}
          />
        ) : updatePath && vaultId ? (
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
                  onOpenSkill={openSkill}
                  onLocate={locateSkill}
                  onDistribute={(paths) => setDistributePaths(paths)}
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
                />
              ) : (
                <div className="skills-empty" data-testid="skills-no-vault">
                  <p>还没有打开知识库。</p>
                  <button type="button" className="btn-accent" onClick={() => void chooseVault()}>
                    打开知识库…
                  </button>
                </div>
              )
            ) : null}
          </>
        )}
      </main>

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
    </section>
  );
}
