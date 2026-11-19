import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Faders, Folders, GearSix, Info, UsersThree } from "@phosphor-icons/react";
import type { ProvidersResult } from "../../mygo";
import { settingsApi, TransportError } from "../../lib/transport";
import { dirtyDraftIds, draftLabel } from "../../lib/settingsDrafts";
import { useAppStore } from "../../stores/appStore";
import { useSettingsStore, type SettingsSection } from "../../stores/settingsStore";
import { TitleBar } from "../TitleBar";
import { ModelServiceSection } from "./ModelServiceSection";
import { AgentsSection } from "./AgentsSection";
import { CategoriesSection } from "./CategoriesSection";
import { GeneralSection } from "./GeneralSection";
import { SettingsSaveBar } from "./SettingsSaveBar";

interface SectionMeta {
  readonly id: SettingsSection;
  readonly group: string;
  readonly label: string;
  readonly scope: string;
  readonly needsVault: boolean;
  readonly icon: typeof GearSix;
}

/** Four nav groups, five sections (PRD FR-SET). */
const SECTIONS: readonly SectionMeta[] = [
  { id: "general", group: "应用", label: "常规", scope: "保存在这台电脑", needsVault: false, icon: GearSix },
  { id: "model", group: "AI", label: "模型服务", scope: "随当前知识库", needsVault: true, icon: Faders },
  { id: "agents", group: "AI", label: "Agents", scope: "随当前知识库", needsVault: true, icon: UsersThree },
  { id: "categories", group: "知识库", label: "分类与检索", scope: "随当前知识库", needsVault: true, icon: Folders },
  { id: "about", group: "其他", label: "关于与诊断", scope: "只读", needsVault: false, icon: Info },
];

const NAV_GROUPS = ["应用", "AI", "知识库", "其他"] as const;

