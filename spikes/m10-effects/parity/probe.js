// M10 T02 parity probe: computed values of the skills-page material that the
// refactor moved from local pilot tokens to global effects.css tokens. Run in
// the real window through the automation eval channel; the driver compares
// the JSON from the 1.7.0 build and the migrated build.
(() => {
  const page = document.querySelector(".skills-page");
  if (!page) return JSON.stringify({ error: "no skills page" });
  const style = (el) => getComputedStyle(el);
  const pick = (el, props) => Object.fromEntries(props.map((p) => [p, style(el).getPropertyValue(p)]));
  const tokens = {};
  for (const token of [
    "--glass-blur",
    "--glass-card",
    "--glass-card-hover",
    "--glass-strong",
    "--glass-edge",
    "--field-a",
    "--field-b",
    "--field-c",
    "--dur-enter",
    "--dur-slide",
    "--dur-hover",
    "--stagger",
    "--ease-out-expo",
    "--ease-spring-soft",
  ]) {
    tokens[token] = style(page).getPropertyValue(token).trim();
  }
  const card = document.querySelector(".kb-card");
  const nav = document.querySelector(".skills-nav");
  const field = document.querySelector(".skills-field span");
  return JSON.stringify({
    theme: document.documentElement.dataset.theme,
    blur: document.documentElement.dataset.blur ?? null,
    motion: document.documentElement.dataset.motion ?? null,
    tokens,
    page: pick(page, ["background-color"]),
    nav: nav ? pick(nav, ["background-color", "border-right-color"]) : null,
    card: card
      ? pick(card, ["background-color", "backdrop-filter", "border-top-color", "box-shadow", "border-radius"])
      : null,
    field: field ? pick(field, ["background-image", "filter", "opacity", "animation-name", "animation-duration"]) : null,
  });
})()
