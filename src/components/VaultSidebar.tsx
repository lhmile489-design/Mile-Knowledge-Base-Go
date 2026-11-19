import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  Archive,
  CaretRight,
  ClockCounterClockwise,
  FilePlus,
  FolderPlus,
  GearSix,
  MagnifyingGlass,
  PencilSimple,
  PuzzlePiece,
  Tag,
  Trash,
} from "@phosphor-icons/react";
import { useTreeStore } from "../stores/treeStore";
import { useAppStore } from "../stores/appStore";
import { useDocStore } from "../stores/docStore";
import { useSearchStore } from "../stores/searchStore";
import { useMutationStore } from "../stores/mutationStore";
import { events, mutationApi, settingsApi } from "../lib/transport";
import type { FileEntry, PlanResult } from "../mygo";
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
const DRAG_MIME = "application/x-mile-path";

type Row =
  | { kind: "area"; key: string; area: string; count: number }
  | { kind: "entry"; key: string; entry: FileEntry; depth: number }
  | { kind: "create"; key: string; parent: string; depth: number; directory: boolean };

type Editing =
  | { mode: "rename"; path: string; initial: string }
  | { mode: "alias"; path: string; initial: string }
  | { mode: "createFile"; parent: string }
  | { mode: "createDir"; parent: string }
  | null;

