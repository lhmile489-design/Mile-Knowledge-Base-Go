import { useCallback, useEffect, useState } from "react";
import { Faders, GearSix, UsersThree } from "@phosphor-icons/react";
import type { ProvidersResult } from "../../mygo";
import { settingsApi, TransportError } from "../../lib/transport";
import { useAppStore } from "../../stores/appStore";
import { TitleBar } from "../TitleBar";
import { ModelServiceSection } from "./ModelServiceSection";

/** Settings overlay (T17/T18). Only the AI sections exist in M2; the other
 * settings groups arrive with their own milestones. */
export function SettingsPage() {
  const closeSettings = useAppStore((state) => state.closeSettings);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const vaults = useAppStore((state) => state.vaults);
  const vault = vaults.find((item) => item.id === vaultId);
  const [section, setSection] = useState<"model" | "agents">("model");
  const [data, setData] = useState<ProvidersResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    if (!vaultId) return;
    try {
      const result = await settingsApi.providers(vaultId);
      setData(result);
      setError(null);
    } catch (err) {
      setError(err instanceof TransportError ? err.message : String(err));
    }
  }, [vaultId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") closeSettings();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [closeSettings]);

  const unconfigured = (data?.providers.length ?? 0) === 0;

  return (
    <div className="app-shell settings-shell" style={{ gridTemplateRows: "var(--titlebar-h) minmax(0, 1fr)" }}>
      <TitleBar />
      <div className="settings-grid">
        <aside className="workbench-panel" data-panel="settings-nav" aria-label="设置导航">
          <div className="settings-nav-head">
            <GearSix size={14} aria-hidden />
            设置
          </div>
          <div className="settings-nav-group">AI</div>
          <button
            type="button"
            className="settings-nav-item"
            data-active={section === "model" ? "true" : "false"}
            aria-current={section === "model" ? "page" : undefined}
            onClick={() => setSection("model")}
            data-testid="settings-nav-model"
          >
            <Faders size={14} aria-hidden />
            模型服务
            {unconfigured ? (
              <span className="pill" data-tone="warn">
                未配置
              </span>
            ) : null}
          </button>
          <button
            type="button"
            className="settings-nav-item"
            data-active={section === "agents" ? "true" : "false"}
            disabled
            aria-disabled="true"
            title="Agents"
          >
            <UsersThree size={14} aria-hidden />
            Agents
          </button>
          <div className="settings-nav-spacer" />
          <div className="settings-nav-foot">
            <span>当前知识库</span>
            <strong>{vault?.displayName ?? "—"}</strong>
          </div>
        </aside>

        <section className="workbench-panel" data-panel="settings-content" aria-label="设置内容">
          {section === "model" ? (
            <ModelServiceSection vaultId={vaultId} data={data} error={error} reload={reload} onClose={closeSettings} />
          ) : null}
        </section>
      </div>
    </div>
  );
}
