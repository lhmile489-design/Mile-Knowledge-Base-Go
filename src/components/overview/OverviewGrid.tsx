import { useEffect } from "react";
import type { CSSProperties } from "react";
import { AGENT_IDS, AGENT_META } from "../../lib/agentMeta";
import {
  agentStatus,
  healthCounts,
  healthSummary,
  indexDetail,
  indexLabel,
  pendingCount,
} from "../../lib/overview";
import { events } from "../../lib/transport";
import { useAgentStore } from "../../stores/agentStore";
import { useAppStore } from "../../stores/appStore";
import { useComposerStore } from "../../stores/composerStore";
import { useDocStore } from "../../stores/docStore";
import { useOverviewStore } from "../../stores/overviewStore";
import { useTreeStore } from "../../stores/treeStore";

const KIND_LABEL: Record<string, string> = {
  create: "新建",
  createDirectory: "新建目录",
  edit: "写入",
  move: "移动",
  copy: "复制",
  archive: "归档",
  trash: "回收站",
  permanentDelete: "永久删除",
};

function basename(path: string): string {
  const at = path.lastIndexOf("/");
  return at >= 0 ? path.slice(at + 1) : path;
}

/** The workbench overview (M3 T09; PRD FR-OVW). Shown in the chat column while
 * the target agent has neither messages nor pending approvals. */
