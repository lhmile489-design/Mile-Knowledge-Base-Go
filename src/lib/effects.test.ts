import { describe, expect, it } from "vitest";
import { applyEffectsTo, isEffectLevel } from "./effects";

describe("isEffectLevel", () => {
  it("accepts the four levels only", () => {
    for (const value of ["off", "low", "std", "high"]) {
      expect(isEffectLevel(value)).toBe(true);
    }
    for (const value of ["", "turbo", "STD", 1, null, undefined]) {
      expect(isEffectLevel(value)).toBe(false);
    }
  });
});

describe("applyEffectsTo", () => {
  it("writes both levels onto the root element", () => {
    const root = document.createElement("div");
    applyEffectsTo(root, { motion: "low", blur: "high" });
    expect(root.dataset.motion).toBe("low");
    expect(root.dataset.blur).toBe("high");
  });

  it("writes the valid level and skips the invalid one", () => {
    const root = document.createElement("div");
    applyEffectsTo(root, { motion: "turbo", blur: "off" });
    expect(root.dataset.motion).toBeUndefined();
    expect(root.dataset.blur).toBe("off");
  });

  it("does nothing without a value", () => {
    const root = document.createElement("div");
    applyEffectsTo(root, null);
    applyEffectsTo(root, undefined);
    expect(root.dataset.motion).toBeUndefined();
    expect(root.dataset.blur).toBeUndefined();
  });
});
