import { useMemo, useState } from "react";
import { Archive, ArrowClockwise, FolderOpen, Tray } from "@phosphor-icons/react";
import { skillsApi, type SkillTool, type SkillToolScan, type ToolSkill } from "../../lib/transport";
import { ToolIcon } from "./ToolIcon";
import { UninstallDialog } from "./UninstallDialog";
import { AdoptDialog } from "./AdoptDialog";

const ORIGIN_LABEL: Record<string, { text: string; tone: "live" | "warn" | "fault" | "idle" }> = {
  linked: { text: "链接到知识库", tone: "live" },
  "link-broken": { text: "链接失效", tone: "fault" },
  copied: { text: "拷贝自知识库", tone: "live" },
  independent: { text: "独立", tone: "idle" },
  builtin: { text: "内置", tone: "idle" },
  "external-link": { text: "链接到外部", tone: "idle" },
};

const SYNC_LABEL: Record<string, { text: string; tone: "live" | "warn" | "idle" }> = {
  "in-sync": { text: "一致", tone: "live" },
  "vault-updated": { text: "知识库有更新", tone: "warn" },
  "tool-updated": { text: "工具里有改动", tone: "warn" },
  "both-updated": { text: "两边都改了", tone: "warn" },
  missing: { text: "工具里不见了", tone: "idle" },
};

function statusOf(skill: ToolSkill) {
  if (skill.origin === "copied" && skill.sync) return SYNC_LABEL[skill.sync] ?? ORIGIN_LABEL.copied ?? { text: "拷贝", tone: "idle" as const };
  return ORIGIN_LABEL[skill.origin] ?? { text: skill.origin, tone: "idle" as const };
}

/** 工具技能页 (M4 T11; UI 稿 tools-*): filter chips, rows with source status,
 * row actions, uninstall and adopt confirmations. */
