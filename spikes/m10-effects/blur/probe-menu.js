// Probe for the menu state: the strong tier (menus / popovers).
(() => {
  const specs = [
    { name: "strong-accent", tier: "strong", sel: ".menu .mi[data-on]" },
    { name: "strong-text", tier: "strong", sel: ".menu .mi:nth-child(2)" },
    { name: "strong-info", tier: "strong", sel: ".menu .mi:last-child" }
  ];
  const out = [];
  for (const s of specs) {
    const el = document.querySelector(s.sel);
    if (!el) {
      out.push({ name: s.name, tier: s.tier, missing: true });
      continue;
    }
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    out.push({
      name: s.name,
      tier: s.tier,
      color: cs.color,
      rect: { x: r.x, y: r.y, w: r.width, h: r.height }
    });
  }
  return JSON.stringify({
    state: "menu",
    theme: document.documentElement.dataset.theme,
    blur: document.documentElement.dataset.blur,
    dpr: window.devicePixelRatio,
    vw: window.innerWidth,
    vh: window.innerHeight,
    samples: out
  });
})()
