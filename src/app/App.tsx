import { useEffect } from "react";
import { Workbench } from "../components/Workbench";
import { useAppStore } from "../stores/appStore";

export function App() {
  const booted = useAppStore((state) => state.booted);
  const bootError = useAppStore((state) => state.bootError);
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
  return <Workbench />;
}