export function ToolSkillsPage({
  vaultId,
  groups,
  tools,
  onChanged,
  onResult,
}: {
  readonly vaultId: string;
  readonly groups: SkillToolScan[];
  readonly tools: SkillTool[];
  readonly onChanged: () => void;
  readonly onResult?: (lines: string[]) => void;
}) {
  const [chip, setChip] = useState("all");
  const [showHidden, setShowHidden] = useState(false);
  const [uninstall, setUninstall] = useState<ToolSkill | null>(null);
  const [adopt, setAdopt] = useState<ToolSkill | null>(null);

  const countOf = (id: string) => groups.find((group) => group.toolId === id)?.skills.length ?? 0;
  const visible = (tool: SkillTool) => {
    if (tool.id === "agents") return true;
    if (showHidden) return true;
    return tool.enabled && (tool.status === "detected" || countOf(tool.id) > 0);
  };
  const chips = useMemo(() => tools.filter(visible), [tools, groups, showHidden]);
  const hiddenCount = tools.filter(
    (tool) => tool.id !== "agents" && !(tool.enabled && (tool.status === "detected" || countOf(tool.id) > 0)),
  ).length;
  const detectedCount = tools.filter((tool) => tool.id !== "agents" && tool.status === "detected").length;

  const rows = useMemo(() => {
    const all = groups.flatMap((group) => group.skills.map((skill) => ({ skill, toolId: group.toolId })));
    return all
      .filter((row) => chip === "all" || row.toolId === chip)
      .sort((a, b) => a.skill.name.toLowerCase().localeCompare(b.skill.name.toLowerCase()));
  }, [groups, chip]);

  const toolFor = (id: string) => tools.find((tool) => tool.id === id);
  const sameName = (skill: ToolSkill) =>
    rows
      .filter((row) => row.skill.name === skill.name && row.skill.path !== skill.path)
      .map((row) => ({ toolId: row.toolId, path: row.skill.path }));

  const reDistribute = async (skill: ToolSkill) => {
    if (!vaultId) return;
    // A copy that fell behind is refreshed through the normal distribute path
    // (overwrite keeps a backup first).
    try {
      const value = await skillsApi.distribute(
        vaultId,
        skill.sourcePath ?? "",
        [{ targetId: skill.toolId, allowOverwrite: true }],
        "copy",
      );
      onResult?.(value.items.map((item) => `${skill.name}：${item.result}${item.detail ? `（${item.detail}）` : ""}`));
      onChanged();
    } catch (error) {
      onResult?.([`重新分发失败：${error instanceof Error ? error.message : String(error)}`]);
    }
  };

  return (
    <div className="tool-skills">
      <div className="tool-chips" role="tablist" aria-label="工具筛选">
        <button
          type="button"
          role="tab"
          aria-selected={chip === "all"}
          className="skm-chip"
          data-testid="tool-chip-all"
          onClick={() => setChip("all")}
        >
          全部
        </button>
        {chips.map((tool) => {
          const count = countOf(tool.id);
          return (
            <button
              key={tool.id}
              type="button"
              role="tab"
              aria-selected={chip === tool.id}
              className="skm-chip"
              data-testid={`tool-chip-${tool.id}`}
              onClick={() => setChip(tool.id)}
            >
              <ToolIcon tool={tool} size={16} />
              {tool.name}
              <span className="tool-chip-count">{count}</span>
            </button>
          );
        })}
      </div>

      {hiddenCount > 0 ? (
        <button
          type="button"
          className="btn-quiet"
          data-testid="tool-toggle-hidden"
          onClick={() => setShowHidden(!showHidden)}
        >
          {showHidden ? "收起未检测到的工具" : `显示未检测到的 ${hiddenCount} 个`}
        </button>
      ) : null}

      {rows.length === 0 ? (
        <div className="skills-empty">
          <p>{detectedCount === 0 ? "没有检测到已安装的工具。" : "没有可显示的工具技能。"}</p>
          {detectedCount === 0 ? (
            <p className="field-hint">探测依据：CLI、安装记录、安装目录。装好工具后重开浮层即可看到。</p>
          ) : null}
        </div>
      ) : (
        <div className="tool-rows">
          {rows.map(({ skill, toolId }) => {
            const status = statusOf(skill);
            const tool = toolFor(toolId);
            return (
              <div key={skill.path} className="tool-row" data-testid={`tool-skill-${skill.name}-${toolId}`}>
                <ToolIcon tool={tool} size={18} />
                <span className="tool-row-name">{skill.name}</span>
                <span className="tool-row-path" title={skill.path}>
                  {skill.path}
                </span>
                <span className="pill" data-tone={status.tone}>
                  {status.text}
                </span>
                <span className="tool-row-actions">
                  {skill.origin === "copied" && skill.sync === "vault-updated" && skill.sourcePath ? (
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={`重新分发 ${skill.name}`}
                      onClick={() => void reDistribute(skill)}
                    >
                      <ArrowClockwise size={14} aria-hidden />
                    </button>
                  ) : null}
                  {skill.origin !== "builtin" ? (
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={`卸载 ${skill.name}`}
                      onClick={() => setUninstall(skill)}
                      data-testid={`tool-uninstall-${skill.name}`}
                    >
                      <Tray size={14} aria-hidden />
                    </button>
                  ) : null}
                  {skill.origin === "independent" || skill.origin === "builtin" || skill.origin === "external-link" ? (
                    <button
                      type="button"
                      className="icon-button"
                      aria-label={`收进知识库 ${skill.name}`}
                      onClick={() => setAdopt(skill)}
                      data-testid={`tool-adopt-${skill.name}`}
                    >
                      <Archive size={14} aria-hidden />
                    </button>
                  ) : null}
                  <button
                    type="button"
                    className="icon-button"
                    aria-label={`打开所在目录 ${skill.name}`}
                    onClick={() => void skillsApi.reveal(toolId, skill.name).catch(() => undefined)}
                  >
                    <FolderOpen size={14} aria-hidden />
                  </button>
                </span>
              </div>
            );
          })}
        </div>
      )}

      {uninstall ? (
        <UninstallDialog
          toolId={uninstall.toolId}
          path={uninstall.name}
          skillName={uninstall.name}
          onClose={() => setUninstall(null)}
          onDone={() => {
            setUninstall(null);
            onChanged();
          }}
          onResult={onResult}
        />
      ) : null}

      {adopt ? (
        <AdoptDialog
          vaultId={vaultId}
          skill={adopt}
          builtin={adopt.origin === "builtin"}
          extras={sameName(adopt)}
          onClose={() => setAdopt(null)}
          onDone={() => {
            setAdopt(null);
            onChanged();
          }}
          onResult={onResult}
        />
      ) : null}
    </div>
  );
}
