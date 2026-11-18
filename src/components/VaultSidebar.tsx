import { useEffect, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { CaretRight } from "@phosphor-icons/react";
import { useTreeStore } from "../stores/treeStore";
import { useAppStore } from "../stores/appStore";
import { events } from "../lib/transport";
import type { FileEntry } from "../mygo";
import { iconFor } from "./treeIcons";

const AREA_LABEL: Record<string, { cn: string; latin: string }> = {
  skills: { cn: "技能", latin: "SKILLS" },
  raws: { cn: "暂存", latin: "STAGING" },
  projects: { cn: "项目", latin: "PROJECTS" },
  tools: { cn: "工具", latin: "TOOLS" },
  wiki: { cn: "独立知识库", latin: "WIKI" },
  constraints: { cn: "约束", latin: "CONSTRAINTS" },
  archives: { cn: "归档", latin: "ARCHIVES" },
  other: { cn: "其他目录", latin: "OTHER" },
  system: { cn: "系统配置", latin: "SYSTEM" },
};

const AREA_ORDER = ["skills", "raws", "projects", "tools", "wiki", "constraints", "archives", "other", "system"];
const ROW_HEIGHT = 28;

type Row =
  | { kind: "area"; key: string; area: string; count: number }
  | { kind: "entry"; key: string; entry: FileEntry; depth: number };

function flatten(nodes: Record<string, FileEntry[]>, expanded: Record<string, boolean>): Row[] {
  const out: Row[] = [];

  const walk = (path: string, depth: number): void => {
    const entries = nodes[path];
    if (!entries) return;
    for (const entry of entries) {
      out.push({ kind: "entry", key: entry.path, entry, depth });
      if (entry.kind === "directory" && expanded[entry.path] && nodes[entry.path]) {
        walk(entry.path, depth + 1);
      }
    }
  };

  const root = nodes[""];
  if (!root) return out;
  const grouped = new Map<string, FileEntry[]>();
  for (const entry of root) {
    const list = grouped.get(entry.area) ?? [];
    list.push(entry);
    grouped.set(entry.area, list);
  }
  for (const area of AREA_ORDER) {
    const list = grouped.get(area);
    if (!list || list.length === 0) continue;
    out.push({ kind: "area", key: `area:${area}`, area, count: list.length });
    for (const entry of list) {
      out.push({ kind: "entry", key: entry.path, entry, depth: 1 });
      if (entry.kind === "directory" && expanded[entry.path] && nodes[entry.path]) {
        walk(entry.path, 2);
      }
    }
  }
  return out;
}

/** The vault tree: lazy per-directory loading, area sections at the root,
 * semantic icons, aliases, and a virtualised row list so a 10k-entry
 * directory scrolls at 60fps (PRD FR-TREE, NFR-PERF). */
export function VaultSidebar() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const vault = useAppStore((state) => state.vaults.find((v) => v.id === state.activeVaultId));
  const nodes = useTreeStore((state) => state.nodes);
  const expanded = useTreeStore((state) => state.expanded);
  const loading = useTreeStore((state) => state.loading);
  const error = useTreeStore((state) => state.error);
  const selected = useTreeStore((state) => state.selected);
  const skillCategories = useTreeStore((state) => state.skillCategories);
  const reset = useTreeStore((state) => state.reset);
  const toggle = useTreeStore((state) => state.toggle);
  const select = useTreeStore((state) => state.select);
  const invalidate = useTreeStore((state) => state.invalidate);

  useEffect(() => {
    if (vaultId) void reset(vaultId);
  }, [vaultId, reset]);

  useEffect(() => {
    const off = events.vaultChanged.on((event) => {
      if (event.vaultId === vaultId) void invalidate(event.directories);
    });
    return () => {
      off();
    };
  }, [vaultId, invalidate]);

  const rows = useMemo(() => flatten(nodes, expanded), [nodes, expanded]);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  const onRowActivate = (row: Row) => {
    if (row.kind !== "entry") return;
    if (row.entry.kind === "directory") {
      void toggle(row.entry.path);
    } else {
      select(row.entry.path);
    }
  };

  return (
    <div className="vault-sidebar">
      <div className="vault-sidebar-head">
        <span className="vault-sidebar-name" title={vault?.root}>
          {vault?.displayName ?? "知识库"}
        </span>
      </div>

      <div ref={scrollRef} className="tree-scroll" role="tree" aria-label="知识库目录">
        {rows.length === 0 && !loading[""] ? (
          <p className="tree-empty muted">{error ?? (Object.keys(nodes).length ? "目录为空" : "正在读取目录…")}</p>
        ) : null}
        <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
          {virtualizer.getVirtualItems().map((item) => {
            const row = rows[item.index];
            if (!row) return null;
            return (
              <div
                key={row.key}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  right: 0,
                  height: item.size,
                  transform: `translateY(${item.start}px)`,
                }}
              >
                {row.kind === "area" ? (
                  <div className="tree-area eyebrow">
                    <span>{AREA_LABEL[row.area]?.cn ?? row.area}</span>
                    <span className="tree-area-latin">{AREA_LABEL[row.area]?.latin ?? ""}</span>
                    <span className="tree-area-count">{row.area === "skills" && skillCategories > 0 ? skillCategories : row.count}</span>
                  </div>
                ) : (
                  <TreeRow
                    entry={row.entry}
                    depth={row.depth}
                    open={!!expanded[row.entry.path]}
                    active={selected === row.entry.path}
                    onActivate={() => onRowActivate(row)}
                  />
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

type TreeRowProps = {
  entry: FileEntry;
  depth: number;
  open: boolean;
  active: boolean;
  onActivate: () => void;
};

function TreeRow({ entry, depth, open, active, onActivate }: TreeRowProps) {
  const isDir = entry.kind === "directory";
  const Icon = iconFor(entry.icon ?? (isDir ? "folder" : "file"));
  return (
    <button
      type="button"
      role="treeitem"
      aria-level={depth + 1}
      aria-selected={active}
      aria-expanded={isDir ? open : undefined}
      className="tree-row"
      style={{ paddingLeft: 8 + depth * 14 }}
      onClick={onActivate}
      onDoubleClick={onActivate}
      title={entry.path}
    >
      <span className="tree-caret" data-open={open} aria-hidden>
        {isDir ? <CaretRight size={11} weight="bold" /> : null}
      </span>
      <span className="tree-icon" aria-hidden>
        <Icon size={14} weight={active ? "fill" : "regular"} />
      </span>
      <span className="tree-label">{entry.alias || entry.name}</span>
      {entry.ignored ? <span className="tree-badge">未索引</span> : null}
      {isDir && entry.childCount !== undefined ? <span className="tree-count">{entry.childCount}</span> : null}
    </button>
  );
}
