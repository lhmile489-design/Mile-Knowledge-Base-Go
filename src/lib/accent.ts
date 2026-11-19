// Theme color (R2, PRD §16, DESIGN.md §12). The six preset palettes are
// fixed constants; a custom hex is derived here and nowhere else, and the
// Go side only stores the result. The contrast helpers are exported so the
// visual checks (M3.5 T07) use the same math instead of a second copy.
import type { AccentCache, AccentSetting, AccentTokens } from "../mygo";

export const PRESET_NAMES = ["orange", "indigo", "blue", "green", "teal", "violet"] as const;
export type AccentPresetName = (typeof PRESET_NAMES)[number];
export type AccentName = AccentPresetName | "custom";

/** The base color each preset is named after (DESIGN.md 12.3). */
export const PRESET_BASE: Record<AccentPresetName, string> = {
  orange: "#fa7b29",
  indigo: "#4f46e5",
  blue: "#2563eb",
  green: "#16a34a",
  teal: "#0d9488",
  violet: "#7c3aed",
};

/** Display names for the swatches (docs/UI设计稿.md §10). */
export const PRESET_LABEL: Record<AccentPresetName, string> = {
  orange: "橙",
  indigo: "靛",
  blue: "蓝",
  green: "绿",
  teal: "青",
  violet: "紫",
};

const ORANGE_LIGHT: AccentTokens = {
  accent: "#e87226", hover: "#d16722", active: "#ba5b1e", soft: "#fceee5",
  text: "#aa541c", btn: "#ba5b1e", btnHover: "#a7521b", btnActive: "#954918",
  onAccent: "#ffffff", focus: "#e87226",
};
const ORANGE_DARK: AccentTokens = {
  accent: "#fa8234", hover: "#fb914c", active: "#dc722e", soft: "#4d3426",
  text: "#fa8234", btn: "#fa8234", btnHover: "#fb914c", btnActive: "#dc722e",
  onAccent: "#111214", focus: "#fa8234",
};
const INDIGO_LIGHT: AccentTokens = {
  accent: "#4f46e5", hover: "#4338ca", active: "#3730a3", soft: "#ebeafd",
  text: "#4f46e5", btn: "#4f46e5", btnHover: "#4338ca", btnActive: "#3730a3",
  onAccent: "#ffffff", focus: "#4f46e5",
};
const INDIGO_DARK: AccentTokens = {
  accent: "#8b8ff8", hover: "#a2a5fa", active: "#7579ec", soft: "#2a2b45",
  text: "#8b8ff8", btn: "#8b8ff8", btnHover: "#a2a5fa", btnActive: "#7579ec",
  onAccent: "#111214", focus: "#8b8ff8",
};
const BLUE_LIGHT: AccentTokens = {
  accent: "#2563eb", hover: "#2159d4", active: "#1e4fbc", soft: "#e5ecfd",
  text: "#2461e6", btn: "#2563eb", btnHover: "#2159d4", btnActive: "#1e4fbc",
  onAccent: "#ffffff", focus: "#2563eb",
};
const BLUE_DARK: AccentTokens = {
  accent: "#81a5f3", hover: "#90b0f4", active: "#7291d6", soft: "#323c50",
  text: "#81a5f3", btn: "#81a5f3", btnHover: "#90b0f4", btnActive: "#7291d6",
  onAccent: "#111214", focus: "#81a5f3",
};
const GREEN_LIGHT: AccentTokens = {
  accent: "#16a34a", hover: "#149343", active: "#12823b", soft: "#e3f4e9",
  text: "#117c38", btn: "#12873d", btnHover: "#107a37", btnActive: "#0e6c31",
  onAccent: "#ffffff", focus: "#16a34a",
};
const GREEN_DARK: AccentTokens = {
  accent: "#4cb874", hover: "#61c185", active: "#43a266", soft: "#274034",
  text: "#4cb874", btn: "#4cb874", btnHover: "#61c185", btnActive: "#43a266",
  onAccent: "#111214", focus: "#4cb874",
};
const TEAL_LIGHT: AccentTokens = {
  accent: "#0d9488", hover: "#0c857a", active: "#0a766d", soft: "#e2f2f1",
  text: "#0b7970", btn: "#0c857a", btnHover: "#0b786e", btnActive: "#0a6a62",
  onAccent: "#ffffff", focus: "#0d9488",
};
const TEAL_DARK: AccentTokens = {
  accent: "#56b4ac", hover: "#6abdb6", active: "#4c9e97", soft: "#293f40",
  text: "#56b4ac", btn: "#56b4ac", btnHover: "#6abdb6", btnActive: "#4c9e97",
  onAccent: "#111214", focus: "#56b4ac",
};
const VIOLET_LIGHT: AccentTokens = {
  accent: "#7c3aed", hover: "#7034d5", active: "#632ebe", soft: "#efe7fd",
  text: "#7c3aed", btn: "#7c3aed", btnHover: "#7034d5", btnActive: "#632ebe",
  onAccent: "#ffffff", focus: "#7c3aed",
};
const VIOLET_DARK: AccentTokens = {
  accent: "#b793f5", hover: "#c0a0f6", active: "#a181d8", soft: "#3e3850",
  text: "#b793f5", btn: "#b793f5", btnHover: "#c0a0f6", btnActive: "#a181d8",
  onAccent: "#111214", focus: "#b793f5",
};

