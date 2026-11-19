// 记录首帧前后的可观察量，再经绑定回传给 Go 打印。
(() => {
  const t = (window.__PROBE = window.__PROBE || {});
  const color = () => {
    const btn = document.getElementById("btn");
    return btn ? getComputedStyle(btn).backgroundColor : null;
  };
  const finish = () => {
    t.readyState = document.readyState;
    t.btnColor = color();
    t.paints = performance.getEntriesByType("paint").map((e) => ({
      name: e.name,
      startTime: Math.round(e.startTime * 10) / 10,
    }));
    t.themeInitAt = Math.round((t.themeInitAt || 0) * 10) / 10;
    t.preloadAt = t.preloadAt == null ? null : Math.round(t.preloadAt * 10) / 10;
    t.pwned = typeof window.__PWNED__ !== "undefined";
    t.callable = typeof window.mygo !== "undefined" && typeof window.mygo.call === "function";
    try {
      mygo.call("Probe.Report", JSON.stringify(t));
    } catch (e) {
      t.error = String(e);
      document.title = JSON.stringify(t);
    }
  };
  // defer 的脚本在 DOMContentLoaded 之前跑；再等两帧，首帧的 paint 条目就有了。
  const atDCL = () => {
    t.dclAt = Math.round(performance.now() * 10) / 10;
    t.btnColorAtDCL = color();
    requestAnimationFrame(() => requestAnimationFrame(finish));
  };
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", atDCL, { once: true });
  } else {
    atDCL();
  }
})();
