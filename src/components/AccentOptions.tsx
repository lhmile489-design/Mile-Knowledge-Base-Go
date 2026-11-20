import { useEffect, useRef, useState } from "react";
import { Check } from "@phosphor-icons/react";
import {
  PRESETS,
  PRESET_BASE,
  PRESET_LABEL,
  PRESET_NAMES,
  deriveAccent,
  type AccentPresetName,
} from "../lib/accent";
import { nextOption } from "../lib/radioGroup";
import { useResolvedTheme } from "../lib/useResolvedTheme";
import { useAppStore } from "../stores/appStore";

const HEX_RE = /^#[0-9a-fA-F]{6}$/;
const INVALID_HINT = "请输入 #RRGGBB 格式，例如 #2563eb。当前颜色不变。";

type Hint = { tone: "error" | "adjusted" | "same"; text: string } | null;

/** The custom-color hint: an error, the adjusted color, or "as typed". */
function hintFor(draft: string): Hint {
  if (draft.length === 0) return null;
  if (!HEX_RE.test(draft)) return { tone: "error", text: INVALID_HINT };
  try {
    const used = deriveAccent(draft).light.accent;
    return used.toLowerCase() === draft.toLowerCase()
      ? { tone: "same", text: "与输入一致。" }
      : { tone: "adjusted", text: `实际使用 ${used}（浅色），为了让文字和按钮看得清做了调整。` };
  } catch {
    return { tone: "error", text: INVALID_HINT };
  }
}

/** Six preset swatches plus the custom hex row (docs/UI设计稿.md 10.1–10.2).
 * Shared by the title-bar popover and the settings row so the two can never
 * drift apart; both write through appStore.setAccent (M61) and apply at once. */
export function AccentOptions({
  testIdPrefix = "accent",
  variant = "popover",
}: {
  readonly testIdPrefix?: string;
  readonly variant?: "popover" | "inline";
}) {
  const accent = useAppStore((state) => state.settings?.accent);
  const rawTheme = useAppStore((state) => state.settings?.theme);
  const setAccent = useAppStore((state) => state.setAccent);
  const theme = useResolvedTheme(rawTheme === "light" || rawTheme === "dark" ? rawTheme : "system");
  const current = accent?.preset ?? "orange";
  const isCustom = current === "custom";
  const [focusIndex, setFocusIndex] = useState(() =>
    Math.max(0, PRESET_NAMES.indexOf(current as AccentPresetName)),
  );
  const [draft, setDraft] = useState(() => (isCustom ? (accent?.hex ?? "") : ""));
  const groupRef = useRef<HTMLDivElement | null>(null);

  // Follow a color picked elsewhere (the other entry point); keyboard focus in
  // this group only moves focusIndex, which never touches `current`.
  useEffect(() => {
    setFocusIndex(Math.max(0, PRESET_NAMES.indexOf(current as AccentPresetName)));
  }, [current]);

  const pickPreset = (preset: AccentPresetName) => {
    void setAccent({ preset });
  };

  const hint = hintFor(draft);

  const commitCustom = () => {
    if (!HEX_RE.test(draft)) return;
    void setAccent({ preset: "custom", hex: draft.toLowerCase() });
  };

  const onSwatchKeyDown = (event: React.KeyboardEvent) => {
    const target = nextOption(PRESET_NAMES, PRESET_NAMES[focusIndex] as AccentPresetName, event.key);
    if (!target) return;
    event.preventDefault();
    setFocusIndex(PRESET_NAMES.indexOf(target));
    const buttons = groupRef.current?.querySelectorAll<HTMLButtonElement>(".accent-swatch");
    buttons?.[PRESET_NAMES.indexOf(target)]?.focus();
  };

  return (
    <div className="accent-options" data-variant={variant}>
      <div ref={groupRef} className="accent-swatches" role="radiogroup" aria-label="主题色" onKeyDown={onSwatchKeyDown}>
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
            data-testid={`${testIdPrefix}-preset-${preset}`}
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
      {variant === "popover" ? <div className="accent-sep" role="separator" /> : null}
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
          data-testid={`${testIdPrefix}-hex`}
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
        <p className="accent-note" data-tone={hint.tone} data-testid={`${testIdPrefix}-hint`}>
          {hint.text}
        </p>
      ) : null}
    </div>
  );
}
