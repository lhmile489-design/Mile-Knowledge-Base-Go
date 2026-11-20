import { useEffect, useRef, useState } from "react";
import { PRESET_LABEL } from "../lib/accent";
import { useAppStore } from "../stores/appStore";
import { AccentOptions } from "./AccentOptions";

/** Title-bar accent dot and its swatch popover (docs/UI设计稿.md 10.1,
 * DESIGN.md 12). Picking applies immediately: appStore.setAccent writes M61. */
export function AccentPicker() {
  const accent = useAppStore((state) => state.settings?.accent);
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement | null>(null);
  const dotRef = useRef<HTMLButtonElement | null>(null);

  const current = accent?.preset ?? "orange";
  const label = current === "custom" ? "自定义" : (PRESET_LABEL[current as keyof typeof PRESET_LABEL] ?? "橙");

  useEffect(() => {
    if (!open) return;
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
  }, [open]);

  const close = () => {
    setOpen(false);
    dotRef.current?.focus();
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
          <AccentOptions />
        </div>
      ) : null}
    </div>
  );
}
