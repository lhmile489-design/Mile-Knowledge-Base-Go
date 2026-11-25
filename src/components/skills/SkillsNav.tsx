import { useEffect, useRef, useState, type ReactNode } from "react";
import { CaretDown, FolderSimple, MagnifyingGlass, SlidersHorizontal, SquaresFour, Stack, X } from "@phosphor-icons/react";
import type { SkillScan, SkillTool, SkillUpdateCheckResult } from "../../lib/transport";
import { checkedLabel, clockOf } from "../../lib/skillRemote";
import {
  attentionCounts,
  distributionProgress,
  filterParts,
  isToolDetected,
  toolNeedsAttention,
  totals,
} from "../../lib/skillsView";
import { ToolIcon } from "./ToolIcon";

/** What the skills page is showing. T04/T05 read this; T06/T07 add subviews. */
export type SkillsSelection =
  | { kind: "kb"; category: string | null }
  | { kind: "tools"; toolId: string | null }
  | { kind: "targets" };

export function selectionKey(selection: SkillsSelection): string {
  switch (selection.kind) {
    case "kb":
      return selection.category ? `kb:${selection.category}` : "kb:all";
    case "tools":
      return selection.toolId ? `tools:${selection.toolId}` : "tools:all";
    case "targets":
      return "targets";
  }
}

export type NavGroup = "kb" | "tools";

interface SkillsNavProps {
  readonly selection: SkillsSelection;
  readonly onSelect: (selection: SkillsSelection) => void;
  readonly scan: SkillScan | null;
  readonly tools: SkillTool[];
  readonly hasVault: boolean;
  /** Saved collapsed state of the two groups (settings `layout`, M9 Q-UX-3). */
  readonly collapsed: { readonly kb: boolean; readonly tools: boolean };
  readonly onSetCollapsed: (group: NavGroup, collapsed: boolean) => void;
  readonly update?: SkillUpdateCheckResult | null;
  readonly onCheckUpdates?: () => void;
}

/** Left navigation of the skills page (UI 稿 11.1, 12.2): a filter box, two
 * collapsible groups (knowledge-base categories, tool directories), the
 * targets manager button on the tools group head, tools that were not detected
 * folded at the end, and a two-row footer. Below 1024px the CSS turns it into
 * a 56px icon rail. */
