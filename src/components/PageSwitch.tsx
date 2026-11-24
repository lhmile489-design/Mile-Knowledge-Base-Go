import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { SquaresFour, Stack } from "@phosphor-icons/react";
import { badgeCount } from "../lib/skillsView";
import { useAppStore } from "../stores/appStore";
import { useSkillsStore } from "../stores/skillsStore";

/** Inset of the thumb inside the pill track; mirrors `.page-switch` padding. */
const PILL_PAD = 3;

/** Title-bar 「工作台 / 技能」 switch (UI 稿 11.1, M8). The thumb is measured
 * from the live button, so it follows label and font changes; the spring
 * slide lives in page-switch.css and turns off under reduced motion. */
export function PageSwitch() {
  const page = useAppStore((state) => state.page);
  const showPage = useAppStore((state) => state.showPage);
  const settingsOpen = useAppStore((state) => state.settingsOpen);
  const openPanel = useAppStore((state) => state.openSkillsPanel);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const scan = useSkillsStore((state) => state.scan);

  // The badge needs a scan; keep the first one cached-first and cheap, and a
  // vault switch invalidates the knowledge-base half (same as M4–M7).
  useEffect(() => {
    void useSkillsStore.getState().refresh(vaultId);
  }, [vaultId]);

  const navRef = useRef<HTMLElement | null>(null);
  const workRef = useRef<HTMLButtonElement | null>(null);
  const skillsRef = useRef<HTMLButtonElement | null>(null);
  const [thumb, setThumb] = useState<{ width: number; x: number } | null>(null);
  // Transitions stay off for the very first placement, otherwise the thumb
  // would slide in from zero width on mount.
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const frame = requestAnimationFrame(() => setReady(true));
    return () => cancelAnimationFrame(frame);
  }, []);

  const measure = useCallback(() => {
    const button = page === "skills" ? skillsRef.current : workRef.current;
    if (!button) return;
    setThumb({ width: button.offsetWidth, x: button.offsetLeft - PILL_PAD });
  }, [page]);

  useLayoutEffect(() => {
    measure();
  }, [measure]);

  useEffect(() => {
    const nav = navRef.current;
    if (!nav || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => measure());
    observer.observe(nav);
    for (const button of [workRef.current, skillsRef.current]) {
      if (button) observer.observe(button);
    }
    return () => observer.disconnect();
  }, [measure]);

  const count = badgeCount(scan);
  const skillsLabel = count > 0 ? `技能，${count} 项需要处理` : "技能";

  // Transitional until T09 rewrites the e2e entry: with settings open the
  // legacy floating panel still opens above it (AC-SKM-11). T09 replaces this
  // with the showPage leave guard, T14 removes the panel.
  const goSkills = () => {
    if (settingsOpen) {
      openPanel();
      return;
    }
    showPage("skills");
  };

  return (
    <nav ref={navRef} className="page-switch no-drag" aria-label="页面" data-testid="page-switch">
      <span
        aria-hidden
        className={ready ? "page-switch-thumb is-ready" : "page-switch-thumb"}
        style={thumb ? { width: thumb.width, transform: `translateX(${thumb.x}px)` } : { visibility: "hidden" }}
      />
      <button
        type="button"
        ref={workRef}
        className="page-switch-item"
        aria-current={page === "work" ? "page" : undefined}
        onClick={() => showPage("work")}
      >
        <SquaresFour size={14} aria-hidden />
        <span>工作台</span>
      </button>
      <button
        type="button"
        ref={skillsRef}
        className="page-switch-item"
        data-testid="open-skills"
        aria-label={skillsLabel}
        aria-current={page === "skills" ? "page" : undefined}
        onClick={goSkills}
      >
        <Stack size={14} aria-hidden />
        <span>技能</span>
        {count > 0 ? (
          <span className="skill-badge" data-testid="skills-badge" aria-hidden>
            {count}
          </span>
        ) : null}
      </button>
    </nav>
  );
}
