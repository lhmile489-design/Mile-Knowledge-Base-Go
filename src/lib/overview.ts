// Overview tiles (M3 T09; PRD FR-OVW). Pure view helpers so the tile texts and
// counts are testable without mounting the workbench.
import type { ChatBucket } from "./chatStream";
import type { FileEntry, IndexStatus, KbHealthReport } from "../mygo";

/** Index phase in the tile's words (legacy INDEX_LABEL). */
export function indexLabel(phase: string | undefined): string {
  switch (phase) {
    case "scanning":
      return "正在建立";
    case "ready":
      return "已就绪";
    case "error":
      return "建立失败";
    default:
      return "尚未建立";
  }
}

/** Coarse "how long ago" for the last build (legacy relative label). */
export function relativeTime(iso: string | undefined, now: Date): string {
  if (!iso) return "";
  const then = new Date(iso);
  if (Number.isNaN(then.getTime())) return "";
  const minutes = Math.floor((now.getTime() - then.getTime()) / 60000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  return `${Math.floor(hours / 24)} 天前`;
}

/** Second line of the index tile: progress while scanning, age when ready. */
export function indexDetail(status: IndexStatus | null | undefined, now: Date): string {
  if (!status) return "";
  if (status.phase === "scanning" && status.scanned > 0) return `· 已处理 ${status.scanned}`;
  if (status.phase === "ready" && status.lastBuiltAt) {
    const age = relativeTime(status.lastBuiltAt, now);
    return age ? `· ${age}` : "";
  }
  return "";
}

export interface HealthCounts {
  readonly unresolved: number;
  readonly orphans: number;
  readonly duplicates: number;
  readonly untitled: number;
  readonly total: number;
}

/** Four issue counts out of one health report. */
export function healthCounts(report: KbHealthReport | null | undefined): HealthCounts {
  const unresolved = report?.unresolvedLinks?.length ?? 0;
  const orphans = report?.orphans?.length ?? 0;
  const duplicates = report?.duplicates?.length ?? 0;
  const untitled = report?.untitled?.length ?? 0;
  return {
    unresolved,
    orphans,
    duplicates,
    untitled,
    total: unresolved + orphans + duplicates + untitled,
  };
}

/** The health tile's one-line summary (legacy wording). */
export function healthSummary(report: KbHealthReport | null | undefined, ready: boolean): string {
  if (!report) return "正在读取";
  const counts = healthCounts(report);
  const line = `断链 ${counts.unresolved} · 孤立 ${counts.orphans} · 重名 ${counts.duplicates} · 缺标题 ${counts.untitled}`;
  if (!ready) return `索引未就绪 · ${line}`;
  if (counts.total === 0) return "断链、孤立、重名、缺标题都是 0";
  return line;
}

/** Files in the drafts directory; a missing directory is zero, not an error. */
export function draftsCount(entries: readonly FileEntry[] | null | undefined): number {
  if (!entries) return 0;
  return entries.filter((entry) => entry.kind === "file").length;
}

/** Pending approval cards across the four agents. */
export function pendingCount(buckets: Record<string, ChatBucket>): number {
  let total = 0;
  for (const bucket of Object.values(buckets)) total += bucket.pendingPermissions.length;
  return total;
}

/** One agent's status word for the status tile (legacy labels). */
export function agentStatus(
  bucket: ChatBucket | undefined,
  enginePhase: string | undefined,
): string {
  if (enginePhase === "offline") return "不可用";
  if (enginePhase === "starting") return "启动中";
  if (!bucket) return "未启动";
  if (bucket.pendingPermissions.length > 0) return "等待你确认";
  if (bucket.busy) return "正在工作";
  return bucket.transcript.lines.length > 0 ? "就绪" : "未启动";
}
