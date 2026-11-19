import { useEffect } from "react";
import { Minus, Square, X, Copy as RestoreIcon } from "@phosphor-icons/react";
import { useAppStore, type ThemeMode } from "../stores/appStore";
import { useWindowStore } from "../stores/windowStore";
import { windowApi } from "../lib/transport";
import appIcon from "../../resources/icon.png";
import { AccentPicker } from "./AccentPicker";

const THEME_LABEL: Record<ThemeMode, string> = {
  light: "浅色",
  dark: "深色",
  system: "系统",
};

const THEME_ORDER: ThemeMode[] = ["light", "dark", "system"];

/** 40px self-drawn title bar (DESIGN.md §4). The whole strip drags; every
 * interactive element carries .no-drag or it would be swallowed by the drag
 * region. */
export function TitleBar() {
  const theme = useAppStore((state) => state.settings?.theme ?? "system");
  const setTheme = useAppStore((state) => state.setTheme);
  const activeVaultId = useAppStore((state) => state.activeVaultId);
  const vaults = useAppStore((state) => state.vaults);
  const maximized = useWindowStore((state) => state.maximized);
  const subscribeWindow = useWindowStore((state) => state.subscribe);

  useEffect(() => subscribeWindow(), [subscribeWindow]);

  const active = vaults.find((vault) => vault.id === activeVaultId);

  return (
    <header
      className="drag-region flex items-center gap-3"
      style={{ height: "var(--titlebar-h)", paddingLeft: 12, paddingRight: 0, background: "var(--raw-canvas)" }}
    >
      <div className="flex min-w-0 items-center gap-2">
        <img src={appIcon} alt="" aria-hidden width={18} height={18} style={{ flex: "none", display: "block" }} />
        <span className="text-body font-semibold" style={{ color: "var(--raw-text)" }}>
          Mile Knowledge Base
        </span>
        <span
          className="hidden text-caption uppercase sm:inline"
          style={{ fontFamily: "var(--font-mono)", letterSpacing: "0.32em", color: "var(--raw-faint)", marginTop: 1 }}
        >
          VAULT
        </span>
      </div>

      <div className="min-w-0 flex-1 truncate text-body-sm" style={{ color: "var(--raw-muted)" }}>
        {active ? active.displayName : "未选择知识库"}
      </div>

      <AccentPicker />

      <div
        className="segmented no-drag"
        role="tablist"
        aria-label="主题"
        style={{ flex: "none" }}
      >
        {THEME_ORDER.map((mode) => (
          <button
            key={mode}
            role="tab"
            aria-selected={theme === mode}
            className="segmented-item"
            onClick={() => void setTheme(mode)}
          >
            {THEME_LABEL[mode]}
          </button>
        ))}
      </div>

      <div className="no-drag flex items-stretch" style={{ flex: "none", marginLeft: 8 }}>
        <button className="icon-btn window-btn" aria-label="最小化" onClick={() => void windowApi.minimize()}>
          <Minus size={14} aria-hidden />
        </button>
        <button
          className="icon-btn window-btn"
          aria-label={maximized ? "还原" : "最大化"}
          onClick={() => void windowApi.toggleMaximize()}
        >
          {maximized ? <RestoreIcon size={13} aria-hidden /> : <Square size={12} aria-hidden />}
        </button>
        <button className="icon-btn window-btn window-btn-close" aria-label="关闭" onClick={() => void windowApi.close()}>
          <X size={14} aria-hidden />
        </button>
      </div>
    </header>
  );
}
