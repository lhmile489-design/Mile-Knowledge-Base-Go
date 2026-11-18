import { useEffect, useRef } from "react";
import { MagnifyingGlass, X } from "@phosphor-icons/react";
import { useSearchStore, type SortMode } from "../stores/searchStore";
import { useDocStore } from "../stores/docStore";
import { useAppStore } from "../stores/appStore";
import { describeMatchedFields, highlightRanges, tokenize, type HighlightRange } from "../lib/search";

const AREA_LABELS: Record<string, string> = {
  skills: "技能",
  raws: "暂存",
  projects: "项目",
  tools: "工具",
  wiki: "独立库",
  constraints: "约束",
  archives: "归档",
  other: "其他",
  system: "系统",
};

const SORT_LABELS: Record<SortMode, string> = {
  relevance: "相关度",
  updated: "更新时间",
  path: "路径",
};

const TIME_RANGES: { label: string; days: number | null }[] = [
  { label: "全部", days: null },
  { label: "1 天", days: 1 },
  { label: "7 天", days: 7 },
  { label: "30 天", days: 30 },
];

const QUICK_EXTS = ["md", "txt", "ts"];

function formatTime(ms: number): string {
  if (!ms) return "";
  const date = new Date(ms);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** Highlights ranges from the shared tokenizer; plain text stays as-is. */
function Excerpt({ text, ranges }: { text: string; ranges: readonly HighlightRange[] }) {
  if (ranges.length === 0) return <>{text}</>;
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  ranges.forEach((range, index) => {
    if (range.start > cursor) parts.push(text.slice(cursor, range.start));
    parts.push(
      <mark key={index} className="search-mark">
        {text.slice(range.start, range.end)}
      </mark>,
    );
    cursor = range.end;
  });
  if (cursor < text.length) parts.push(text.slice(cursor));
  return <>{parts}</>;
}

/** Ctrl+P search panel at the top of the document column (PRD FR-SRCH). */
export function SearchPanel() {
  const open = useSearchStore((state) => state.open);
  const text = useSearchStore((state) => state.text);
  const hits = useSearchStore((state) => state.hits);
  const total = useSearchStore((state) => state.total);
  const partial = useSearchStore((state) => state.partial);
  const tookMs = useSearchStore((state) => state.tookMs);
  const searching = useSearchStore((state) => state.searching);
  const error = useSearchStore((state) => state.error);
  const selectedIndex = useSearchStore((state) => state.selectedIndex);
  const areas = useSearchStore((state) => state.areas);
  const extensions = useSearchStore((state) => state.extensions);
  const updatedAfter = useSearchStore((state) => state.updatedAfter);
  const sort = useSearchStore((state) => state.sort);

  const setText = useSearchStore((state) => state.setText);
  const closePanel = useSearchStore((state) => state.closePanel);
  const toggleArea = useSearchStore((state) => state.toggleArea);
  const toggleExtension = useSearchStore((state) => state.toggleExtension);
  const setUpdatedAfter = useSearchStore((state) => state.setUpdatedAfter);
  const clearFilters = useSearchStore((state) => state.clearFilters);
  const setSort = useSearchStore((state) => state.setSort);
  const moveSelection = useSearchStore((state) => state.moveSelection);

  const vaultId = useAppStore((state) => state.activeVaultId);
  const openDoc = useDocStore((state) => state.open);

  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  if (!open) return null;

  const queryTokens = text.trim() === "" ? [] : tokenize(text);
  const filterActive = areas.length > 0 || extensions.length > 0 || updatedAfter !== null;

  const openHit = (path: string) => {
    if (!vaultId) return;
    void openDoc(vaultId, path);
    closePanel();
  };

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "Escape") {
      event.preventDefault();
      closePanel();
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      moveSelection(1);
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      moveSelection(-1);
    } else if (event.key === "Enter") {
      event.preventDefault();
      const hit = hits[selectedIndex];
      if (hit) openHit(hit.path);
    }
  };

  return (
    <section className="search-panel" aria-label="搜索" onKeyDown={onKeyDown}>
      <div className="search-title">
        <span className="eyebrow">搜索</span>
        <span className="search-title-count">SEARCH {total}</span>
        <button type="button" className="icon-btn search-close" aria-label="关闭搜索" onClick={closePanel}>
          <X size={14} aria-hidden />
        </button>
      </div>

      <div className="search-head">
        <MagnifyingGlass size={14} aria-hidden className="search-head-icon" />
        <input
          ref={inputRef}
          className="search-input"
          placeholder="搜索知识库（标题、正文、路径、标签）"
          value={text}
          onChange={(event) => setText(event.target.value)}
          aria-label="搜索关键词"
        />
        {text ? (
          <button type="button" className="icon-btn" aria-label="清空输入" onClick={() => setText("")}>
            <X size={12} aria-hidden />
          </button>
        ) : null}
        <div className="segmented" role="tablist" aria-label="排序">
          {(Object.keys(SORT_LABELS) as SortMode[]).map((mode) => (
            <button
              key={mode}
              type="button"
              role="tab"
              aria-selected={sort === mode}
              className="segmented-item"
              onClick={() => setSort(mode)}
            >
              {SORT_LABELS[mode]}
            </button>
          ))}
        </div>
      </div>

      <div className="search-filters">
        <div className="search-chips" role="group" aria-label="功能区筛选">
          {Object.entries(AREA_LABELS).map(([area, label]) => (
            <button
              key={area}
              type="button"
              className="search-chip"
              aria-pressed={areas.includes(area)}
              onClick={() => toggleArea(area)}
            >
              {label}
            </button>
          ))}
        </div>

        <span className="search-sep" aria-hidden />

        <div className="search-chips" role="group" aria-label="时间范围">
          {TIME_RANGES.map((range) => {
            const value = range.days === null ? null : Date.now() - range.days * 86_400_000;
            const active = range.days === null ? updatedAfter === null : updatedAfter !== null && updatedAfter >= value!;
            return (
              <button
                key={range.label}
                type="button"
                className="search-chip"
                aria-pressed={active}
                onClick={() => setUpdatedAfter(range.days === null ? null : Date.now() - range.days * 86_400_000)}
              >
                {range.label}
              </button>
            );
          })}
        </div>

        <span className="search-sep" aria-hidden />

        <div className="search-chips" role="group" aria-label="扩展名">
          {QUICK_EXTS.map((ext) => (
            <button
              key={ext}
              type="button"
              className="search-chip"
              aria-pressed={extensions.includes(ext)}
              onClick={() => toggleExtension(ext)}
            >
              {ext}
            </button>
          ))}
        </div>

        {filterActive ? (
          <button type="button" className="btn-quiet search-clear" onClick={clearFilters}>
            清除过滤
          </button>
        ) : null}
      </div>

      <div className="search-body" role="listbox" aria-label="搜索结果">
        {error ? (
          <p className="search-status" role="alert">
            {error}
          </p>
        ) : text.trim() === "" ? (
          <p className="search-status muted">输入关键词开始搜索。支持中文；空格分隔多个词。</p>
        ) : searching && hits.length === 0 ? (
          <p className="search-status muted">正在检索…</p>
        ) : hits.length === 0 ? (
          <p className="search-status muted">没有匹配的文档。</p>
        ) : (
          hits.map((hit, index) => (
            <button
              key={hit.path}
              type="button"
              role="option"
              aria-selected={index === selectedIndex}
              className="search-hit"
              data-active={index === selectedIndex || undefined}
              onClick={() => openHit(hit.path)}
            >
              <span className="search-hit-head">
                <span className="search-hit-title">{hit.title}</span>
                <span className="search-hit-time">{formatTime(hit.mtimeMs)}</span>
              </span>
              <span className="search-hit-path">{hit.path}</span>
              {hit.excerpt ? (
                <span className="search-hit-excerpt">
                  <Excerpt text={hit.excerpt} ranges={highlightRanges(hit.excerpt, queryTokens)} />
                </span>
              ) : null}
              <span className="search-hit-match">
                <span className="search-hit-match-label">匹配</span>
                {describeMatchedFields(hit.matchedFields as never[])}
                {hit.tags.length ? ` · ${hit.tags.join("、")}` : ""}
                {hit.ignored ? " · 系统区" : ""}
              </span>
            </button>
          ))
        )}
      </div>

      <div className="search-foot">
        <span>
          {total} 条结果 · {tookMs} ms · ↑↓ 选择 · Enter 打开 · Esc 关闭
        </span>
        {partial ? <span className="search-partial">索引未完成</span> : null}
      </div>
    </section>
  );
}
