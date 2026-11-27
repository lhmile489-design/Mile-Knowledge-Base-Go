(() => {
  const lines = [];
  const walk = (el, depth) => {
    if (depth > 5) return;
    for (const c of el.children) {
      const r = c.getBoundingClientRect();
      const s = getComputedStyle(c);
      if (r.width < 30 || r.height < 14) continue;
      const cls = (c.className && c.className.toString ? c.className.toString() : '').trim().split(/\s+/).slice(0, 3).join('.');
      lines.push('  '.repeat(depth) + c.tagName.toLowerCase() + (cls ? '.' + cls : '') + ' ' + Math.round(r.width) + 'x' + Math.round(r.height) + ' bg=' + s.backgroundColor.replace('rgba(0, 0, 0, 0)', '-') + ' ov=' + s.overflowY);
      walk(c, depth + 1);
    }
  };
  for (const p of document.querySelectorAll('.workbench-panel')) {
    lines.push('## ' + p.getAttribute('data-panel'));
    walk(p, 1);
  }
  return lines.join('\n');
})()
