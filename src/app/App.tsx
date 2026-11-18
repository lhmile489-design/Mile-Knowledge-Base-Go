import { useEffect } from "react";
import { Workbench } from "../components/Workbench";
import { SettingsPage } from "../components/settings/SettingsPage";
import { WelcomeView } from "../components/WelcomeView";
import { NoticeStack } from "../components/NoticeStack";
import { useAppStore } from "../stores/appStore";

export function App() {
  const booted = useAppStore((state) => state.booted);
  const bootError = useAppStore((state) => state.bootError);
  const activeVaultId = useAppStore((state) => state.activeVaultId);
  const settingsOpen = useAppStore((state) => state.settingsOpen);
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
  return (
    <>
      {settingsOpen ? <SettingsPage /> : activeVaultId ? <Workbench /> : <WelcomeView />}
      <NoticeStack />
    </>
  );
}
