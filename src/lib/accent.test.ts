import { describe, expect, it } from "vitest";
import type { AccentTokens } from "../mygo";
import {
  PRESET_BASE,
  PRESET_NAMES,
  PRESETS,
  accentCssVars,
  accentInlineVars,
  applyAccentTo,
  contrastRatio,
  deriveAccent,
  isNearGray,
  tokensFor,
} from "./accent";

// DESIGN.md 12.3 的验证标准。
const LIGHT_SURFACES = ["#ffffff", "#eceef1", "#eceef1"];
const DARK_SURFACES = ["#1c1e22", "#0e0f11", "#26282d"];

function expectPalette(tokens: AccentTokens, surfaces: string[], label: string) {
  const card = surfaces[0] ?? "#ffffff";
  const bgs = [...surfaces, tokens.soft];
  for (const bg of bgs) {
    expect(contrastRatio(tokens.text, bg), `${label} text on ${bg}`).toBeGreaterThanOrEqual(4.5);
  }
  for (const fill of [tokens.btn, tokens.btnHover, tokens.btnActive]) {
    expect(contrastRatio(tokens.onAccent, fill), `${label} on-accent on ${fill}`).toBeGreaterThanOrEqual(4.5);
  }
  expect(contrastRatio(tokens.accent, card), `${label} accent on card`).toBeGreaterThanOrEqual(3);
}

describe("preset palettes", () => {
  it("ships six presets", () => {
    expect(PRESET_NAMES).toHaveLength(6);
  });

  it("every preset meets the contrast standard in both themes", () => {
    for (const name of PRESET_NAMES) {
      expectPalette(PRESETS[name].light, LIGHT_SURFACES, `${name} light`);
      expectPalette(PRESETS[name].dark, DARK_SURFACES, `${name} dark`);
    }
  });

  it("deriveAccent reproduces the fixed palettes (indigo is locked to the legacy values)", () => {
    for (const name of PRESET_NAMES) {
      if (name === "indigo") continue;
      expect(deriveAccent(PRESET_BASE[name]), name).toEqual(PRESETS[name]);
    }
  });
});

describe("deriveAccent", () => {
  it("produces usable palettes for extreme and gray inputs", () => {
    for (const hex of ["#ffff00", "#000000", "#ffffff", "#808080"]) {
      const derived = deriveAccent(hex);
      expectPalette(derived.light, LIGHT_SURFACES, `${hex} light`);
      expectPalette(derived.dark, DARK_SURFACES, `${hex} dark`);
    }
  });

  it("accepts upper case and returns lower case", () => {
    expect(deriveAccent("#FFFF00")).toEqual(deriveAccent("#ffff00"));
    expect(deriveAccent("#FFFF00").light.btn).toMatch(/^#[0-9a-f]{6}$/);
  });

  it("rejects malformed hex", () => {
    for (const bad of ["#fff", "ff7b29", "#gggggg", "", "#1234567"]) {
      expect(() => deriveAccent(bad), bad).toThrow();
    }
  });

  it("flags near-gray inputs only", () => {
    expect(isNearGray("#808080")).toBe(true);
    expect(isNearGray("#8a8b8c")).toBe(true);
    expect(isNearGray("#ff0000")).toBe(false);
    expect(isNearGray("#fa7b29")).toBe(false);
  });
});

describe("tokensFor", () => {
  it("returns preset constants", () => {
    expect(tokensFor({ preset: "teal" }, "dark")).toEqual(PRESETS.teal.dark);
  });

  it("uses the cache for a custom color", () => {
    const cache = deriveAccent("#2563eb");
    expect(tokensFor({ preset: "custom", hex: "#123456", cache }, "light")).toEqual(cache.light);
  });

  it("derives when a custom color has no cache", () => {
    expect(tokensFor({ preset: "custom", hex: "#2563eb" }, "dark")).toEqual(deriveAccent("#2563eb").dark);
  });

  it("falls back to orange for an unknown preset", () => {
    expect(tokensFor({ preset: "pink" }, "light")).toEqual(PRESETS.orange.light);
  });
});

describe("accentCssVars", () => {
  it("maps the palette onto CSS variables", () => {
    const vars = accentCssVars(PRESETS.orange.light);
    expect(Object.keys(vars)).toHaveLength(13);
    for (const [name, value] of Object.entries(vars)) {
      expect(name.startsWith("--"), name).toBe(true);
      expect(value, name).toMatch(/^#[0-9a-f]{6}$/);
    }
    expect(vars["--raw-accent"]).toBe(PRESETS.orange.light.accent);
    expect(vars["--raw-accent-btn"]).toBe(PRESETS.orange.light.btn);
    expect(vars["--raw-on-accent"]).toBe(PRESETS.orange.light.onAccent);
  });
});

describe("inline variables and application", () => {
  it("names twenty variables for a custom color", () => {
    const vars = accentInlineVars(deriveAccent("#2563eb"));
    expect(Object.keys(vars)).toHaveLength(20);
    expect(vars["--accent-light-btn"]).toBe(deriveAccent("#2563eb").light.btn);
    expect(vars["--accent-dark-on-accent"]).toBe(deriveAccent("#2563eb").dark.onAccent);
  });

  it("applies a preset name to the root element", () => {
    const root = document.createElement("html");
    applyAccentTo(root, { preset: "teal" });
    expect(root.dataset.accent).toBe("teal");
  });

  it("applies a custom color and clears its variables when switching back", () => {
    const root = document.createElement("html");
    applyAccentTo(root, { preset: "custom", hex: "#ffff00", cache: deriveAccent("#ffff00") });
    expect(root.dataset.accent).toBe("custom");
    expect(root.style.getPropertyValue("--accent-light-accent")).toBe(deriveAccent("#ffff00").light.accent);
    applyAccentTo(root, { preset: "blue" });
    expect(root.dataset.accent).toBe("blue");
    expect(root.style.getPropertyValue("--accent-light-accent")).toBe("");
  });

  it("falls back to orange for an unusable custom color", () => {
    const root = document.createElement("html");
    applyAccentTo(root, { preset: "custom", hex: "nope" });
    expect(root.dataset.accent).toBe("orange");
  });
});
