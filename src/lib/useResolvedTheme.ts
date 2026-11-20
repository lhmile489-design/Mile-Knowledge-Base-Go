import { useEffect, useState } from "react";

export type ThemeSetting = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

/** The theme that is actually in effect right now (system follows the OS). */
export function useResolvedTheme(mode: ThemeSetting): ResolvedTheme {
  const resolve = (): ResolvedTheme => {
    if (mode !== "system") return mode;
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  };
  const [resolved, setResolved] = useState<ResolvedTheme>(resolve);
  useEffect(() => {
    if (mode !== "system") {
      setResolved(mode);
      return;
    }
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = (): void => setResolved(media.matches ? "dark" : "light");
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [mode]);
  return resolved;
}
