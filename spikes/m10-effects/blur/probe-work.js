// Probe for the work-doc state: computed colours and bounding boxes of text
// on the panel tier, the card tier and the opaque sheet. Consumed by sample.py.
(() => {
  const specs = [
    { name: "panel-primary", tier: "panel", sel: ".side-head b" },
    { name: "panel-muted", tier: "panel", sel: ".side-foot span" },
    { name: "panel-chat-head", tier: "panel", sel: ".chat-head b" },
    { name: "card-muted", tier: "card", sel: ".controls > div:first-child" },
    { name: "card-bar", tier: "card", sel: ".composer .bar", fracX: 0.55 },
    { name: "card-accent", tier: "card", sel: ".seg span[data-on]" },
    { name: "sheet-body", tier: "sheet", sel: ".reader p" },
    { name: "sheet-answer", tier: "sheet", sel: ".answer p" }
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
      fracX: s.fracX || 1,
      rect: { x: r.x, y: r.y, w: r.width, h: r.height }
    });
  }
  return JSON.stringify({
    state: "work-doc",
    theme: document.documentElement.dataset.theme,
    blur: document.documentElement.dataset.blur,
    dpr: window.devicePixelRatio,
    vw: window.innerWidth,
    vh: window.innerHeight,
    samples: out
  });
})()
