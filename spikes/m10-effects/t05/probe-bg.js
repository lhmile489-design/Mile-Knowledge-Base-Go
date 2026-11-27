(() => {
  const out = [];
  const roots = [...document.querySelectorAll('.workbench-panel, header.drag-region, .app-shell')];
  const seen = new Set();
  for (const root of roots) {
    for (const el of [root, ...root.querySelectorAll('*')]) {
      if (seen.has(el)) continue;
      seen.add(el);
      const s = getComputedStyle(el);
      const r = el.getBoundingClientRect();
      if (r.width < 24 || r.height < 16) continue;
      const bg = s.backgroundColor;
      const transparent = bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent';
      if (transparent && s.backdropFilter === 'none') continue;
      const cls = (el.className && el.className.toString ? el.className.toString() : '').slice(0, 60);
      const panel = el.closest('[data-panel]')?.getAttribute('data-panel') || (el.tagName === 'HEADER' ? 'titlebar' : '-');
      out.push([panel, el.tagName.toLowerCase() + (cls ? '.' + cls.trim().split(/\s+/).join('.') : ''), Math.round(r.width) + 'x' + Math.round(r.height) + '@' + Math.round(r.left) + ',' + Math.round(r.top), bg, s.backdropFilter === 'none' ? '' : s.backdropFilter, s.borderTopWidth + ' ' + s.borderTopColor, s.borderRadius, s.boxShadow === 'none' ? '' : s.boxShadow.slice(0, 60)].join(' | '));
    }
  }
  return out.join('\n');
})()
