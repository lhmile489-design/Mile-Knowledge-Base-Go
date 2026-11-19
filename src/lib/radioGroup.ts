// Arrow-key movement inside a radiogroup (DESIGN.md §7.2: segmented controls
// are radiogroups). Returns the option to select, or null when the key is not
// ours so the browser keeps its default behaviour.

export function nextOption<T>(options: readonly T[], current: T, key: string): T | null {
  const index = options.indexOf(current);
  if (index < 0) return null;
  switch (key) {
    case "ArrowRight":
    case "ArrowDown":
      return options[(index + 1) % options.length] ?? null;
    case "ArrowLeft":
    case "ArrowUp":
      return options[(index - 1 + options.length) % options.length] ?? null;
    case "Home":
      return options[0] ?? null;
    case "End":
      return options[options.length - 1] ?? null;
    default:
      return null;
  }
}
