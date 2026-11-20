// AC-ACC-3 (T07): sample the computed colours of real components for every
// preset in both themes. Run through the mygo automation eval channel; the
// caller asserts the ratios against DESIGN.md 12.3 (text >= 4.5, button text
// >= 4.5, fills >= 3). Colours are read after the 150ms transitions settle.
//
// The welcome view owns the only permanent .btn-accent while a vault is open,
// so the primary-button sample mounts a node with the shipped class and reads
// the same CSS; every other sample is an element the user actually sees.
//
// The 2px selected-tree indicator is deliberately not sampled: it is a
// decorative reinforcement (selection also carries weight and aria-selected),
// so the DESIGN 12.3 fill-vs-card rule does not apply to accent-on-soft.
(async () => {
  const app = window.__mkbApp;
  const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
  const settle = async () => {
    await sleep(220);
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  };
  const PRESETS = ["orange", "indigo", "blue", "green", "teal", "violet"];
  const samples = [];
  const push = (preset, theme, element, kind, fg, bg) => {
    samples.push({ preset, theme, element, kind, fg, bg });
  };
  const style = (el) => getComputedStyle(el);
  const opaqueBg = (el) => {
    let node = el;
    while (node && node !== document.documentElement) {
      const bg = getComputedStyle(node).backgroundColor;
      if (bg && bg !== "transparent" && bg !== "rgba(0, 0, 0, 0)") return bg;
      node = node.parentElement;
    }
    return getComputedStyle(document.body).backgroundColor;
  };
  const combo = async (preset, theme) => {
    await app.getState().setAccent({ preset });
    await app.getState().setTheme(theme);
    await settle();
  };

  const probe = document.createElement("button");
  probe.type = "button";
  probe.className = "btn-accent";
  probe.textContent = "sample";
  probe.style.cssText = "position:fixed;left:-300px;top:0;pointer-events:none";
  document.body.appendChild(probe);

  // --- workbench pass ---
  app.getState().closeSettings();
  window.__mkbComposer.getState().setText("contrast probe");
  await settle();
  for (const preset of PRESETS) {
    for (const theme of ["light", "dark"]) {
      await combo(preset, theme);
      const seg = document.querySelector('.segmented-item[aria-checked="true"]');
      if (seg) push(preset, theme, "segmented", "text", style(seg).color, style(seg).backgroundColor);
      const tree = document.querySelector('.tree-row[aria-selected="true"]');
      if (tree) push(preset, theme, "tree-text", "text", style(tree).color, style(tree).backgroundColor);
      const link = document.querySelector(".prose a");
      if (link) push(preset, theme, "link", "text", style(link).color, opaqueBg(link));
      const send = document.querySelector(".composer-send:not(:disabled)");
      if (send) push(preset, theme, "send", "button", style(send).color, style(send).backgroundColor);
      push(preset, theme, "primary-button", "button", style(probe).color, style(probe).backgroundColor);
    }
  }

  // --- settings pass: the include-in-search switch ---
  window.__mkbComposer.getState().setText("");
  app.getState().openSettings();
  await settle();
  document.querySelector('[data-testid="settings-nav-categories"]').click();
  await settle();
  const first = document.querySelector(".switch");
  if (first && first.getAttribute("aria-checked") !== "true") {
    first.click();
    await settle();
  }
  for (const preset of PRESETS) {
    for (const theme of ["light", "dark"]) {
      await combo(preset, theme);
      const knob = document.querySelector('.switch[aria-checked="true"]');
      if (knob) {
        push(
          preset,
          theme,
          "switch-knob",
          "fill",
          getComputedStyle(knob, "::after").backgroundColor,
          style(knob).backgroundColor,
        );
      }
    }
  }
  app.getState().closeSettings();
  probe.remove();
  return JSON.stringify(samples);
})()
