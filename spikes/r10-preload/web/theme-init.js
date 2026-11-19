// 仿 public/theme-init.js：读到注入的值后，在首帧前设 data-accent。
(() => {
  const t = (window.__PROBE = window.__PROBE || {});
  t.themeInitAt = performance.now();
  t.preloadPresent = typeof window.__MKB_ACCENT__ !== "undefined";
  t.preloadAt = window.__PRELOAD_AT ?? null;
  t.inlineBlocked = typeof window.__INLINE_RAN__ === "undefined";
  let accent = "orange";
  if (t.preloadPresent) {
    const v = window.__MKB_ACCENT__;
    if (v && typeof v.preset === "string") accent = v.preset;
    // 只记录原样值，用来验证 </script> 之类的字符不会破坏脚本。
    t.note = v && v.note;
  }
  document.documentElement.dataset.accent = accent;
  document.documentElement.style.colorScheme = "light";
  new PerformanceObserver((list) => {
    for (const e of list.getEntries()) t[e.name] = e.startTime;
  }).observe({ type: "paint", buffered: true });
  // WebView2 没有 Paint Timing；用第一次 rAF 采样代替：它在第一帧绘制前解析样式。
  requestAnimationFrame(() => {
    t.firstRafAt = Math.round(performance.now() * 10) / 10;
    const btn = document.getElementById("btn");
    t.firstRafColor = btn ? getComputedStyle(btn).backgroundColor : "no-btn-yet";
  });
})();