export function SkillsNav({
  selection,
  onSelect,
  scan,
  tools,
  hasVault,
  collapsed,
  onSetCollapsed,
  update,
  onCheckUpdates,
}: SkillsNavProps) {
  const current = selectionKey(selection);
  const counts = totals(scan);
  const progress = distributionProgress(scan);
  const attention = attentionCounts(scan);
  const attentionTotal = attention.updates + attention.broken;
  const registry = new Map(tools.map((tool) => [tool.id, tool]));
  const [filter, setFilter] = useState("");
  const [showUndetected, setShowUndetected] = useState(false);
  const needle = filter.trim().toLowerCase();
  const filtering = needle !== "";

  // Selecting something that sits in a folded group opens that group (and
  // writes it back). Only a change of selection counts: collapsing the group
  // the current item is in must stay collapsed, and so must a first render.
  const lastKey = useRef(current);
  useEffect(() => {
    if (lastKey.current === current) return;
    lastKey.current = current;
    if (selection.kind === "kb" && collapsed.kb) onSetCollapsed("kb", false);
    if (selection.kind === "tools" && collapsed.tools) onSetCollapsed("tools", false);
    // only a new selection matters; the collapsed state is read as it is then
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current]);

  const categories = hasVault ? (scan?.kb ?? []) : [];
  const toolGroups = (scan?.tools ?? []).map((group) => {
    const tool = registry.get(group.toolId);
    return { group, tool, name: tool?.name ?? group.toolId, detected: tool ? isToolDetected(tool) : true };
  });
  const matches = (label: string) => label.toLowerCase().includes(needle);
  const categoryHits = filtering ? categories.filter((group) => matches(group.category)) : categories;
  const toolHits = filtering ? toolGroups.filter((entry) => matches(entry.name)) : toolGroups;
  const shownTools = filtering ? toolHits : toolHits.filter((entry) => entry.detected);
  const undetected = filtering ? [] : toolGroups.filter((entry) => !entry.detected);
  const selectedUndetected =
    selection.kind === "tools" && undetected.some((entry) => entry.group.toolId === selection.toolId);
  const undetectedOpen = showUndetected || selectedUndetected;

  const kbExpanded = filtering || !collapsed.kb;
  const toolsExpanded = filtering || !collapsed.tools;

  const item = (
    key: string,
    target: SkillsSelection,
    icon: ReactNode,
    label: string,
    count?: number,
    dot?: boolean,
  ) => {
    const selected = key === current;
    return (
      <button
        key={key}
        type="button"
        className="skills-nav-item"
        title={label}
        aria-current={selected ? "page" : undefined}
        aria-label={dot ? `${label}，有需要处理的项` : undefined}
        onClick={() => onSelect(target)}
      >
        <span className="skills-nav-ico" aria-hidden>
          {icon}
        </span>
        <span className="skills-nav-label">
          {filtering
            ? filterParts(label, needle).map((part, index) =>
                part.hit ? <mark key={index}>{part.text}</mark> : <span key={index}>{part.text}</span>,
              )
            : label}
        </span>
        {count === undefined ? null : (
          <span className="skills-nav-count" aria-hidden>
            {count}
          </span>
        )}
        {dot ? <span className="skills-nav-dot" data-testid={`skills-nav-dot-${key.slice("tools:".length)}`} aria-hidden /> : null}
      </button>
    );
  };

  const toolItem = (entry: (typeof toolGroups)[number]) =>
    item(
      `tools:${entry.group.toolId}`,
      { kind: "tools", toolId: entry.group.toolId },
      <ToolIcon tool={entry.tool ?? { id: entry.group.toolId, name: entry.name, iconKey: "" }} size={16} />,
      entry.name,
      entry.group.skills.length,
      toolNeedsAttention(entry.group),
    );

  const groupHead = (
    group: NavGroup,
    title: string,
    shown: number,
    total: number,
    expanded: boolean,
    inside: boolean,
    extra?: ReactNode,
  ) => (
    <div className="skills-nav-grouphead" data-testid={`skills-nav-head-${group}`}>
      <button
        type="button"
        className="skills-nav-toggle"
        aria-expanded={expanded}
        aria-disabled={filtering || undefined}
        title={filtering ? "筛选时两组都展开" : undefined}
        data-testid={`skills-nav-toggle-${group}`}
        onClick={() => {
          if (filtering) return;
          onSetCollapsed(group, !collapsed[group]);
        }}
      >
        <CaretDown size={12} className="skills-nav-chev" aria-hidden />
        <span className="skills-nav-grouptitle">{title}</span>
        {!expanded && inside ? (
          <span className="skills-nav-curmark" data-testid={`skills-nav-curmark-${group}`} aria-label="当前项在这一组" />
        ) : null}
        <span className="skills-nav-groupcount">{filtering ? `${shown} / ${total}` : total}</span>
      </button>
      {extra}
    </div>
  );

  const noMatch = filtering && categoryHits.length === 0 && toolHits.length === 0;

  return (
    <aside className="skills-nav">
      <div className="skills-nav-head">
        <div className="skills-nav-eyebrow">SKILLS</div>
        <h1 className="skills-nav-title">技能</h1>
        <p className="skills-nav-sub">知识库是源，工具目录是去处</p>
      </div>

      <nav className="skills-nav-list" aria-label="技能导航" data-testid="skills-nav">
        <div className="skills-nav-filter" data-filled={filtering ? "true" : undefined}>
          <MagnifyingGlass size={13} aria-hidden />
          <input
            type="text"
            value={filter}
            placeholder="筛选分类与工具"
            aria-label="筛选分类与工具"
            spellCheck={false}
            data-testid="skills-nav-filter"
            onChange={(event) => setFilter(event.target.value)}
            onKeyDown={(event) => {
              if (event.key !== "Escape" || !filter) return;
              event.preventDefault();
              event.stopPropagation();
              setFilter("");
            }}
          />
          {filtering ? (
            <button type="button" className="skills-nav-filter-x" aria-label="清除筛选" onClick={() => setFilter("")}>
              <X size={12} aria-hidden />
            </button>
          ) : null}
        </div>

        <div className="skills-nav-section">
          {groupHead("kb", "知识库", categoryHits.length, categories.length, kbExpanded, selection.kind === "kb")}
          <div className="skills-nav-items" data-collapsed={kbExpanded ? undefined : "true"} data-testid="skills-nav-items-kb">
            {filtering ? null : item("kb:all", { kind: "kb", category: null }, <Stack size={16} />, "全部技能", counts.kb)}
            {categoryHits.map((group) =>
              item(
                `kb:${group.category}`,
                { kind: "kb", category: group.category },
                <FolderSimple size={16} />,
                group.category,
                group.skills.length,
              ),
            )}
            {filtering && categoryHits.length === 0 ? (
              <div className="skills-nav-none">{noMatch ? "没有匹配的分类或工具" : "没有匹配的分类"}</div>
            ) : null}
          </div>
        </div>

        <div className="skills-nav-section">
          {groupHead(
            "tools",
            "工具目录",
            toolHits.length,
            toolGroups.length,
            toolsExpanded,
            selection.kind === "tools",
            <button
              type="button"
              className="skills-nav-grpbtn"
              aria-label="管理目录"
              title="管理目录…"
              aria-current={current === "targets" ? "page" : undefined}
              data-testid="skills-nav-targets"
              onClick={() => onSelect({ kind: "targets" })}
            >
              <SlidersHorizontal size={14} aria-hidden />
            </button>,
          )}
          <div className="skills-nav-items" data-collapsed={toolsExpanded ? undefined : "true"} data-testid="skills-nav-items-tools">
            {filtering ? null : item("tools:all", { kind: "tools", toolId: null }, <SquaresFour size={16} />, "全部工具", counts.tools)}
            {shownTools.map(toolItem)}
            {filtering && toolHits.length === 0 && categoryHits.length > 0 ? (
              <div className="skills-nav-none">没有匹配的工具</div>
            ) : null}
            {undetected.length > 0 ? (
              <>
                <button
                  type="button"
                  className="skills-nav-more"
                  aria-expanded={undetectedOpen}
                  data-testid="skills-nav-undetected"
                  onClick={() => setShowUndetected(!showUndetected)}
                >
                  <CaretDown size={12} className="skills-nav-chev" aria-hidden />
                  <span>未检测到的工具（{undetected.length}）</span>
                  {undetected.some((entry) => toolNeedsAttention(entry.group)) ? (
                    <span className="skills-nav-curmark" data-testid="skills-nav-undetected-dot" aria-label="有需要处理的项" />
                  ) : null}
                </button>
                {undetectedOpen ? undetected.map(toolItem) : null}
              </>
            ) : null}
          </div>
        </div>
      </nav>

      <div className="skills-nav-foot">
        {hasVault ? (
          <>
            <div className="skills-nav-row1">
              <span>已分发</span>
              <div className="skills-nav-meter" aria-hidden>
                <i style={{ width: `${progress.total > 0 ? Math.round((progress.distributed / progress.total) * 100) : 0}%` }} />
              </div>
              <b>
                {progress.distributed} / {progress.total}
              </b>
            </div>
            <div className="skills-nav-row2">
              <span
                className={attentionTotal > 0 ? "skills-nav-warn" : undefined}
                title={`${attention.updates} 个有更新 · ${attention.broken} 个链接失效`}
                data-testid="skills-attention"
              >
                {attentionTotal > 0 ? `${attentionTotal} 项需要处理` : "没有需要处理的项"}
              </span>
              {onCheckUpdates ? (
                <>
                  <span aria-hidden>·</span>
                  <span data-testid="skills-checked">
                    {update?.checking ? "检查中…" : checkedLabel(update?.checkedAt) || "尚未检查"}
                  </span>
                  <button
                    type="button"
                    className="skills-nav-check"
                    disabled={update?.checking ?? false}
                    onClick={onCheckUpdates}
                    data-testid="skills-check-updates"
                  >
                    检查更新
                  </button>
                </>
              ) : null}
            </div>
            {update && update.unchecked > 0 ? (
              <div className="skills-nav-attention" data-testid="skills-unchecked">
                {update.unchecked} 个来源这次没查
              </div>
            ) : null}
            {update?.rateLimitResetAt ? (
              <div className="skills-nav-attention" data-testid="skills-rate-limit">
                额度用完，{clockOf(update.rateLimitResetAt)} 后再试
              </div>
            ) : null}
          </>
        ) : (
          <div className="skills-nav-attention">打开知识库后显示分发进度</div>
        )}
        {scan?.scanning || !scan ? (
          <div className="skills-nav-attention" data-testid="skills-status">
            {scan ? "后台扫描中…" : "尚未扫描"}
          </div>
        ) : null}
      </div>
    </aside>
  );
}
