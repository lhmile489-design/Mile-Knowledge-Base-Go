// Global material levels (PRD section 19, 技术设计 21.2). The same four values
// are validated in public/theme-init.js for the first frame; this module is
// the in-app equivalent, used when settings change at runtime.

export type EffectLevel = "off" | "low" | "std" | "high";

export const EFFECT_LEVELS: readonly EffectLevel[] = ["off", "low", "std", "high"];

export function isEffectLevel(value: unknown): value is EffectLevel {
  return typeof value === "string" && (EFFECT_LEVELS as readonly string[]).includes(value);
}

/** Mirrors the resolved levels onto <html>; unknown values are not written. */
export function applyEffectsTo(
  root: HTMLElement,
  effects: { motion?: string; blur?: string } | null | undefined,
): void {
  if (!effects) return;
  if (isEffectLevel(effects.motion)) {
    root.dataset.motion = effects.motion;
  }
  if (isEffectLevel(effects.blur)) {
    root.dataset.blur = effects.blur;
  }
}
