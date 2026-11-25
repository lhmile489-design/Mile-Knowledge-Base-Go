import { useEffect, useMemo, useState } from "react";
import { Archive, ArrowClockwise, FolderOpen, Tray } from "@phosphor-icons/react";
import { skillsApi, type SkillRestorePreview, type SkillTool, type SkillToolScan, type ToolSkill } from "../../lib/transport";
import { remotePill, sourceOf } from "../../lib/skillRemote";
import { isToolDetected } from "../../lib/skillsView";
import { ToolIcon } from "./ToolIcon";
import { UninstallDialog } from "./UninstallDialog";
import { AdoptDialog } from "./AdoptDialog";
import { ToolUpdateDialog } from "./ToolUpdateDialog";

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
  "vault-missing": { text: "知识库已删除", tone: "warn" },
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
  onManageTargets,
  onOpenRestore,
  onInstall,
  title = "全部工具",
}: {
  readonly vaultId: string;
  readonly groups: SkillToolScan[];
  readonly tools: SkillTool[];
  readonly onChanged: () => void;
  readonly onResult?: (lines: string[]) => void;
  readonly onManageTargets?: () => void;
  readonly onOpenRestore?: () => void;
  readonly onInstall?: () => void;
  readonly title?: string;
}) {
  const [chip, setChip] = useState("all");
  const [showHidden, setShowHidden] = useState(false);
  const [uninstall, setUninstall] = useState<ToolSkill | null>(null);
  const [adopt, setAdopt] = useState<ToolSkill | null>(null);
  const [updating, setUpdating] = useState<ToolSkill | null>(null);
  const [restore, setRestore] = useState<SkillRestorePreview | null>(null);

  // The manifest banner (UI 稿 9.6): pending items or an unreadable file.
  useEffect(() => {
    if (!vaultId) {
      setRestore(null);
      return;
    }
    let alive = true;
    void skillsApi
      .restorePreview(vaultId)
      .then((value) => {
        if (alive) setRestore(value.preview ?? null);
      })
      .catch(() => {
        if (alive) setRestore(null);
      });
    return () => {
      alive = false;
    };
  }, [vaultId]);

  const restoreBanner = (() => {
    if (!restore || !onOpenRestore) return null;
    if (restore.status === "ok" && restore.pending > 0) {
      return (
        <div className="restore-banner" data-testid="restore-banner">
          <span className="restore-banner-text">知识库清单里有 {restore.pending} 项尚未在本机还原</span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={() => onOpenRestore()} data-testid="restore-banner-open">
            查看并还原
          </button>
        </div>
      );
    }
    if (restore.status === "unreadable" || restore.status === "too-large") {
      return (
        <div className="restore-banner" data-tone="warn" data-testid="restore-banner">
          <span className="restore-banner-text">分发清单无法读取，之后的分发不会写入清单</span>
          <span className="spacer" />
          <button type="button" className="btn-quiet" onClick={() => onOpenRestore()} data-testid="restore-banner-open">
            用本机记录重建
          </button>
        </div>
      );
    }
    if (restore.status === "version-unknown") {
      return (
        <div className="restore-banner" data-tone="warn" data-testid="restore-banner">
          <span className="restore-banner-text">分发清单来自更新版本的应用，请升级后再还原</span>
        </div>
      );
    }
    return null;
  })();

  const countOf = (id: string) => groups.find((group) => group.toolId === id)?.skills.length ?? 0;
  // Undetected tools stay folded even when their leftover directory still
  // holds skills (UI 稿 9.2: 默认折叠); the navigation uses the same rule.
  const visible = (tool: SkillTool) => showHidden || isToolDetected(tool);
  const chips = useMemo(() => tools.filter(visible), [tools, groups, showHidden]);
  const hiddenCount = tools.filter((tool) => !isToolDetected(tool)).length;
  const detectedCount = tools.filter((tool) => tool.id !== "agents" && tool.status === "detected").length;

  // Rows stay grouped by tool (UI 稿 11.2); the chip row only narrows the
  // groups for the legacy panel until T09 moves the entry to the navigation.
  const visibleGroups = useMemo(() => {
    return groups
      .filter((group) => chip === "all" || group.toolId === chip)
      .map((group) => ({
        ...group,
        skills: [...group.skills].sort((a, b) => a.name.toLowerCase().localeCompare(b.name.toLowerCase())),
      }))
      .filter((group) => group.skills.length > 0);
  }, [groups, chip]);
  const rows = useMemo(
    () => visibleGroups.flatMap((group) => group.skills.map((skill) => ({ skill, toolId: group.toolId }))),
    [visibleGroups],
  );

  const specOf = (skill: ToolSkill) => (skill.base && skill.rel ? `${skill.base}:${skill.rel}` : skill.name);
  const toolFor = (id: string) => tools.find((tool) => tool.id === id);
  const activeTarget = chip === "all" ? undefined : toolFor(chip);
  // Masked paths are not unique across projects; identity uses the wire base.
  const identityOf = (skill: ToolSkill, toolId: string) =>
    `${toolId}:${skill.subdir ?? ""}:${skill.rel ?? skill.name}`;
  const sameName = (skill: ToolSkill, toolId: string) =>
    rows
      .filter(
        (row) =>
          row.skill.name === skill.name && identityOf(row.skill, row.toolId) !== identityOf(skill, toolId),
      )
      .map((row) => ({ toolId: row.toolId, path: specOf(row.skill) }));

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
      {restoreBanner}
      <div className="targets-toolbar" data-testid="tool-manage-row">
        <span className="tool-chip-count" data-testid="tool-manage-count">
          自定义 {tools.filter((tool) => tool.kind === "custom").length} · 项目{" "}
          {tools.filter((tool) => tool.kind === "project").length}
        </span>
        <span className="spacer" />
        <button type="button" className="btn-quiet" onClick={() => onManageTargets?.()} data-testid="manage-targets">
          <FolderOpen size={13} aria-hidden />
          管理目录
        </button>
      </div>
      <div className="tool-toolbar">
        <h2 className="tool-title">{title}</h2>
        <span className="tool-count">
          {rows.length} 个 · {visibleGroups.length} 个工具
        </span>
        <span className="spacer" />
        {onInstall ? (
          <button type="button" className="btn-primary" data-testid="kb-install-top" onClick={onInstall}>
            安装
          </button>
        ) : null}
      </div>
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

      {activeTarget && activeTarget.kind !== "builtin" ? (
        <div className="kb-group-title" data-testid="tool-group-caption">
          {activeTarget.kind === "project" ? "项目" : "自定义"} · {activeTarget.name}
          <span className="tool-chip-count">{rows.length}</span>
        </div>
      ) : null}

      {rows.length === 0 ? (
        <div className="skills-empty">
          <p>{detectedCount === 0 ? "没有检测到已安装的工具。" : "没有可显示的工具技能。"}</p>
          {detectedCount === 0 ? (
            <p className="field-hint">探测依据：CLI、安装记录、安装目录。装好工具后重开浮层即可看到。</p>
          ) : null}
        </div>
      ) : (
        <div className="tool-groups">
          {visibleGroups.map((group) => {
            const tool = toolFor(group.toolId);
            const label = tool?.name ?? group.toolId;
            return (
              <section key={group.toolId} className="tool-group" data-testid={`tool-group-${group.toolId}`}>
                <div className="tool-group-head">
                  <span className="tool-group-ico" aria-hidden>
                    <ToolIcon tool={tool ?? { id: group.toolId, name: label, iconKey: "" }} size={16} />
                  </span>
                  <span className="tool-group-name">{label}</span>
                  <span className="tool-group-count">{group.skills.length}</span>
                </div>
                <div className="tool-list">
                  {group.skills.map((skill) => {
                    const remote = skill.remote ? remotePill(skill.remote) : null;
                    const status = remote ?? statusOf(skill);
                    const updatable = skill.remote?.updateState === "update-available";
                    return (
                      <div key={identityOf(skill, group.toolId)} className="tool-row" data-testid={`tool-skill-${skill.name}-${group.toolId}`}>
                        <span
                          className="tool-row-icon"
                          title={skill.sourceToolId && tool ? `${toolFor(skill.sourceToolId)?.name ?? skill.sourceToolId}${skill.subdir ? ` · ${skill.subdir}` : ""}` : undefined}
                        >
                          <ToolIcon tool={skill.sourceToolId ? toolFor(skill.sourceToolId) : tool} size={18} />
                        </span>
                        <span className="tool-row-name">{skill.name}</span>
                        <span className="tool-row-path" title={skill.path}>
                          {skill.path}
                          {skill.remote ? ` · ${sourceOf(skill.remote)}` : ""}
                        </span>
                        <span className="pill" data-tone={status.tone} title={remote?.title}>
                          {status.text}
                        </span>
                        <span className="tool-row-actions">
                          {updatable ? (
                            <button
                              type="button"
                              className="icon-button"
                              aria-label={`更新 ${skill.name}`}
                              title="更新"
                              onClick={() => setUpdating(skill)}
                              data-testid={`tool-update-${skill.name}-${group.toolId}`}
                            >
                              <ArrowClockwise size={14} aria-hidden />
                            </button>
                          ) : null}
                          {skill.origin === "copied" && skill.sync === "vault-updated" && skill.sourcePath && tool?.kind !== "project" ? (
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
                              data-testid={`tool-uninstall-${skill.name}-${group.toolId}`}
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
                              data-testid={`tool-adopt-${skill.name}-${group.toolId}`}
                            >
                              <Archive size={14} aria-hidden />
                            </button>
                          ) : null}
                          <button
                            type="button"
                            className="icon-button"
                            aria-label={`打开所在目录 ${skill.name}`}
                            onClick={() => void skillsApi.reveal(group.toolId, specOf(skill)).catch(() => undefined)}
                          >
                            <FolderOpen size={14} aria-hidden />
                          </button>
                        </span>
                      </div>
                    );
                  })}
                </div>
              </section>
            );
          })}
        </div>
      )}

      {uninstall ? (
        <UninstallDialog
          toolId={uninstall.toolId}
          path={specOf(uninstall)}
          skillName={uninstall.name}
          onClose={() => setUninstall(null)}
          onDone={() => {
            setUninstall(null);
            onChanged();
          }}
          onResult={onResult}
        />
      ) : null}

      {updating?.remote ? (
        <ToolUpdateDialog
          vaultId={vaultId}
          toolId={updating.toolId}
          toolName={toolFor(updating.toolId)?.name ?? updating.toolId}
          skillName={updating.name}
          spec={specOf(updating)}
          remote={updating.remote}
          onClose={() => setUpdating(null)}
          onChanged={onChanged}
          onResult={onResult}
        />
      ) : null}

      {adopt ? (
        <AdoptDialog
          vaultId={vaultId}
          skill={adopt}
          spec={specOf(adopt)}
          builtin={adopt.origin === "builtin"}
          extras={sameName(adopt, adopt.toolId)}
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
