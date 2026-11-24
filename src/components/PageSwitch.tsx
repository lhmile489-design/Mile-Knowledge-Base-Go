import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { SquaresFour, Stack } from "@phosphor-icons/react";
import { badgeCount } from "../lib/skillsView";
import { useAppStore } from "../stores/appStore";
import { usePageLockStore } from "../stores/pageLockStore";
import { useSkillsStore } from "../stores/skillsStore";

/** Inset of the thumb inside the pill track; mirrors `.page-switch` padding. */
const PILL_PAD = 3;

/** Title-bar 「工作台 / 技能」 switch (UI 稿 11.1, M8). The thumb is measured
 * from the live button, so it follows label and font changes; the spring
 * slide lives in page-switch.css and turns off under reduced motion. */
export function PageSwitch() {
  const page = useAppStore((state) => state.page);
  const showPage = useAppStore((state) => state.showPage);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const scan = useSkillsStore((state) => state.scan);
  const locked = usePageLockStore((state) => state.locks > 0);
  const lockProps = locked
    ? ({ "aria-disabled": true, title: "先关闭当前弹窗再切换页面" } as const)
    : {};
  const go = (next: "work" | "skills") => {
    if (!locked) showPage(next);
  };

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
        onClick={() => go("work")}
        {...lockProps}
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
        onClick={() => go("skills")}
        {...lockProps}
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
