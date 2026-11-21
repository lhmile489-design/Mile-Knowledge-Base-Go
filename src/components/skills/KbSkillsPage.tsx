import { useMemo, useState } from "react";
import { DotsThree, SquaresFour, ListBullets, Check, GithubLogo } from "@phosphor-icons/react";
import type { KbSkill, SkillCategory, SkillTool } from "../../lib/transport";
import { remotePill, sourceOf } from "../../lib/skillRemote";
import { ToolIcon } from "./ToolIcon";

type View = "grid" | "list";

/** 知识库技能页 (M4 T10; UI 稿 9.2 kb-*): grouped cards, search, grid/list,
 * batch selection and the distribute entry. */
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

  const toggleSelect = (path: string) => {
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });
  };

  return (
    <div className="kb-skills">
      <div className="kb-toolbar">
        <input
          className="kb-search"
          type="search"
          value={query}
          placeholder="搜索技能"
          aria-label="搜索技能"
          data-testid="kb-skills-search"
          onChange={(event) => setQuery(event.target.value)}
        />
        <div className="segmented" role="tablist" aria-label="显示方式">
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
        <button
          type="button"
          className="btn-quiet"
          data-testid="kb-batch-toggle"
          aria-pressed={selecting}
          onClick={() => {
            setSelecting(!selecting);
            setSelected(new Set());
          }}
        >
          {selecting ? "取消选择" : "批量选择"}
        </button>
      </div>

      {filtered.length === 0 ? (
        <div className="skills-empty">
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
          {filtered.map((group) => (
            <section key={group.category} data-testid={`kb-group-${group.category}`}>
              <h3 className="kb-group-title">{group.category}</h3>
              <div className={view === "grid" ? "kb-grid" : "kb-list"} data-testid={`kb-${view}-${group.category}`}>
                {group.skills.map((skill) => (
                  <KbSkillCard
                    key={skill.path}
                    skill={skill}
                    tools={tools}
                    selecting={selecting}
                    selected={selected.has(skill.path)}
                    onToggle={() => toggleSelect(skill.path)}
                    onOpen={() => onOpenSkill(skill.path)}
                    onLocate={() => onLocate(skill.path)}
                    onDistribute={() => onDistribute([skill.path])}
                    onUpdate={() => onUpdate(skill)}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      )}

      {selecting ? (
        <div className="kb-batchbar" role="status">
          <span>已选 {selected.size} 个</span>
          <button type="button" className="btn-quiet" onClick={() => setSelected(new Set())} disabled={selected.size === 0}>
            清空
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
  skill,
  tools,
  selecting,
  selected,
  onToggle,
  onOpen,
  onLocate,
  onDistribute,
  onUpdate,
}: {
  readonly skill: KbSkill;
  readonly tools: SkillTool[];
  readonly selecting: boolean;
  readonly selected: boolean;
  readonly onToggle: () => void;
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
      ) : null}
      <span className="kb-card-name">{skill.name}</span>
      <span className="kb-card-desc">{skill.description || "—"}</span>
      <span className="kb-card-tools">
        {distributed.map((item) => (
          <ToolIcon key={item.toolId} tool={tools.find((tool) => tool.id === item.toolId) ?? { id: item.toolId, name: item.toolId, iconKey: "" }} size={16} />
        ))}
        {extra > 0 ? <span className="kb-card-more">+{extra}</span> : null}
        {skill.remote ? (
          <span className="kb-card-src" title={sourceOf(skill.remote)}>
            <GithubLogo size={12} aria-hidden />
            {sourceOf(skill.remote)}
          </span>
        ) : null}
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
        <span className="kb-card-actions">
        {!selecting ? (
          <>
            {updatable ? (
              <button
                type="button"
                className="btn-quiet kb-distribute"
                onClick={onUpdate}
                data-testid={`kb-update-${skill.name}`}
              >
                更新
              </button>
            ) : (
              <button type="button" className="btn-quiet kb-distribute" onClick={onDistribute}>
                分发
              </button>
            )}
            <span className="kb-menu-wrap">
              <button
                type="button"
                className="icon-button"
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
          </>
        ) : null}
        </span>
      </span>
    </article>
  );
}