/** Settings overlay (M2 T17/T18; M3 T04 = shell + page-level save bar). */
export function SettingsPage() {
  const closeSettings = useAppStore((state) => state.closeSettings);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const vaults = useAppStore((state) => state.vaults);
  const vault = vaults.find((item) => item.id === vaultId);
  const [section, setSection] = useState<SettingsSection>("general");
  const requestLeave = useSettingsStore((state) => state.requestLeave);
  const resolveLeave = useSettingsStore((state) => state.resolveLeave);
  const leaveTarget = useSettingsStore((state) => state.leaveTarget);
  const drafts = useSettingsStore((state) => state.drafts);
  const config = useSettingsStore((state) => state.config);
  const saving = useSettingsStore((state) => state.saving);
  const error = useSettingsStore((state) => state.error);
  const notice = useSettingsStore((state) => state.notice);
  const clearNotice = useSettingsStore((state) => state.clearNotice);
  const loadConfig = useSettingsStore((state) => state.load);
  const discardDrafts = useSettingsStore((state) => state.discardDrafts);
  const saveDrafts = useSettingsStore((state) => state.saveDrafts);
  const [data, setData] = useState<ProvidersResult | null>(null);
  const [providersError, setProvidersError] = useState<string | null>(null);
  const navRefs = useRef<Record<string, HTMLButtonElement | null>>({});

  const reload = useCallback(async () => {
    if (!vaultId) return;
    try {
      const result = await settingsApi.providers(vaultId);
      setData(result);
      setProvidersError(null);
    } catch (err) {
      setProvidersError(err instanceof TransportError ? err.message : String(err));
    }
  }, [vaultId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  // The whole settings page shares one config load (steering + categories).
  useEffect(() => {
    if (vaultId) void loadConfig(vaultId);
  }, [vaultId, loadConfig]);

  // 「已保存」两秒后自动消失（旧版 2000ms）。
  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => clearNotice(), 2000);
    return () => window.clearTimeout(timer);
  }, [notice, clearNotice]);

  const dirty = useMemo(() => dirtyDraftIds(drafts, config), [drafts, config]);
  const labels = useMemo(() => dirty.map((id) => draftLabel(id, config)), [dirty, config]);
  const current = SECTIONS.find((item) => item.id === section) ?? SECTIONS[0];

  const go = useCallback(
    (target: SettingsSection) => {
      if (target === section) return;
      if (!requestLeave(target)) return;
      setSection(target);
    },
    [section, requestLeave],
  );

  const guardedClose = useCallback(() => {
    if (requestLeave("close")) closeSettings();
  }, [closeSettings, requestLeave]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") {
        if (leaveTarget) {
          // 对话框打开时 Esc 等于「继续编辑」。
          event.preventDefault();
          resolveLeave(false);
          navRefs.current[section]?.focus();
          return;
        }
        if (requestLeave("close")) closeSettings();
        return;
      }
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
        // 阻止冒泡，避免把背后的文档编辑器也一起保存了（旧版同款处理）。
        event.preventDefault();
        event.stopPropagation();
        if (vaultId && dirty.length > 0) void saveDrafts(vaultId);
      }
    };
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [closeSettings, requestLeave, resolveLeave, saveDrafts, vaultId, dirty.length, leaveTarget, section]);

  const onNavKeyDown = (event: React.KeyboardEvent<HTMLElement>): void => {
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const target = event.target as HTMLElement;
    if (!target.closest("[data-section-nav]")) return;
    event.preventDefault();
    const index = SECTIONS.findIndex((item) => item.id === section);
    const next =
      event.key === "ArrowDown"
        ? SECTIONS[(index + 1) % SECTIONS.length]
        : SECTIONS[(index - 1 + SECTIONS.length) % SECTIONS.length];
    if (!next) return;
    go(next.id);
    window.requestAnimationFrame(() => navRefs.current[next.id]?.focus());
  };

  const noVaultNotice = "打开一个知识库后才能配置模型服务、Agent 与分类。常规设置不受影响。";
  const unconfigured = (data?.providers.length ?? 0) === 0;
  const sectionNeedsVault = current?.needsVault ?? false;

  return (
    <div className="app-shell settings-shell" style={{ gridTemplateRows: "var(--titlebar-h) minmax(0, 1fr)" }}>
      <TitleBar />
      <div className="settings-grid">
        <aside
          className="workbench-panel"
          data-panel="settings-nav"
          aria-label="设置导航"
          data-section-nav
          inert={leaveTarget !== null}
        >
          <div className="settings-nav-head">
            <GearSix size={14} aria-hidden />
            设置
          </div>
          {NAV_GROUPS.map((group) => {
            const items = SECTIONS.filter((item) => item.group === group);
            if (items.length === 0) return null;
            return (
              <div key={group}>
                <div className="settings-nav-group">{group}</div>
                {items.map((item) => {
                  const Icon = item.icon;
                  const active = section === item.id;
                  return (
                    <button
                      key={item.id}
                      ref={(node) => {
                        navRefs.current[item.id] = node;
                      }}
                      type="button"
                      className="settings-nav-item"
                      data-active={active ? "true" : "false"}
                      aria-current={active ? "page" : undefined}
                      onClick={() => go(item.id)}
                      onKeyDown={onNavKeyDown}
                      data-testid={`settings-nav-${item.id}`}
                    >
                      <Icon size={14} aria-hidden />
                      {item.label}
                      {item.id === "model" && unconfigured ? (
                        <span className="pill" data-tone="warn">
                          未配置
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            );
          })}
          <div className="settings-nav-spacer" />
          <div className="settings-nav-foot">
            <span>当前知识库</span>
            <strong>{vault?.displayName ?? "—"}</strong>
          </div>
        </aside>

        <section className="workbench-panel settings-content-host" data-panel="settings-content" aria-label="设置内容" inert={leaveTarget !== null}>
          {notice ? (
            <div className="settings-notice" role="status" data-testid="settings-notice">
              {notice}
            </div>
          ) : null}
          {sectionNeedsVault && !vaultId ? (
            <div className="settings-content">
              <header className="settings-content-head">
                <div>
                  <h1 className="settings-title">{current?.label}</h1>
                  <div className="settings-chips">
                    <span className="settings-chip">{current?.scope}</span>
                  </div>
                </div>
              </header>
              <div className="settings-body">
                <p className="settings-no-vault" data-testid="settings-no-vault">
                  {noVaultNotice}
                </p>
              </div>
            </div>
          ) : section === "categories" ? (
            <CategoriesSection onClose={guardedClose} />
          ) : section === "general" ? (
            <GeneralSection onClose={guardedClose} />
          ) : section === "model" ? (
            <ModelServiceSection vaultId={vaultId} data={data} error={providersError} reload={reload} onClose={guardedClose} />
          ) : section === "agents" ? (
            <AgentsSection vaultId={vaultId ?? ""} onClose={guardedClose} />
          ) : (
            <SectionPlaceholder section={current} />
          )}

          <SettingsSaveBar
            labels={labels}
            error={error}
            saving={saving}
            onDiscard={discardDrafts}
            onSave={() => void saveDrafts(vaultId)}
          />
        </section>
      </div>

      {leaveTarget ? (
        <div className="settings-dialog-backdrop" role="presentation">
          <div
            className="settings-dialog settings-dialog-narrow"
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="settings-leave-title"
            data-testid="settings-leave-dialog"
          >
            <div className="settings-dialog-head">
              <strong id="settings-leave-title">有 {labels.length} 处未保存的改动</strong>
            </div>
            <p className="field-hint">
              {labels.join("、")}。关闭设置或切换分节会丢弃它们。
            </p>
            <div className="settings-dialog-actions">
              <button
                type="button"
                className="btn-quiet"
                onClick={() => {
                  resolveLeave(false);
                  navRefs.current[section]?.focus();
                }}
                data-testid="settings-leave-stay"
              >
                继续编辑
              </button>
              <button
                type="button"
                className="btn-primary"
                onClick={() => {
                  const target = resolveLeave(true);
                  if (target === "close") {
                    closeSettings();
                    return;
                  }
                  if (target) {
                    setSection(target);
                    window.requestAnimationFrame(() => navRefs.current[target]?.focus());
                  }
                }}
                data-testid="settings-leave-discard"
              >
                放弃改动
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

/** Placeholder until T05/T06/T07 ship the section itself. */
function SectionPlaceholder({ section }: { readonly section?: SectionMeta }) {
  if (!section) return null;
  return (
    <div className="settings-content" data-testid={`settings-placeholder-${section.id}`}>
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">{section.label}</h1>
          <div className="settings-chips">
            <span className="settings-chip">{section.scope}</span>
          </div>
        </div>
      </header>
      <div className="settings-body">
        <p className="settings-no-vault">该分节将随 M3 的后续任务交付（见 docs/PLAN-M3.md）。</p>
      </div>
    </div>
  );
}