/** The 12 fixed palettes, verbatim from DESIGN.md 12.3. */
export const PRESETS: Record<AccentPresetName, { light: AccentTokens; dark: AccentTokens }> = {
  orange: { light: ORANGE_LIGHT, dark: ORANGE_DARK },
  indigo: { light: INDIGO_LIGHT, dark: INDIGO_DARK },
  blue: { light: BLUE_LIGHT, dark: BLUE_DARK },
  green: { light: GREEN_LIGHT, dark: GREEN_DARK },
  teal: { light: TEAL_LIGHT, dark: TEAL_DARK },
  violet: { light: VIOLET_LIGHT, dark: VIOLET_DARK },
};

// ---- contrast math (WCAG 2.x), shared with the T07 checks ----

export function hexToRgb(hex: string): [number, number, number] {
  return [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)) as [number, number, number];
}

export function relativeLuminance(hex: string): number {
  const channel = (v: number) => {
    const s = v / 255;
    return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  const [r, g, b] = hexToRgb(hex);
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

export function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const hi = Math.max(la, lb);
  const lo = Math.min(la, lb);
  return (hi + 0.05) / (lo + 0.05);
}

// ---- custom color derivation (DESIGN.md 12.4) ----

const WHITE_CARD = "#ffffff";
const CANVAS = "#eceef1";
const RAISED = "#eceef1";
const DARK_CARD = "#1c1e22";
const DARK_CANVAS = "#0e0f11";
const DARK_RAISED = "#26282d";
const NEAR_BLACK = "#111214";
const HEX_RE = /^#[0-9a-fA-F]{6}$/;

function toHex(rgb: number[]): string {
  return "#" + rgb.map((v) => Math.round(v).toString(16).padStart(2, "0")).join("");
}

/** mix returns t of a and (1-t) of b, like CSS color-mix. */
export function mix(t: number, a: string, b: string): string {
  const A = hexToRgb(a);
  const B = hexToRgb(b);
  return toHex([A[0] * t + B[0] * (1 - t), A[1] * t + B[1] * (1 - t), A[2] * t + B[2] * (1 - t)]);
}

function until(base: string, toward: string, ok: (c: string) => boolean): string {
  for (let k = 0; k <= 100; k++) {
    const c = mix(k / 100, toward, base);
    if (ok(c)) return c;
  }
  return toward;
}

/** The near-gray hint of DESIGN.md 12.4: very low saturation is still usable. */
export function isNearGray(hex: string): boolean {
  const [r, g, b] = hexToRgb(hex);
  return Math.max(r, g, b) - Math.min(r, g, b) < 12;
}

/** deriveAccent computes both palettes for a custom hex (DESIGN.md 12.4). */
export function deriveAccent(input: string): AccentCache {
  if (!HEX_RE.test(input)) {
    throw new Error("accent hex must be #rrggbb");
  }
  const base = input.toLowerCase();

  // Light: the fill keeps the color where it can, the button goes dark
  // enough for white text, and the text tone goes dark enough for 4.5:1 on
  // every light surface.
  const accent = contrastRatio(base, WHITE_CARD) >= 3
    ? base
    : until(base, "#000000", (c) => contrastRatio(c, WHITE_CARD) >= 3);
  const btn = until(accent, "#000000", (c) => contrastRatio("#ffffff", c) >= 4.5);
  const lightBase: AccentTokens = {
    accent,
    hover: mix(0.1, "#000000", accent),
    active: mix(0.2, "#000000", accent),
    soft: mix(0.12, accent, WHITE_CARD),
    text: until(base, "#000000", (c) =>
      [WHITE_CARD, CANVAS, RAISED, mix(0.12, accent, WHITE_CARD)].every((bg) => contrastRatio(c, bg) >= 4.5)),
    btn,
    btnHover: mix(0.1, "#000000", btn),
    btnActive: mix(0.2, "#000000", btn),
    onAccent: "#ffffff",
    focus: accent,
  };

  // Dark: the accent is light enough for 4.5:1 on every dark surface, and the
  // button reuses it with near-black text.
  const darkAccent = until(base, "#ffffff", (c) =>
    [DARK_CARD, DARK_CANVAS, DARK_RAISED].every((bg) => contrastRatio(c, bg) >= 4.5)
    && contrastRatio(c, mix(0.22, c, DARK_CARD)) >= 4.5);
  const darkOn = contrastRatio(NEAR_BLACK, darkAccent) >= 4.5 ? NEAR_BLACK : "#ffffff";
  const darkHover = mix(0.12, "#ffffff", darkAccent);
  const darkActive = mix(0.12, "#000000", darkAccent);
  const darkBase: AccentTokens = {
    accent: darkAccent,
    hover: darkHover,
    active: darkActive,
    soft: mix(0.22, darkAccent, DARK_CARD),
    text: darkAccent,
    btn: darkAccent,
    btnHover: darkHover,
    btnActive: darkActive,
    onAccent: darkOn,
    focus: darkAccent,
  };

  return { light: lightBase, dark: darkBase };
}

/** tokensFor is what the UI applies: a preset's constants or derived values. */
export function tokensFor(accent: AccentSetting, theme: "light" | "dark"): AccentTokens {
  if (accent.preset === "custom") {
    if (accent.cache) return theme === "light" ? accent.cache.light : accent.cache.dark;
    return deriveAccent(accent.hex ?? "")[theme];
  }
  const preset = PRESETS[accent.preset as AccentPresetName];
  if (!preset) return PRESETS.orange[theme];
  return preset[theme];
}

/** accentCssVars maps tokens onto the CSS custom properties (DESIGN.md 12.1). */
export function accentCssVars(t: AccentTokens): Record<string, string> {
  return {
    "--raw-accent": t.accent,
    "--raw-accent-hover": t.hover,
    "--raw-accent-active": t.active,
    "--raw-accent-soft": t.soft,
    "--raw-accent-text": t.text,
    "--raw-accent-btn": t.btn,
    "--raw-accent-btn-hover": t.btnHover,
    "--raw-accent-btn-active": t.btnActive,
    "--raw-on-accent": t.onAccent,
    "--text-on-accent": t.onAccent,
    "--focus": t.focus,
    "--color-accent": t.accent,
    "--color-text-on-accent": t.onAccent,
  };
}
