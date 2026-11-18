// Runs before the app mounts, so the first paint already has the right theme
// (PRD FR-THEME). Kept as a separate file because production CSP forbids inline
// scripts. The Go side sets Theme source and the window background color; this
// script only mirrors the resolved appearance onto <html>.
(() => {
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const apply = () => {
    const theme = media.matches ? "dark" : "light";
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  };
  apply();
  media.addEventListener("change", apply);
})();
