import { beforeEach, describe, expect, it } from "vitest";
// The first-paint script is shipped as a plain file under public/ (the
// production CSP forbids inline scripts), so the test evaluates the real file
// in the jsdom window instead of re-implementing it.
import source from "../../public/theme-init.js?raw";

function run(effects: unknown): void {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: () => ({
      matches: false,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }),
  });
  delete (window as unknown as Record<string, unknown>).__MKB_ACCENT__;
  delete (window as unknown as Record<string, unknown>).__MKB_EFFECTS__;
  if (effects !== undefined) {
    (window as unknown as Record<string, unknown>).__MKB_EFFECTS__ = effects;
  }
  // eslint-disable-next-line no-new-func
  new Function("window", "document", source)(window, document);
}

beforeEach(() => {
  delete document.documentElement.dataset.motion;
  delete document.documentElement.dataset.blur;
});

describe("theme-init effects", () => {
  it("writes both levels for valid values", () => {
    run({ motion: "high", blur: "off" });
    expect(document.documentElement.dataset.motion).toBe("high");
    expect(document.documentElement.dataset.blur).toBe("off");
  });

  it("writes a single provided level and leaves the other to the CSS default", () => {
    run({ motion: "low" });
    expect(document.documentElement.dataset.motion).toBe("low");
    expect(document.documentElement.dataset.blur).toBeUndefined();
  });

  it("ignores unknown values without failing the boot", () => {
    run({ motion: "turbo", blur: 7 });
    expect(document.documentElement.dataset.motion).toBeUndefined();
    expect(document.documentElement.dataset.blur).toBeUndefined();
  });

  it("writes nothing when the preload value is absent", () => {
    run(undefined);
    expect(document.documentElement.dataset.motion).toBeUndefined();
    expect(document.documentElement.dataset.blur).toBeUndefined();
  });

  it("survives a malformed container", () => {
    run("effects");
    expect(document.documentElement.dataset.motion).toBeUndefined();
    expect(document.documentElement.dataset.blur).toBeUndefined();
  });
});
