import { useEffect, useRef, useState } from "react";
import { Plus, X } from "@phosphor-icons/react";
import { useAppStore } from "../../stores/appStore";
import { useSkillsStore } from "../../stores/skillsStore";
import { totals } from "../../lib/skillsView";
import { useDocStore } from "../../stores/docStore";
import { useTreeStore } from "../../stores/treeStore";
import { KbSkillsPage } from "./KbSkillsPage";
import { DistributeDialog } from "./DistributeDialog";

type Page = "kb" | "tools";

/** The skills panel shell (M4 T09; UI 稿 9.1–9.2). T10/T11 fill the two
 * pages; this task delivers the layer, the two tabs, the install entry, the
 * badge refresh, and the keyboard/focus rules. */
export function SkillsPanel() {
  const open = useAppStore((state) => state.skillsPanelOpen);
  const closePanel = useAppStore((state) => state.closeSkillsPanel);
  const openWizard = useAppStore((state) => state.openSkillWizard);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const chooseVault = useAppStore((state) => state.chooseVault);
  const scan = useSkillsStore((state) => state.scan);
  const tools = useSkillsStore((state) => state.tools);
  const [page, setPage] = useState<Page>("kb");
  const [distributePaths, setDistributePaths] = useState<string[] | null>(null);
  const [box, setBox] = useState<{ right: number; width: number; height: number } | null>(null);
  const panelRef = useRef<HTMLDivElement | null>(null);

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

  useEffect(() => {
    if (!open) return;
    const first = panelRef.current?.querySelector<HTMLElement>("[data-skills-tab]");
    first?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
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
                onClick={() => setPage(id)}
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
          <button type="button" className="btn-quiet skills-install" onClick={() => openWizard()}>
            <Plus size={13} aria-hidden />
            安装
          </button>
          <button type="button" className="icon-button" aria-label="关闭技能浮层" onClick={() => close()} data-testid="skills-close">
            <X size={14} aria-hidden />
          </button>
        </header>

        <div className="skills-body scroll-y" data-testid="skills-body">
          {page === "kb" ? (
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
                onInstall={() => openWizard()}
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
            <div className="skills-tools">
              {tools.filter((tool) => tool.enabled).map((tool) => {
                const group = scan?.tools.find((item) => item.toolId === tool.id);
                const count = group?.skills.length ?? 0;
                return (
                  <div key={tool.id} className="skills-tool-row" data-testid={`skills-tool-${tool.id}`}>
                    <span className="skills-tool-name">{tool.name}</span>
                    <span className="pill" data-tone={tool.status === "detected" ? "live" : "idle"}>
                      {tool.status === "detected"
                        ? "已检测到"
                        : tool.status === "leftover"
                          ? "未检测到（有残留目录）"
                          : "未检测到"}
                    </span>
                    <span className="skills-tool-count">{count} 个技能</span>
                  </div>
                );
              })}
            </div>
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
            onDone={() => {
              void useSkillsStore.getState().loadScan(vaultId);
            }}
          />
        ) : null}

        <footer className="skills-foot">
          <span data-testid="skills-status">
            {scanning ? "后台扫描中…" : scan ? `上次扫描 ${scan.cachedAt}` : "尚未扫描"}
          </span>
        </footer>
      </div>
    </div>
  );
}