export function OverviewGrid() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const openHealth = useAppStore((state) => state.openHealth);
  const toggleAudit = useAppStore((state) => state.toggleAudit);
  const recent = useOverviewStore((state) => state.recent);
  const recentLoading = useOverviewStore((state) => state.recentLoading);
  const recentError = useOverviewStore((state) => state.recentError);
  const status = useOverviewStore((state) => state.status);
  const health = useOverviewStore((state) => state.health);
  const drafts = useOverviewStore((state) => state.drafts);
  const changes = useOverviewStore((state) => state.changes);
  const load = useOverviewStore((state) => state.load);
  const scheduleLoad = useOverviewStore((state) => state.scheduleLoad);
  const setStatus = useOverviewStore((state) => state.setStatus);
  const rootEntries = useTreeStore((state) => state.nodes[""]);
  const skillCategories = useTreeStore((state) => state.skillCategories);
  const reveal = useTreeStore((state) => state.reveal);
  const buckets = useAgentStore((state) => state.buckets);
  const openDoc = useDocStore((state) => state.open);
  const activePath = useDocStore((state) => state.activePath);
  const enginePhase = useComposerStore((state) => state.session?.engine?.phase);

  useEffect(() => {
    if (!vaultId) return;
    void load(vaultId);
    const offVault = events.vaultChanged.on((event) => {
      if (event.vaultId === vaultId) scheduleLoad(vaultId);
    });
    const offIndex = events.indexStatus.on((event) => {
      if (!event.vaultId || event.vaultId === vaultId) setStatus(event);
    });
    const offAudit = events.auditChanged.on((event) => {
      if (!event.vaultId || event.vaultId === vaultId) scheduleLoad(vaultId);
    });
    return () => {
      offVault();
      offIndex();
      offAudit();
    };
  }, [vaultId, load, scheduleLoad, setStatus]);

  // 打开文档会更新最近列表（M73），但记录是读盘成功后才写的，晚一拍刷新。
  useEffect(() => {
    if (!vaultId || !activePath) return;
    const timer = window.setTimeout(() => void load(vaultId), 600);
    return () => window.clearTimeout(timer);
  }, [activePath, vaultId, load]);

  const pending = pendingCount(buckets);
  const counts = healthCounts(health);
  const ready = status?.phase === "ready";
  const directories = (rootEntries ?? []).filter((entry) => entry.kind === "directory");
  const now = new Date();
  const glow = (phase: string | undefined): CSSProperties =>
    ({
      "--glow":
        phase === "ready" ? "var(--live)" : phase === "offline" ? "var(--fault)" : "var(--idle)",
    }) as CSSProperties;

  const focusComposer = (): void => {
    document.querySelector<HTMLTextAreaElement>(".composer-input")?.focus();
  };

  return (
    <div className="overview-shell" data-testid="overview-grid">
      <div className="overview-grid">
        <section className="overview-tile" data-span="2x2" data-testid="overview-recent">
          <span className="overview-title">最近文档</span>
          {recentLoading && recent.length === 0 ? (
            <p className="overview-line">正在读取</p>
          ) : recentError ? (
            <p className="overview-line">读取失败：{recentError}</p>
          ) : recent.length === 0 ? (
            <p className="overview-line">还没有打开过文件。从左侧目录树选一个开始。</p>
          ) : (
            <ul className="overview-list">
              {recent.slice(0, 12).map((item) => (
                <li key={item.path}>
                  <button
                    type="button"
                    className="overview-row"
                    onClick={() => vaultId && void openDoc(vaultId, item.path)}
                    data-testid={`overview-recent-${item.path}`}
                  >
                    <strong>{basename(item.path)}</strong>
                    <span>{item.path}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="overview-tile" data-testid="overview-index">
          <span className="overview-title">文档与索引</span>
          <span className="overview-main">{indexLabel(status?.phase)}</span>
          <span className="overview-line">
            {status?.documentCount ?? 0} 篇{indexDetail(status, now)}
          </span>
        </section>

        <button
          type="button"
          className="overview-tile overview-button"
          data-testid="overview-skills"
          onClick={() => useAppStore.getState().showPage("skills")}
        >
          <span className="overview-title">技能分类</span>
          <span className="overview-main">{skillCategories}</span>
          <span className="overview-line">
            {rootEntries?.some((entry) => entry.name === "skills") === false
              ? "没有 skills 目录"
              : "打开技能浮层"}
          </span>
        </button>

        <button
          type="button"
          className="overview-tile overview-button"
          onClick={() => openHealth("unresolvedLinks")}
          data-testid="overview-health"
        >
          <span className="overview-title">健康</span>
          <span className="overview-main">{counts.total}</span>
          <span className="overview-line">{healthSummary(health, ready)}</span>
        </button>

        <button
          type="button"
          className="overview-tile overview-button"
          onClick={() => void reveal(".mile-kb/drafts")}
          data-testid="overview-drafts"
        >
          <span className="overview-title">草稿</span>
          <span className="overview-main">{drafts}</span>
          <span className="overview-line">
            {drafts === 0 ? "蒸馏学习还没有产出草稿" : "在侧栏中打开草稿目录"}
          </span>
        </button>

        <button
          type="button"
          className="overview-tile overview-button"
          data-warn={pending > 0 ? "true" : undefined}
          onClick={focusComposer}
          data-testid="overview-pending"
          data-count={pending}
        >
          <span className="overview-title">待审批</span>
          <span className="overview-main">{pending}</span>
          <span className="overview-line">
            {pending > 0 ? `有 ${pending} 项等待你确认` : "没有等待确认的操作"}
          </span>
        </button>

        <section className="overview-tile" data-span="2x1" data-testid="overview-areas">
          <span className="overview-title">分区</span>
          {directories.length === 0 ? (
            <span className="overview-line">知识库根目录下还没有目录</span>
          ) : (
            <div className="overview-chips">
              {directories.map((entry) => (
                <button
                  key={entry.path}
                  type="button"
                  className="overview-chip"
                  onClick={() => void reveal(entry.path)}
                >
                  {entry.name}
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="overview-tile" data-span="2x1" data-testid="overview-agents">
          <span className="overview-title">
            Agent 状态
            <span className="status-dot" data-halo aria-hidden style={glow(enginePhase)} />
          </span>
          <ul className="overview-list">
            {AGENT_IDS.map((id) => (
              <li key={id} className="overview-agent-row">
                <span>{AGENT_META[id].name}</span>
                <span className="overview-line">{agentStatus(buckets[id], enginePhase)}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className="overview-tile" data-span="2x1" data-testid="overview-changes">
          <span className="overview-title">最近改动</span>
          {changes.length === 0 ? (
            <span className="overview-line">还没有记录</span>
          ) : (
            <ul className="overview-list">
              {changes.slice(0, 5).map((record) => (
                <li key={record.id} className="overview-change-row">
                  <span className="overview-line">{KIND_LABEL[record.kind] ?? record.kind}</span>
                  <span className="overview-line">{record.target ?? record.source ?? ""}</span>
                </li>
              ))}
            </ul>
          )}
          <button type="button" className="overview-link" onClick={() => toggleAudit()}>
            打开操作日志
          </button>
        </section>
      </div>
    </div>
  );
}
