import { useEffect } from "react";
import { useAppStore } from "../stores/appStore";

/**
 * The window-level colour field behind every page (DESIGN 15.3). It is not
 * rendered at blur level off or after the automatic degrade, and its drift is
 * paused through a root attribute while the page is hidden so a minimized
 * window does not keep compositing it (AC-MAT-9).
 */
export function BackgroundField() {
  const blur = useAppStore((state) => state.settings?.effects.blur);
  const degraded = useAppStore((state) => state.effectsDegraded);

  useEffect(() => {
    const root = document.documentElement;
    const sync = (): void => {
      if (document.visibilityState === "hidden") root.setAttribute("data-field-paused", "");
      else root.removeAttribute("data-field-paused");
    };
    sync();
    document.addEventListener("visibilitychange", sync);
    return () => {
      document.removeEventListener("visibilitychange", sync);
      root.removeAttribute("data-field-paused");
    };
  }, []);

  if (blur === "off" || degraded) return null;
  return (
    <div className="background-field" data-testid="background-field" aria-hidden="true">
      <span />
      <span />
      <span />
    </div>
  );
}
