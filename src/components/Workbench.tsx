import { useCallback, useEffect, useRef, useState } from "react";
import { SidebarSimple } from "@phosphor-icons/react";
import { TitleBar } from "./TitleBar";
import { VaultSidebar } from "./VaultSidebar";
import { EditorPane } from "./EditorPane";
import { useDocStore } from "../stores/docStore";
import { events } from "../lib/transport";
import { useAppStore } from "../stores/appStore";

const SIDEBAR_MIN = 192;
const SIDEBAR_MAX = 320;
const DOC_MIN = 320;
const DOC_MAX = 800;
const DOC_RAIL_WIDTH = 40;
const CONVERSATION_MIN_WIDTH = 480;
const GUTTERS = 32;
const KEYBOARD_STEP = 8;

type ResizerProps = {
  label: string;
  value: number;
  bounds: { min: number; max: number };
  direction: 1 | -1;
  onChange: (next: number) => void;
  onCommit: (next: number) => void;
};

/** A seam between panels: drag with the pointer or nudge with ←/→ (a11y). */
function Resizer({ label, value, bounds, direction, onChange, onCommit }: ResizerProps) {
  const dragging = useRef<{ startX: number; startValue: number } | null>(null);

  const clamp = useCallback(
    (next: number) => Math.min(bounds.max, Math.max(bounds.min, Math.round(next))),
    [bounds.max, bounds.min],
  );

  return (
    <button
      type="button"
      role="separator"
      aria-label={label}
      aria-orientation="vertical"
      aria-valuenow={value}
      aria-valuemin={bounds.min}
      aria-valuemax={bounds.max}
      className="resizer"
      onPointerDown={(event) => {
        dragging.current = { startX: event.clientX, startValue: value };
        event.currentTarget.setPointerCapture(event.pointerId);
      }}
      onPointerMove={(event) => {
        const state = dragging.current;
        if (!state) return;
        onChange(clamp(state.startValue + (event.clientX - state.startX) * direction));
      }}
      onPointerUp={(event) => {
        if (!dragging.current) return;
        dragging.current = null;
        event.currentTarget.releasePointerCapture(event.pointerId);
        onCommit(value);
      }}
      onKeyDown={(event) => {
        if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
        event.preventDefault();
        const delta = (event.key === "ArrowRight" ? KEYBOARD_STEP : -KEYBOARD_STEP) * direction;
        const next = clamp(value + delta);
        onChange(next);
        onCommit(next);
      }}
    />
  );
}

/** Three-column workbench: sidebar | conversation | document panel. The
 * 8px seams are the drag handles; the document panel collapses to a 40px rail
 * (DESIGN.md §4, PRD FR-SET). */
export function Workbench() {
  const layout = useAppStore((state) => state.settings?.layout);
  const setLayout = useAppStore((state) => state.setLayout);
  const resetDocs = useDocStore((state) => state.reset);
  const vaultId = useAppStore((state) => state.activeVaultId);

  const [viewportWidth, setViewportWidth] = useState(window.innerWidth);
  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const sidebarWidth = layout?.sidebarWidth ?? 232;
  const docWidth = layout?.docWidth ?? 560;
  const docCollapsed = layout?.docCollapsed ?? true;

  // The conversation keeps its reading width; the doc panel gives way first.
  const docRoom = viewportWidth - sidebarWidth - CONVERSATION_MIN_WIDTH - GUTTERS;
  const effectiveDocWidth = Math.max(DOC_MIN, Math.min(docWidth, docRoom));
  const rightWidth = docCollapsed ? DOC_RAIL_WIDTH : effectiveDocWidth;

  useEffect(() => {
    resetDocs();
  }, [vaultId, resetDocs]);

  // External changes: the tree refreshes itself; open documents re-read here.
  useEffect(() => {
    if (!vaultId) return;
    const off = events.vaultChanged.on((event) => {
      if (event.vaultId === vaultId) {
        void useDocStore.getState().applyExternalChange(vaultId, event.directories);
      }
    });
    return () => {
      off();
    };
  }, [vaultId]);

  const [draftSidebar, setDraftSidebar] = useState<number | null>(null);
  const [draftDoc, setDraftDoc] = useState<number | null>(null);

  const activeSidebar = draftSidebar ?? sidebarWidth;
  const activeDoc = draftDoc ?? rightWidth;

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.ctrlKey && event.key === "\\") {
        event.preventDefault();
        void setLayout({ docCollapsed: !docCollapsed });
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [docCollapsed, setLayout]);

  return (
    <div className="app-shell" style={{ gridTemplateRows: "var(--titlebar-h) minmax(0, 1fr)" }}>
      <TitleBar />
      <div
        className="workbench-grid"
        style={{
          gridTemplateColumns: `${activeSidebar}px var(--gap-panel) minmax(0, 1fr) var(--gap-panel) ${activeDoc}px`,
        }}
      >
        <aside className="workbench-panel" data-panel="sidebar" aria-label="知识库">
          <VaultSidebar />
        </aside>

        <Resizer
          label="调整左栏宽度"
          value={activeSidebar}
          bounds={{ min: SIDEBAR_MIN, max: SIDEBAR_MAX }}
          direction={1}
          onChange={setDraftSidebar}
          onCommit={(next) => {
            setDraftSidebar(null);
            void setLayout({ sidebarWidth: next });
          }}
        />

        <section className="workbench-panel" data-panel="agnes" aria-label="对话">
          <div className="panel-placeholder">
            <span className="eyebrow">Agnes</span>
            <p className="muted">对话面板将在 M2 接入。</p>
          </div>
        </section>

        <Resizer
          label="调整文档面板宽度"
          value={activeDoc}
          bounds={{ min: DOC_MIN, max: DOC_MAX }}
          direction={-1}
          onChange={setDraftDoc}
          onCommit={(next) => {
            setDraftDoc(null);
            void setLayout({ docWidth: next });
          }}
        />

        {docCollapsed ? (
          <aside className="workbench-panel panel-rail" data-panel="doc-rail" aria-label="文档面板入口">
            <button
              type="button"
              className="icon-btn"
              aria-label="展开文档面板"
              onClick={() => void setLayout({ docCollapsed: false })}
            >
              <SidebarSimple size={16} aria-hidden />
            </button>
          </aside>
        ) : (
          <section className="workbench-panel" data-panel="editor" aria-label="文档">
            <EditorPane />
          </section>
        )}
      </div>
    </div>
  );
}
