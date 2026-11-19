import { useEffect, useRef, useState } from "react";
import { Check } from "@phosphor-icons/react";
import { PRESETS, PRESET_BASE, PRESET_LABEL, PRESET_NAMES, deriveAccent } from "../lib/accent";
import { nextOption } from "../lib/radioGroup";
import { useAppStore } from "../stores/appStore";

const HEX_RE = /^#[0-9a-fA-F]{6}$/;

/** Title-bar accent dot and its swatch popover (docs/UI设计稿.md 10.1,
 * DESIGN.md 12). Picking applies immediately: appStore.setAccent writes M61. */
export function AccentPicker() {
  const accent = useAppStore((state) => state.settings?.accent);
  const themeMode = useAppStore((state) => state.settings?.theme ?? "system");
  const setAccent = useAppStore((state) => state.setAccent);
  const theme: "light" | "dark" =
    themeMode === "light" || themeMode === "dark"
      ? themeMode
      : window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
  const [open, setOpen] = useState(false);
  const [focusIndex, setFocusIndex] = useState(0);
  const [draft, setDraft] = useState("");
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const dotRef = useRef<HTMLButtonElement | null>(null);

  const current = accent?.preset ?? "orange";
  const isCustom = current === "custom";
  const label = isCustom ? "自定义" : (PRESET_LABEL[current as keyof typeof PRESET_LABEL] ?? "橙");

  useEffect(() => {
    if (!open) return;
    setFocusIndex(Math.max(0, PRESET_NAMES.indexOf(current as (typeof PRESET_NAMES)[number])));
    setDraft(isCustom ? (accent?.hex ?? "") : "");
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null;
      if (target && wrapRef.current && !wrapRef.current.contains(target)) close();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        close();
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [open, current, isCustom, accent?.hex]);

  const close = () => {
    setOpen(false);
    dotRef.current?.focus();
  };

  const pickPreset = (preset: (typeof PRESET_NAMES)[number]) => {
    void setAccent({ preset });
  };

  // The custom hint: an error, the adjusted color, or "as typed".
  let hint: { tone: "error" | "adjusted" | "same"; text: string } | null = null;
  if (draft.length > 0) {
    if (!HEX_RE.test(draft)) {
      hint = { tone: "error", text: "请输入 #RRGGBB 格式，例如 #2563eb。当前颜色不变。" };
    } else {
      try {
        const derived = deriveAccent(draft);
        const used = derived.light.accent;
        hint = used.toLowerCase() === draft.toLowerCase()
          ? { tone: "same", text: "与输入一致。" }
          : { tone: "adjusted", text: `实际使用 ${used}（浅色），为了让文字和按钮看得清做了调整。` };
      } catch {
        hint = { tone: "error", text: "请输入 #RRGGBB 格式，例如 #2563eb。当前颜色不变。" };
      }
    }
  }

  const commitCustom = () => {
    if (!HEX_RE.test(draft)) return;
    void setAccent({ preset: "custom", hex: draft.toLowerCase() });
  };

  const onSwatchKeyDown = (event: React.KeyboardEvent) => {
    const target = nextOption(PRESET_NAMES, PRESET_NAMES[focusIndex] as (typeof PRESET_NAMES)[number], event.key);
    if (!target) return;
    event.preventDefault();
    setFocusIndex(PRESET_NAMES.indexOf(target));
    const buttons = wrapRef.current?.querySelectorAll<HTMLButtonElement>(".accent-swatch");
    buttons?.[PRESET_NAMES.indexOf(target)]?.focus();
  };

  return (
    <div className="accent-picker no-drag" ref={wrapRef}>
      <button
        ref={dotRef}
        type="button"
        className="icon-button accent-dot-btn"
        aria-label={`主题色：${label}`}
        aria-haspopup="dialog"
        aria-expanded={open}
        data-testid="accent-dot"
        onClick={() => (open ? close() : setOpen(true))}
      >
        <span className="accent-dot" aria-hidden />
      </button>
      {open ? (
        <div className="popover accent-pop" role="dialog" aria-label="主题色" data-testid="accent-popover">
          <h4 className="accent-title">主题色</h4>
          <div className="accent-swatches" role="radiogroup" aria-label="主题色" onKeyDown={onSwatchKeyDown}>
            {PRESET_NAMES.map((preset, index) => (
              <button
                key={preset}
                type="button"
                role="radio"
                aria-checked={!isCustom && current === preset}
                aria-label={PRESET_LABEL[preset]}
                tabIndex={index === focusIndex ? 0 : -1}
                className="accent-swatch"
                style={{
                  ["--swatch" as string]: PRESET_BASE[preset],
                  ["--on-swatch" as string]: PRESETS[preset][theme].onAccent,
                }}
                data-testid={`accent-preset-${preset}`}
                onClick={() => pickPreset(preset)}
                onFocus={() => setFocusIndex(index)}
              >
                <span className="accent-chip" aria-hidden>
                  {!isCustom && current === preset ? <Check size={14} weight="bold" /> : null}
                </span>
                {PRESET_LABEL[preset]}
              </button>
            ))}
          </div>
          <div className="accent-sep" role="separator" />
          <div className="accent-custom">
            <span>自定义</span>
            <input
              className="accent-hex"
              type="text"
              spellCheck={false}
              value={draft}
              placeholder="#RRGGBB"
              aria-label="自定义主题色"
              aria-invalid={hint?.tone === "error"}
              data-testid="accent-hex"
              onChange={(event) => setDraft(event.target.value.trim())}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  commitCustom();
                }
              }}
            />
            <span
              className="accent-preview"
              aria-hidden
              style={{ ["--swatch" as string]: HEX_RE.test(draft) ? draft : "transparent" }}
            />
          </div>
          {hint ? (
            <p className="accent-note" data-tone={hint.tone} data-testid="accent-hint">
              {hint.text}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
