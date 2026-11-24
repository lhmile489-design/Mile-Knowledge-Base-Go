import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Check, DotsThree, FolderSimple, GithubLogo, ListBullets, Plus, SquaresFour } from "@phosphor-icons/react";
import type { KbSkill, SkillCategory, SkillTool } from "../../lib/transport";
import { remotePill, sourceOf } from "../../lib/skillRemote";
import { ToolIcon } from "./ToolIcon";

type View = "grid" | "list";

/** 知识库技能视图 (M4 T10; M8 T04 page layout, UI 稿 11.2). Grouped cards with
 * the category glyph, two-line description, distributed tool stack, status
 * pills, hover actions, batch selection with a bottom dock and the dashed
 * install card. Data flow and callbacks are unchanged from the panel. */
export function KbSkillsPage({
  groups,
  tools,
  onOpenSkill,
  onLocate,
  onDistribute,
  onInstall,
  onUpdate,
}: {
  readonly groups: SkillCategory[];
  readonly tools: SkillTool[];
  readonly onOpenSkill: (path: string) => void;
  readonly onLocate: (path: string) => void;
  readonly onDistribute: (paths: string[]) => void;
  readonly onInstall: () => void;
  readonly onUpdate: (skill: KbSkill) => void;
}) {
  const [query, setQuery] = useState("");
  const [view, setView] = useState<View>("grid");
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const searchRef = useRef<HTMLInputElement | null>(null);

  const total = useMemo(() => groups.reduce((sum, group) => sum + group.skills.length, 0), [groups]);
  const title = groups.length === 1 ? (groups[0]?.category ?? "全部技能") : "全部技能";

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return groups;
    return groups
      .map((group) => ({
        ...group,
        skills: group.skills.filter(
          (skill) =>
            skill.name.toLowerCase().includes(needle) ||
            skill.description.toLowerCase().includes(needle) ||
            group.category.toLowerCase().includes(needle),
        ),
      }))
      .filter((group) => group.skills.length > 0);
  }, [groups, query]);

  const filteredTotal = useMemo(
    () => filtered.reduce((sum, group) => sum + group.skills.length, 0),
    [filtered],
  );

  // Ctrl/Cmd+K focuses the search field from anywhere on the page.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "k" || !(event.ctrlKey || event.metaKey)) return;
      event.preventDefault();
      searchRef.current?.focus();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  const toggleSelect = (path: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  const selectGroup = (group: SkillCategory) => {
    setSelected((current) => {
      const next = new Set(current);
      for (const skill of group.skills) next.add(skill.path);
      return next;
    });
  };

  const startSelecting = () => {
    setSelected(new Set());
    setSelecting(true);
  };

  const stopSelecting = () => {
    setSelecting(false);
    setSelected(new Set());
  };

  return (
    <div className="kb-skills" data-selecting={selecting ? "true" : "false"}>
      <div className="kb-toolbar">
        <h2 className="kb-title">{title}</h2>
        <span className="kb-count" data-testid="kb-count">
          {query ? `${filteredTotal} / ${total} 个` : `${total} 个 · ${groups.length} 个分类`}
        </span>
        <span className="kb-spacer" />
        <input
          ref={searchRef}
          className="kb-search"
          type="search"
          value={query}
          placeholder="搜索名称或描述"
          aria-label="搜索技能"
          data-testid="kb-skills-search"
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="segmented kb-view-toggle" role="tablist" aria-label="显示方式">
          <button
            type="button"
            role="tab"
            aria-selected={view === "grid"}
            className="segmented-item"
            aria-label="网格"
            onClick={() => setView("grid")}
          >
            <SquaresFour size={14} aria-hidden />
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={view === "list"}
            className="segmented-item"
            aria-label="列表"
            onClick={() => setView("list")}
          >
            <ListBullets size={14} aria-hidden />
          </button>
        </div>
        <button type="button" className="btn-primary kb-install" data-testid="kb-install-top" onClick={onInstall}>
          <Plus size={13} aria-hidden />
          安装
        </button>
      </div>

      <div className="kb-content">
        {filtered.length === 0 ? (
          <div className="skills-empty" data-testid="kb-empty">
            <p>{query ? "没有匹配的技能。" : "知识库里还没有技能。"}</p>
            {query ? (
              <button type="button" className="btn-quiet" onClick={() => setQuery("")}>
                清除搜索
              </button>
            ) : (
              <button type="button" className="btn-quiet" onClick={onInstall}>
                安装技能…
              </button>
            )}
          </div>
        ) : (
          <div className="kb-groups">
            {filtered.map((group, index) => {
              const last = index === filtered.length - 1;
              return (
                <section key={group.category} data-testid={`kb-group-${group.category}`}>
                  <div className="kb-group-head">
                    <span className="kb-group-title">{group.category}</span>
                    <span className="kb-group-count">{group.skills.length}</span>
                    {selecting ? (
                      <button
                        type="button"
                        className="kb-group-all"
                        data-testid={`kb-select-all-${group.category}`}
                        onClick={() => selectGroup(group)}
                      >
                        全选本组
                      </button>
                    ) : null}
                  </div>
                  <div className={view === "grid" ? "kb-grid" : "kb-list"} data-testid={`kb-${view}-${group.category}`}>
                    {group.skills.map((skill, cardIndex) => (
                      <KbSkillCard
                        key={skill.path}
                        index={cardIndex}
                        skill={skill}
                        tools={tools}
                        selecting={selecting}
                        selected={selected.has(skill.path)}
                        onToggle={() => toggleSelect(skill.path)}
                        onStartSelecting={startSelecting}
                        onOpen={() => onOpenSkill(skill.path)}
                        onLocate={() => onLocate(skill.path)}
                        onDistribute={() => onDistribute([skill.path])}
                        onUpdate={() => onUpdate(skill)}
                      />
                    ))}
                    {last && !selecting ? (
                      <button type="button" className="kb-add-card" data-testid="kb-add-card" onClick={onInstall}>
                        <span className="kb-card-glyph" aria-hidden>
                          <Plus size={16} />
                        </span>
                        <b>安装新技能</b>
                        <span>GitHub 地址、zip 包或本地文件夹</span>
                      </button>
                    ) : null}
                  </div>
                </section>
              );
            })}
          </div>
        )}
      </div>

      {selecting ? (
        <div className="kb-batchbar" role="status" data-testid="kb-batchbar">
          <span>已选 {selected.size} 个</span>
          <span className="kb-spacer" />
          <button type="button" className="btn-quiet" onClick={stopSelecting} data-testid="kb-batch-cancel">
            取消
          </button>
          <button
            type="button"
            className="btn-primary"
            disabled={selected.size === 0}
            onClick={() => onDistribute([...selected])}
            data-testid="kb-batch-distribute"
          >
            分发到…
          </button>
        </div>
      ) : null}
    </div>
  );
}

