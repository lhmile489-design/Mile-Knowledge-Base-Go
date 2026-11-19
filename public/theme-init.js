// Runs before the app mounts, so the first paint already has the right theme
// and accent (PRD FR-THEME, FR-ACCENT). Kept as a separate file because
// production CSP forbids inline scripts. Go sets Theme source, the window
// background and window.__MKB_ACCENT__ (技术设计 14.2); this script only
// mirrors the resolved appearance onto <html>.
(() => {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = () => {
    const theme = media.matches ? "dark" : "light";
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  };
  apply();
  media.addEventListener("change", apply);

  // The accent: a preset name picks its [data-accent] block, a custom color
  // carries the twenty inline variables Go stored with it. Anything
  // unexpected leaves the CSS default (orange) in place.
  const PRESETS = ["orange", "indigo", "blue", "green", "teal", "violet"];
  const TOKENS = ["accent", "hover", "active", "soft", "text", "btn", "btn-hover", "btn-active", "on-accent", "focus"];
  const hex = (v) => typeof v === "string" && /^#[0-9a-fA-F]{6}$/.test(v);
  const accent = window.__MKB_ACCENT__;
  try {
    if (accent && PRESETS.includes(accent.preset)) {
      document.documentElement.dataset.accent = accent.preset;
    } else if (accent && accent.preset === "custom" && hex(accent.hex)) {
      document.documentElement.dataset.accent = "custom";
      const cache = accent.cache;
      for (const theme of ["light", "dark"]) {
        const tokens = cache && cache[theme];
        if (!tokens) continue;
        for (const key of TOKENS) {
          if (hex(tokens[key])) {
            document.documentElement.style.setProperty(`--accent-${theme}-${key}`, tokens[key]);
          }
        }
      }
    }
  } catch {
    // A malformed value is never worth failing the boot over.
  }
})();
