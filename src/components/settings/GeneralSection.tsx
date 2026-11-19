import { useEffect, useRef, useState } from "react";
import { Check, X } from "@phosphor-icons/react";
import { nextOption } from "../../lib/radioGroup";
import { useAppStore, type ThemeMode } from "../../stores/appStore";
import { SettingGroup, SettingRow } from "./SettingRow";

const THEME_MODES: readonly ThemeMode[] = ["light", "dark", "system"];
const THEME_LABEL: Record<ThemeMode, string> = { light: "浅色", dark: "深色", system: "跟随系统" };

type CloseBehavior = "ask" | "tray" | "quit";
const CLOSE_MODES: readonly CloseBehavior[] = ["ask", "tray", "quit"];
const CLOSE_LABEL: Record<CloseBehavior, string> = { ask: "每次询问", tray: "挂到托盘", quit: "退出应用" };
/** M21: each label must say what actually happens, especially that tray is not quit. */
const CLOSE_HINT: Record<CloseBehavior, string> = {
  ask: "关窗时问一次：退出，还是挂到托盘（可勾选记住）",
  tray: "关窗只隐藏窗口，应用继续在后台运行，点托盘图标回来",
  quit: "关窗即结束进程",
};

/** A miniature of the app: canvas, sidebar and a page, coloured by its own theme. */
function ThemeThumb({ mode }: { readonly mode: ThemeMode }) {
  const pane = (preview: "light" | "dark", mirrored: boolean) => (
    <span className="theme-pane" data-preview={preview}>
      {mirrored ? null : <span className="theme-block" data-kind="side" />}
      <span className="theme-block" data-kind="main">
        <span className="theme-bar" data-kind="accent" />
        <span className="theme-bar" />
        <span className="theme-bar" data-kind="short" />
      </span>
      {mirrored ? <span className="theme-block" data-kind="side" /> : null}
    </span>
  );
  return (
    <span className="theme-thumb" aria-hidden="true">
      {mode === "system" ? (
        <>
          {pane("light", false)}
          {pane("dark", true)}
        </>
      ) : (
        pane(mode, false)
      )}
    </span>
  );
}

/** The theme that is actually in effect right now (system follows the OS). */
function useResolvedTheme(mode: ThemeMode): "light" | "dark" {
  const resolve = (): "light" | "dark" => {
    if (mode !== "system") return mode;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };
  const [resolved, setResolved] = useState<"light" | "dark">(resolve);
  useEffect(() => {
    if (mode !== "system") {
      setResolved(mode);
      return;
    }
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = (): void => setResolved(media.matches ? "dark" : "light");
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode]);
  return resolved;
}

function asTheme(value: string | undefined): ThemeMode {
  return value === "light" || value === "dark" ? value : "system";
}

function asClose(value: string | undefined): CloseBehavior {
  return value === "tray" || value === "quit" ? value : "ask";
}

/** Settings → 常规 (T05): theme, close behaviour and read-only panel widths.
 * Choices apply immediately and go through M61, the same store the title bar
 * uses, so the two controls can never disagree. */
export function GeneralSection({ onClose }: { readonly onClose: () => void }) {
  const settings = useAppStore((state) => state.settings);
  const setTheme = useAppStore((state) => state.setTheme);
  const setCloseBehavior = useAppStore((state) => state.setCloseBehavior);
  const theme = asTheme(settings?.theme);
  const closeBehavior = asClose(settings?.closeBehavior);
  const resolved = useResolvedTheme(theme);
  const themeRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const closeRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const sidebar = settings?.layout?.sidebarWidth ?? 232;
  const docWidth = settings?.layout?.docWidth ?? 560;

  const moveTheme = (event: React.KeyboardEvent): void => {
    const next = nextOption(THEME_MODES, theme, event.key);
    if (!next) return;
    event.preventDefault();
    void setTheme(next);
    themeRefs.current[next]?.focus();
  };
  const moveClose = (event: React.KeyboardEvent): void => {
    const next = nextOption(CLOSE_MODES, closeBehavior, event.key);
    if (!next) return;
    event.preventDefault();
    void setCloseBehavior(next);
    closeRefs.current[next]?.focus();
  };

  return (
    <div className="settings-content" data-testid="settings-appearance">
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">常规</h1>
          <p className="settings-subtitle">这台电脑上的显示方式，不随知识库走。</p>
          <div className="settings-chips">
            <span className="settings-chip">保存在这台电脑</span>
          </div>
        </div>
        <button type="button" className="icon-button" aria-label="关闭设置" onClick={onClose}>
          <X size={16} aria-hidden />
        </button>
      </header>

      <div className="settings-body scroll-y">
        <div className="settings-stack">
          <SettingGroup title="外观" description="跟随系统时，系统切换深浅会立即跟着变">
            <SettingRow label="主题" help="改完立即生效，标题栏的切换与这里是同一份设置" stacked>
              <div className="theme-tiles" role="radiogroup" aria-label="主题" onKeyDown={moveTheme}>
                {THEME_MODES.map((mode) => {
                  const checked = theme === mode;
                  return (
                    <button
                      key={mode}
                      ref={(node) => {
                        themeRefs.current[mode] = node;
                      }}
                      type="button"
                      role="radio"
                      aria-checked={checked}
                      tabIndex={checked ? 0 : -1}
                      className="theme-tile"
                      onClick={() => void setTheme(mode)}
                      data-testid={`theme-${mode}`}
                    >
                      <ThemeThumb mode={mode} />
                      <span className="theme-tile-label">
                        {THEME_LABEL[mode]}
                        <Check size={12} weight="bold" aria-hidden className="theme-tile-check" />
                      </span>
                      {mode === "system" && checked ? (
                        <span className="theme-tile-sub" data-testid="theme-resolved">
                          当前生效：{resolved === "dark" ? "深色" : "浅色"}
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            </SettingRow>
          </SettingGroup>

          <SettingGroup
            title="窗口"
            footnote="托盘图标始终存在，菜单里有「显示主窗口」与「退出」。"
            testid="settings-close-behavior"
          >
            <SettingRow
              label="点关闭按钮时"
              help={CLOSE_HINT[closeBehavior]}
              control={
                <div className="segmented" role="radiogroup" aria-label="点关闭按钮时" onKeyDown={moveClose}>
                  {CLOSE_MODES.map((mode) => {
                    const checked = closeBehavior === mode;
                    return (
                      <button
                        key={mode}
                        ref={(node) => {
                          closeRefs.current[mode] = node;
                        }}
                        type="button"
                        role="radio"
                        aria-checked={checked}
                        tabIndex={checked ? 0 : -1}
                        className="segmented-item"
                        onClick={() => void setCloseBehavior(mode)}
                        data-testid={`close-behavior-${mode}`}
                      >
                        {CLOSE_LABEL[mode]}
                      </button>
                    );
                  })}
                </div>
              }
            />
            <SettingRow
              label="面板宽度"
              help="在主界面拖分隔条调整，宽度会被记住。左栏 192–320px，文档面板 320–800px"
              control={
                <span className="settings-widths" data-testid="settings-layout-widths">
                  左栏 {sidebar}px · 文档 {docWidth}px
                </span>
              }
            />
          </SettingGroup>
        </div>
      </div>
    </div>
  );
}