function flatten(nodes: Record<string, FileEntry[]>, expanded: Record<string, boolean>, editing: Editing): Row[] {
  const out: Row[] = [];

  const maybeCreate = (parent: string, depth: number) => {
    if (!editing || (editing.mode !== "createFile" && editing.mode !== "createDir")) return;
    if (editing.parent !== parent) return;
    out.push({
      kind: "create",
      key: `create:${parent}`,
      parent,
      depth,
      directory: editing.mode === "createDir",
    });
  };

  const walk = (path: string, depth: number): void => {
    maybeCreate(path, depth);
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
  maybeCreate("", 1);
  return out;
}

/** The vault tree: lazy loading, area sections at the root, semantic icons,
 * aliases, hover actions, inline rename/create/alias, drag-to-move and a
 * virtualised list so a 10k-entry directory scrolls at 60fps (PRD FR-TREE). */
export function VaultSidebar() {
  const auditOpen = useAppStore((state) => state.auditOpen);
  const toggleAudit = useAppStore((state) => state.toggleAudit);
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
  const openDoc = useDocStore((state) => state.open);
  const setLayout = useAppStore((state) => state.setLayout);

  const [editing, setEditing] = useState<Editing>(null);
  const [dropTarget, setDropTarget] = useState<string | null>(null);

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

  const rows = useMemo(() => flatten(nodes, expanded, editing), [nodes, expanded, editing]);

  const scrollRef = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  });

  const notify = (message: string) => useAppStore.getState().pushNotice(message, "warn");

  const proposePlan = async (pending: Promise<PlanResult>, title: string) => {
    try {
      const result = await pending;
      if (result.plan) {
        useMutationStore.getState().propose(result.plan, title);
      } else if (result.failure) {
        notify(result.failure.message);
      }
    } catch (err) {
      notify(err instanceof Error ? err.message : String(err));
    }
  };

  const onRowActivate = (row: Row) => {
    if (row.kind !== "entry") return;
    if (row.entry.kind === "directory") {
      void toggle(row.entry.path);
    } else {
      select(row.entry.path);
      if (vaultId) {
        void openDoc(vaultId, row.entry.path);
        void setLayout({ docCollapsed: false });
      }
    }
  };

  /** New entries go into the selected directory, or the selected file's
   * parent, or the vault root. */
  const creationParent = (): string => {
    if (!selected) return "";
    for (const entries of Object.values(nodes)) {
      const hit = entries.find((entry) => entry.path === selected);
      if (!hit) continue;
      if (hit.kind === "directory") return hit.path;
      const at = hit.path.lastIndexOf("/");
      return at >= 0 ? hit.path.slice(0, at) : "";
    }
    return "";
  };

  const commitRename = (path: string, next: string) => {
    setEditing(null);
    if (!vaultId) return;
    void proposePlan(mutationApi.rename(vaultId, path, next), `重命名为 ${next}`);
  };

  const commitAlias = async (path: string, next: string) => {
    setEditing(null);
    if (!vaultId) return;
    try {
      await settingsApi.appConfigPatchAliases(vaultId, { [path]: next.trim() === "" ? null : next.trim() });
      await invalidate([parentOf(path), path]);
    } catch (err) {
      notify(err instanceof Error ? err.message : String(err));
    }
  };

  const commitCreate = (parent: string, directory: boolean, name: string) => {
    setEditing(null);
    if (!vaultId) return;
    const pending = directory
      ? mutationApi.createDir(vaultId, parent, name)
      : mutationApi.createFile(vaultId, parent, name);
    void proposePlan(pending, directory ? `新建目录 ${name}` : `新建文件 ${name}`);
  };

  const archive = (path: string) => {
    if (!vaultId) return;
    void proposePlan(
      mutationApi.plan({
        vaultId,
        origin: "user",
        actions: [{ id: "archive", kind: "archive", source: path, reason: "用户归档" }],
      }),
      `归档 ${path}`,
    );
  };

  const remove = (path: string) => {
    if (!vaultId) return;
    void proposePlan(
      mutationApi.plan({
        vaultId,
        origin: "user",
        actions: [{ id: "trash", kind: "trash", source: path, reason: "用户删除" }],
      }),
      `移入回收站：${path}`,
    );
  };

  return (
    <div className="vault-sidebar">
      <div className="vault-sidebar-head">
        <span className="vault-sidebar-name" title={vault?.root}>
          {vault?.displayName ?? "知识库"}
        </span>
        <button
          type="button"
          className="vault-sidebar-search"
          onClick={() => {
            void setLayout({ docCollapsed: false });
            useSearchStore.getState().openPanel();
          }}
        >
          <MagnifyingGlass size={12} aria-hidden />
          <span>搜索</span>
          <kbd>Ctrl P</kbd>
        </button>
        <div className="vault-sidebar-new">
          <button
            type="button"
            className="icon-btn"
            aria-label="新建文件"
            title="新建文件"
            onClick={() => setEditing({ mode: "createFile", parent: creationParent() })}
          >
            <FilePlus size={14} aria-hidden />
          </button>
          <button
            type="button"
            className="icon-btn"
            aria-label="新建目录"
            title="新建目录"
            onClick={() => setEditing({ mode: "createDir", parent: creationParent() })}
          >
            <FolderPlus size={14} aria-hidden />
          </button>
          <button
            type="button"
            className="icon-btn"
            aria-label="设置"
            title="设置"
            onClick={() => useAppStore.getState().openSettings()}
            data-testid="open-settings"
          >
            <GearSix size={14} aria-hidden />
          </button>
        </div>
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
                ) : row.kind === "create" ? (
                  <div className="tree-row tree-row-editing" style={{ paddingLeft: 8 + row.depth * 14 }}>
                    <InlineInput
                      placeholder={row.directory ? "目录名" : "文件名"}
                      onCommit={(value) => commitCreate(row.parent, row.directory, value)}
                      onCancel={() => setEditing(null)}
                    />
                  </div>
                ) : (
                  <TreeRow
                    entry={row.entry}
                    depth={row.depth}
                    open={!!expanded[row.entry.path]}
                    active={selected === row.entry.path}
                    dropTarget={dropTarget === row.entry.path}
                    editing={editing && "path" in editing && editing.path === row.entry.path ? editing : null}
                    onActivate={() => onRowActivate(row)}
                    onRename={(name) => commitRename(row.entry.path, name)}
                    onAlias={(name) => void commitAlias(row.entry.path, name)}
                    onCancelEdit={() => setEditing(null)}
                    onStartRename={() => setEditing({ mode: "rename", path: row.entry.path, initial: row.entry.name })}
                    onStartAlias={() => setEditing({ mode: "alias", path: row.entry.path, initial: row.entry.alias ?? "" })}
                    onArchive={() => archive(row.entry.path)}
                    onDelete={() => remove(row.entry.path)}
                    onDropInto={async (source) => {
                      setDropTarget(null);
                      if (!vaultId || row.entry.kind !== "directory" || source === row.entry.path) return;
                      await proposePlan(
                        mutationApi.planMove(vaultId, [source], row.entry.path),
                        `移动 ${source} → ${row.entry.path}`,
                      );
                    }}
                    onDragOverRow={() => {
                      if (row.entry.kind === "directory") setDropTarget(row.entry.path);
                    }}
                    onDragLeaveRow={() => setDropTarget((current) => (current === row.entry.path ? null : current))}
                  />
                )}
              </div>
            );
          })}
        </div>
      </div>

      <div className="vault-sidebar-foot">
        <button
          type="button"
          className="vault-sidebar-foot-btn"
          aria-pressed={auditOpen}
          onClick={() => toggleAudit()}
        >
          <ClockCounterClockwise size={13} aria-hidden />
          <span>操作日志</span>
        </button>
        <button
          type="button"
          className="vault-sidebar-foot-btn"
          onClick={() => useAppStore.getState().openSkillWizard()}
          disabled={!vaultId}
          title="导入技能"
          data-testid="open-skill-import"
        >
          <PuzzlePiece size={13} aria-hidden />
          <span>导入技能</span>
        </button>
      </div>
    </div>
  );
}

function parentOf(path: string): string {
  const at = path.lastIndexOf("/");
  return at >= 0 ? path.slice(0, at) : "";
}

type InlineInputProps = {
  initial?: string;
  placeholder?: string;
  selectStem?: boolean;
  onCommit: (value: string) => void;
  onCancel: () => void;
};

