import type { SkillRemote, SkillUpdateFile } from "./transport";

/** GitHub update states shown as pills (UI 稿 9.4). */
export type RemotePill = { text: string; tone: "live" | "warn" | "idle" | "fault"; title?: string };

export function remotePill(remote?: SkillRemote | null): RemotePill | null {
  if (!remote?.updateState) return null;
  switch (remote.updateState) {
    case "update-available":
      return {
        text: "GitHub 有更新",
        tone: "warn",
        title: remote.pathCommit && remote.latestPathCommit
          ? `${shortSha(remote.pathCommit)} → ${shortSha(remote.latestPathCommit)}`
          : undefined,
      };
    case "both-changed":
      return { text: "两边都改了", tone: "warn", title: "本地改过，不能一键更新，可以重新安装到别的分类" };
    case "local-modified":
      return { text: "GitHub · 本地改过", tone: "idle" };
    case "up-to-date":
      return { text: "GitHub · 最新", tone: "live" };
    case "error":
      return { text: "检查失败", tone: "fault", title: remote.error };
    default:
      return { text: "GitHub · 未检查", tone: "idle" };
  }
}

export function shortSha(value?: string): string {
  return value ? value.slice(0, 7) : "";
}

export function sourceOf(remote: SkillRemote): string {
  return `${remote.owner}/${remote.repo}`;
}

/** "12 分钟前检查过" for the panel footer (Q-SKM-2). */
export function checkedLabel(checkedAt?: string, now = Date.now()): string {
  if (!checkedAt) return "";
  const time = Date.parse(checkedAt);
  if (Number.isNaN(time)) return "";
  const minutes = Math.max(0, Math.round((now - time) / 60000));
  if (minutes < 1) return "刚刚检查过";
  if (minutes < 60) return `${minutes} 分钟前检查过`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前检查过`;
  return `${Math.floor(hours / 24)} 天前检查过`;
}

/** One-line count used by the preview footer and the tool confirm card. */
export function changeSummary(files: readonly SkillUpdateFile[]): string {
  const count = (change: string) => files.filter((file) => file.change === change).length;
  const parts: string[] = [];
  const modified = count("modified");
  const added = count("added");
  const removed = count("removed");
  if (modified > 0) parts.push(`修改 ${modified}`);
  if (added > 0) parts.push(`新增 ${added}`);
  if (removed > 0) parts.push(`删除 ${removed}`);
  return parts.length === 0 ? "没有变化" : parts.join(" · ");
}

export function clockOf(value?: string): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
