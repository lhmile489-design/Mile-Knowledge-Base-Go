(() => {
  const cs = (el, props) => {
    if (!el) return null;
    const s = getComputedStyle(el);
    const out = {};
    for (const p of props) out[p] = s.getPropertyValue(p);
    return out;
  };
  const q = (sel) => document.querySelector(sel);
  const rect = (el) => {
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { top: Math.round(r.top), left: Math.round(r.left), width: Math.round(r.width), height: Math.round(r.height) };
  };
  const root = document.documentElement;
  const field = q('.background-field');
  const spans = field ? [...field.querySelectorAll('span')] : [];
  const center = document.elementFromPoint(Math.round(innerWidth / 2), Math.round(innerHeight / 2));
  return JSON.stringify({
    attrs: { theme: root.dataset.theme, accent: root.dataset.accent, motion: root.dataset.motion, blur: root.dataset.blur, blurAuto: root.dataset.blurAuto || null },
    field: field ? { rect: rect(field), style: cs(field, ['position', 'z-index', 'opacity', 'display', 'pointer-events']), spanAnim: spans.map((s) => getComputedStyle(s).animationName) } : null,
    shell: cs(q('.app-shell'), ['isolation', 'position']),
    skillsHost: cs(q('[data-testid=skills-host]'), ['background-color', 'z-index']),
    workbenchHost: cs(q('[data-testid=workbench-host]'), ['opacity', 'pointer-events', 'display']),
    page: cs(q('.skills-page'), ['background-color', 'backdrop-filter', 'box-shadow']),
    card: cs(q('.skills-page .kb-card'), ['background-color', 'backdrop-filter']),
    nav: cs(q('.skills-page .skills-nav'), ['background-color', 'backdrop-filter']),
    centerHit: center ? center.className.toString().slice(0, 60) : null,
  }, null, 2);
})()
