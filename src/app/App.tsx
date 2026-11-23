import { useEffect } from "react";
import { Workbench } from "../components/Workbench";
import { SettingsPage } from "../components/settings/SettingsPage";
import { WelcomeView } from "../components/WelcomeView";
import { NoticeStack } from "../components/NoticeStack";
import { SkillImportWizard } from "../components/SkillImportWizard";
import { SkillsPanel } from "../components/skills/SkillsPanel";
import { TitleBar } from "../components/TitleBar";
import { useAppStore } from "../stores/appStore";

export function App() {
  const booted = useAppStore((state) => state.booted);
  const bootError = useAppStore((state) => state.bootError);
  const activeVaultId = useAppStore((state) => state.activeVaultId);
  const settingsOpen = useAppStore((state) => state.settingsOpen);
  const page = useAppStore((state) => state.page);
  const boot = useAppStore((state) => state.boot);

  useEffect(() => {
    void boot();
  }, [boot]);

  if (!booted) {
    return <div className="boot muted">正在启动…</div>;
  }
  if (bootError) {
    return (
      <div className="boot">
        <div className="surface-card" data-tone="fault" role="alert" style={{ maxWidth: 520 }}>
          <div className="status-card-head">启动失败</div>
          <div className="status-card-body">{bootError}</div>
        </div>
      </div>
    );
  }
  // D-006: with every registered vault unavailable there is no active vault,
  // so the welcome page (with fault cards) shows instead of an empty shell.
  // M8 T01: the title bar lives here once; the workbench stays mounted (and
  // hidden) while the skills page is showing, so streams, drafts and scroll
  // positions survive the trip (PRD AC-PAGE-1).
  const workbenchVisible = page === "work";
  return (
    <>
      <div className="app-shell" style={{ gridTemplateRows: "var(--titlebar-h) minmax(0, 1fr)" }}>
        <TitleBar />
        {settingsOpen ? (
          <SettingsPage />
        ) : activeVaultId ? (
          <div
            className="page-host"
            data-testid="workbench-host"
            hidden={!workbenchVisible}
            inert={workbenchVisible ? undefined : true}
          >
            <Workbench />
          </div>
        ) : (
          <WelcomeView />
        )}
      </div>
      <SkillsPanel />
      <SkillImportWizard />
      <NoticeStack />
    </>
  );
}