function InlineInput({ initial = "", placeholder, selectStem, onCommit, onCancel }: InlineInputProps) {
  const ref = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    const input = ref.current;
    if (!input) return;
    input.focus();
    if (selectStem) {
      // Renaming preselects the stem, not the extension (PRD FR-TREE).
      const dot = initial.lastIndexOf(".");
      input.setSelectionRange(0, dot > 0 ? dot : initial.length);
    } else {
      input.select();
    }
  }, [initial, selectStem]);

  return (
    <input
      ref={ref}
      className="tree-inline-input"
      defaultValue={initial}
      placeholder={placeholder}
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => {
        if (event.key === "Enter") {
          event.preventDefault();
          const value = event.currentTarget.value.trim();
          if (value === "") onCancel();
          else onCommit(value);
        } else if (event.key === "Escape") {
          event.preventDefault();
          onCancel();
        }
      }}
      onBlur={(event) => {
        const value = event.currentTarget.value.trim();
        if (value === initial) onCancel();
        else if (value === "") onCancel();
        else onCommit(value);
      }}
    />
  );
}

type TreeRowProps = {
  entry: FileEntry;
  depth: number;
  open: boolean;
  active: boolean;
  dropTarget: boolean;
  editing: { mode: "rename" | "alias"; initial: string } | null;
  onActivate: () => void;
  onRename: (name: string) => void;
  onAlias: (name: string) => void;
  onCancelEdit: () => void;
  onStartRename: () => void;
  onStartAlias: () => void;
  onArchive: () => void;
  onDelete: () => void;
  onDropInto: (source: string) => Promise<void>;
  onDragOverRow: () => void;
  onDragLeaveRow: () => void;
};

function TreeRow(props: TreeRowProps) {
  const { entry, depth, open, active, dropTarget, editing } = props;
  const isDir = entry.kind === "directory";
  const Icon = iconFor(entry.icon ?? (isDir ? "folder" : "file"));
  return (
    <div
      role="treeitem"
      aria-level={depth + 1}
      aria-selected={active}
      aria-expanded={isDir ? open : undefined}
      className="tree-row"
      data-drop={dropTarget || undefined}
      style={{ paddingLeft: 8 + depth * 14 }}
      draggable={!editing}
      onDragStart={(event) => {
        event.dataTransfer.setData(DRAG_MIME, entry.path);
        event.dataTransfer.setData("text/plain", entry.path);
        event.dataTransfer.effectAllowed = "move";
      }}
      onDragOver={(event) => {
        if (!isDir) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        props.onDragOverRow();
      }}
      onDragLeave={() => props.onDragLeaveRow()}
      onDrop={(event) => {
        if (!isDir) return;
        event.preventDefault();
        const source = event.dataTransfer.getData(DRAG_MIME) || event.dataTransfer.getData("text/plain");
        if (source) void props.onDropInto(source);
      }}
      onClick={props.onActivate}
      title={entry.path}
    >
      <span className="tree-caret" data-open={open} aria-hidden>
        {isDir ? <CaretRight size={11} weight="bold" /> : null}
      </span>
      <span className="tree-icon" aria-hidden>
        <Icon size={14} weight={active ? "fill" : "regular"} />
      </span>
      {editing?.mode === "rename" ? (
        <InlineInput
          initial={editing.initial}
          selectStem
          onCommit={props.onRename}
          onCancel={props.onCancelEdit}
        />
      ) : editing?.mode === "alias" ? (
        <InlineInput
          initial={editing.initial}
          placeholder="别名（留空清除）"
          onCommit={props.onAlias}
          onCancel={props.onCancelEdit}
        />
      ) : (
        <span className="tree-label">{entry.alias || entry.name}</span>
      )}
      {!editing ? (
        <span className="tree-actions">
          <button
            type="button"
            className="icon-btn"
            aria-label="重命名"
            title="重命名"
            onClick={(event) => {
              event.stopPropagation();
              props.onStartRename();
            }}
          >
            <PencilSimple size={12} aria-hidden />
          </button>
          {isDir ? (
            <button
              type="button"
              className="icon-btn"
              aria-label="编辑别名"
              title="编辑别名"
              onClick={(event) => {
                event.stopPropagation();
                props.onStartAlias();
              }}
            >
              <Tag size={12} aria-hidden />
            </button>
          ) : null}
          <button
            type="button"
            className="icon-btn"
            aria-label="归档"
            title="归档"
            onClick={(event) => {
              event.stopPropagation();
              props.onArchive();
            }}
          >
            <Archive size={12} aria-hidden />
          </button>
          <button
            type="button"
            className="icon-btn"
            aria-label="删除"
            title="移入回收站"
            onClick={(event) => {
              event.stopPropagation();
              props.onDelete();
            }}
          >
            <Trash size={12} aria-hidden />
          </button>
        </span>
      ) : null}
    </div>
  );
}
