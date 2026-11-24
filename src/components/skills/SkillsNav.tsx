import type { ReactNode } from "react";
import { FolderSimple, SlidersHorizontal, SquaresFour, Stack } from "@phosphor-icons/react";
import type { SkillScan, SkillTool } from "../../lib/transport";
import { attentionCounts, distributionProgress, toolNeedsAttention, totals } from "../../lib/skillsView";
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

interface SkillsNavProps {
  readonly selection: SkillsSelection;
  readonly onSelect: (selection: SkillsSelection) => void;
  readonly scan: SkillScan | null;
  readonly tools: SkillTool[];
  readonly hasVault: boolean;
}

/** Left navigation of the skills page (UI 稿 11.1): knowledge-base categories,
 * tool directories, the targets manager entry and the distribution footer.
 * Below 1024px the CSS turns it into a 56px icon rail. */
export function SkillsNav({ selection, onSelect, scan, tools, hasVault }: SkillsNavProps) {
  const current = selectionKey(selection);
  const counts = totals(scan);
  const progress = distributionProgress(scan);
  const attention = attentionCounts(scan);
  const registry = new Map(tools.map((tool) => [tool.id, tool]));

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
        <span className="skills-nav-label">{label}</span>
        {count === undefined ? null : (
          <span className="skills-nav-count" aria-hidden>
            {count}
          </span>
        )}
        {dot ? <span className="skills-nav-dot" data-testid={`skills-nav-dot-${key.slice("tools:".length)}`} aria-hidden /> : null}
      </button>
    );
  };

  return (
    <aside className="skills-nav">
      <div className="skills-nav-head">
        <div className="skills-nav-eyebrow">SKILLS</div>
        <h1 className="skills-nav-title">技能</h1>
        <p className="skills-nav-sub">知识库是源，工具目录是去处</p>
      </div>

      <nav className="skills-nav-list" aria-label="技能导航">
        <div className="skills-nav-group">知识库</div>
        {item("kb:all", { kind: "kb", category: null }, <Stack size={16} />, "全部技能", counts.kb)}
        {hasVault
          ? (scan?.kb ?? []).map((group) =>
              item(
                `kb:${group.category}`,
                { kind: "kb", category: group.category },
                <FolderSimple size={16} />,
                group.category,
                group.skills.length,
              ),
            )
          : null}

        <div className="skills-nav-sep" aria-hidden />

        <div className="skills-nav-group">工具目录</div>
        {item("tools:all", { kind: "tools", toolId: null }, <SquaresFour size={16} />, "全部工具", counts.tools)}
        {(scan?.tools ?? []).map((group) => {
          const tool = registry.get(group.toolId);
          const name = tool?.name ?? group.toolId;
          return item(
            `tools:${group.toolId}`,
            { kind: "tools", toolId: group.toolId },
            <ToolIcon tool={tool ?? { id: group.toolId, name, iconKey: "" }} size={16} />,
            name,
            group.skills.length,
            toolNeedsAttention(group),
          );
        })}
        {item("targets", { kind: "targets" }, <SlidersHorizontal size={16} />, "管理目录…")}
      </nav>

      <div className="skills-nav-foot">
        {hasVault ? (
          <>
            <div className="skills-nav-progress">
              <span>已分发</span>
              <b>
                {progress.distributed} / {progress.total}
              </b>
            </div>
            <div className="skills-nav-meter" aria-hidden>
              <i style={{ width: `${progress.total > 0 ? Math.round((progress.distributed / progress.total) * 100) : 0}%` }} />
            </div>
            <div className="skills-nav-attention">
              {attention.updates} 个有更新 · {attention.broken} 个链接失效
            </div>
          </>
        ) : (
          <div className="skills-nav-attention">打开知识库后显示分发进度</div>
        )}
      </div>
    </aside>
  );
}