function statusOf(skill: KbSkill): { label: string; tone: "live" | "warn" | "idle" } {
  if (!skill.distributed || skill.distributed.length === 0) return { label: "未分发", tone: "idle" };
  if (skill.distributed.some((item) => item.sync === "vault-updated")) {
    return { label: "知识库有更新", tone: "warn" };
  }
  return { label: `已分发 ${skill.distributed.length} 处`, tone: "live" };
}

function KbSkillCard({
  index,
  skill,
  tools,
  selecting,
  selected,
  onToggle,
  onStartSelecting,
  onOpen,
  onLocate,
  onDistribute,
  onUpdate,
}: {
  readonly index: number;
  readonly skill: KbSkill;
  readonly tools: SkillTool[];
  readonly selecting: boolean;
  readonly selected: boolean;
  readonly onToggle: () => void;
  readonly onStartSelecting: () => void;
  readonly onOpen: () => void;
  readonly onLocate: () => void;
  readonly onDistribute: () => void;
  readonly onUpdate: () => void;
}) {
  const [menu, setMenu] = useState(false);
  const status = statusOf(skill);
  const remote = remotePill(skill.remote);
  const updatable = skill.remote?.updateState === "update-available";
  const distributed = (skill.distributed ?? []).slice(0, 4);
  const extra = Math.max(0, (skill.distributed?.length ?? 0) - distributed.length);

  return (
    <article
      className="kb-card"
      data-selected={selected ? "true" : "false"}
      data-testid={`kb-card-${skill.name}`}
      style={{ "--i": Math.min(index, 24) } as CSSProperties}
      onPointerMove={(event) => {
        const rect = event.currentTarget.getBoundingClientRect();
        event.currentTarget.style.setProperty("--mx", `${event.clientX - rect.left}px`);
        event.currentTarget.style.setProperty("--my", `${event.clientY - rect.top}px`);
      }}
    >
      {selecting ? (
        <button
          type="button"
          className="kb-check"
          role="checkbox"
          aria-checked={selected}
          aria-label={`选择 ${skill.name}`}
          onClick={onToggle}
        >
          {selected ? <Check size={12} weight="bold" aria-hidden /> : null}
        </button>
      ) : (
        <span className="kb-menu-wrap">
          <button
            type="button"
            className="kb-kebab"
            aria-label={`更多操作：${skill.name}`}
            aria-haspopup="menu"
            aria-expanded={menu}
            onClick={() => setMenu(!menu)}
          >
            <DotsThree size={16} aria-hidden />
          </button>
          {menu ? (
            <span className="popover kb-menu" role="menu">
              <button type="button" role="menuitem" onClick={() => { setMenu(false); onOpen(); }}>
                打开 SKILL.md
              </button>
              <button type="button" role="menuitem" onClick={() => { setMenu(false); onLocate(); }}>
                在侧栏定位
              </button>
              <button
                type="button"
                role="menuitem"
                data-testid="kb-batch-toggle"
                onClick={() => { setMenu(false); onStartSelecting(); }}
              >
                批量选择
              </button>
              {updatable ? (
                <button type="button" role="menuitem" onClick={() => { setMenu(false); onUpdate(); }}>
                  更新…
                </button>
              ) : null}
              {skill.remote?.updateState === "both-changed" ? (
                <button type="button" role="menuitem" disabled title="本地改过，不能一键更新">
                  更新（本地改过）
                </button>
              ) : null}
            </span>
          ) : null}
        </span>
      )}

      <div className="kb-card-top">
        <span className="kb-card-glyph" aria-hidden>
          <FolderSimple size={16} />
        </span>
        <span className="kb-card-heading">
          <span className="kb-card-name">{skill.name}</span>
          <span className="kb-card-cat">{skill.category}</span>
        </span>
      </div>

      <span className="kb-card-desc">{skill.description || "—"}</span>

      <span className="kb-card-meta">
        {distributed.length > 0 ? (
          <span className="kb-card-stack">
            {distributed.map((item) => (
              <ToolIcon
                key={item.toolId}
                tool={tools.find((tool) => tool.id === item.toolId) ?? { id: item.toolId, name: item.toolId, iconKey: "" }}
                size={16}
              />
            ))}
            {extra > 0 ? <span className="kb-card-more">+{extra}</span> : null}
          </span>
        ) : (
          <span className="kb-card-none">未分发</span>
        )}
        <span className="kb-card-tools">
          {skill.remote ? (
            <span className="kb-card-src" title={sourceOf(skill.remote)}>
              <GithubLogo size={12} aria-hidden />
              {sourceOf(skill.remote)}
            </span>
          ) : null}
        </span>
      </span>

      <span className="kb-card-foot">
        <span className="pill" data-tone={status.tone}>
          {status.label}
        </span>
        {remote ? (
          <span className="pill" data-tone={remote.tone} title={remote.title}>
            {remote.text}
          </span>
        ) : null}
        <span className="kb-spacer" />
        {!selecting ? (
          updatable ? (
            <button
              type="button"
              className="kb-act"
              onClick={onUpdate}
              data-testid={`kb-update-${skill.name}`}
            >
              更新
            </button>
          ) : (
            <button type="button" className="kb-act" onClick={onDistribute}>
              分发
            </button>
          )
        ) : null}
      </span>
    </article>
  );
}
