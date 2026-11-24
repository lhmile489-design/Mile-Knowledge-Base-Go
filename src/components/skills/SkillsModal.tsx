import { useEffect, useId, useRef, type MouseEvent, type ReactNode } from "react";
import { X } from "@phosphor-icons/react";

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

const BUSY_TITLE = "进行中，完成后才能关闭";

const TEXT_FIELDS = 'input:not([disabled]):not([type="hidden"]), textarea:not([disabled]), select:not([disabled])';

/** First input, else the first focusable control in the body, else the first
 * one anywhere (the close button), else the dialog itself. */
function initialFocusTarget(card: HTMLElement): HTMLElement {
  const body = card.querySelector<HTMLElement>("[data-modal-body]");
  return (
    card.querySelector<HTMLElement>(TEXT_FIELDS) ??
    body?.querySelector<HTMLElement>(FOCUSABLE) ??
    card.querySelector<HTMLElement>(FOCUSABLE) ??
    card
  );
}

/** Centered dialog shell shared by the install and distribute flows (M9;
 * DESIGN 14.2–14.4, tech design 20.4). The scrim covers the whole skills page
 * panel. While it is open everything else in the panel is inert, Tab cycles
 * inside the dialog, and focus goes back to the opener on close.
 *
 * Esc order: `onBack` first (a content step or a confirm card; return `true`
 * when it handled the key), then `onClose`. While `busy` the dialog cannot be
 * closed at all — Esc, the close button and the scrim do nothing (PRD Q-UX-6).
 * `focusKey` names the current step: when it changes the content was swapped,
 * so focus is placed again as on open (a re-render alone never moves focus). */
export function SkillsModal({
  title,
  step,
  busy = false,
  onClose,
  onBack,
  head,
  footer,
  focusKey,
  children,
}: {
  readonly title: string;
  readonly step?: string;
  readonly busy?: boolean;
  readonly onClose: () => void;
  readonly onBack?: () => boolean | void;
  readonly head?: ReactNode;
  readonly footer?: ReactNode;
  readonly focusKey?: string;
  readonly children: ReactNode;
}) {
  const titleId = useId();
  const scrimRef = useRef<HTMLDivElement>(null);
  const cardRef = useRef<HTMLDivElement>(null);
  const downOnScrim = useRef(false);

  // A new step replaces the content; place focus again (not on first render,
  // the mount effect below already did).
  const lastFocusKey = useRef(focusKey);
  useEffect(() => {
    if (lastFocusKey.current === focusKey) return;
    lastFocusKey.current = focusKey;
    if (cardRef.current) initialFocusTarget(cardRef.current).focus();
  }, [focusKey]);

  // The key handler is bound once; it reads the latest props through here.
  const latest = useRef({ busy, onClose, onBack });
  useEffect(() => {
    latest.current = { busy, onClose, onBack };
  });

  useEffect(() => {
    const scrim = scrimRef.current;
    const card = cardRef.current;
    if (!scrim || !card) return;

    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;

    // Everything else in the skills page panel stops taking input.
    const root = scrim.closest(".skills-page") ?? scrim.parentElement;
    const madeInert: Element[] = [];
    let node: Element = scrim;
    while (node !== root && node.parentElement) {
      const parent: Element = node.parentElement;
      for (const sibling of Array.from(parent.children)) {
        if (sibling !== node && !sibling.hasAttribute("inert")) {
          sibling.setAttribute("inert", "");
          madeInert.push(sibling);
        }
      }
      node = parent;
    }

    const focusables = () => Array.from(card.querySelectorAll<HTMLElement>(FOCUSABLE));
    initialFocusTarget(card).focus();

    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (event.isComposing) return;
        event.preventDefault();
        event.stopPropagation();
        const { busy: running, onClose: close, onBack: back } = latest.current;
        if (running) return;
        if (back?.() === true) return;
        close();
        return;
      }
      if (event.key !== "Tab") return;
      const items = focusables();
      const first = items[0];
      const last = items[items.length - 1];
      if (!first || !last) {
        event.preventDefault();
        card.focus();
        return;
      }
      const active = document.activeElement;
      if (!active || !card.contains(active)) {
        event.preventDefault();
        (event.shiftKey ? last : first).focus();
      } else if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.addEventListener("keydown", onKey, true);

    return () => {
      document.removeEventListener("keydown", onKey, true);
      // Lift inert before focusing: an inert opener cannot take focus.
      for (const element of madeInert) element.removeAttribute("inert");
      if (opener?.isConnected) opener.focus();
    };
  }, []);

  // Only a click that both started and ended on the scrim closes the dialog,
  // so dragging a text selection out of an input does not dismiss it.
  const onScrimDown = (event: MouseEvent<HTMLDivElement>) => {
    downOnScrim.current = event.target === event.currentTarget;
  };
  const onScrimClick = (event: MouseEvent<HTMLDivElement>) => {
    const started = downOnScrim.current;
    downOnScrim.current = false;
    if (busy || !started || event.target !== event.currentTarget) return;
    onClose();
  };

  return (
    <div
      ref={scrimRef}
      className="skills-modal-scrim"
      data-testid="skills-modal"
      onMouseDown={onScrimDown}
      onClick={onScrimClick}
    >
      <div ref={cardRef} className="skills-modal" role="dialog" aria-modal="true" aria-labelledby={titleId} tabIndex={-1}>
        <header className="skills-modal-head">
          <div className="skills-modal-title">
            <h3 id={titleId}>{title}</h3>
            {step ? <span className="skills-modal-step">{step}</span> : null}
            <button
              type="button"
              className="icon-btn"
              aria-label="关闭"
              aria-disabled={busy || undefined}
              title={busy ? BUSY_TITLE : undefined}
              data-testid="skills-modal-close"
              onClick={() => {
                if (!busy) onClose();
              }}
            >
              <X size={14} aria-hidden />
            </button>
          </div>
          {head}
        </header>
        <div className="skills-modal-body" data-modal-body>
          {children}
        </div>
        {footer ? <footer className="skills-modal-foot">{footer}</footer> : null}
      </div>
    </div>
  );
}
