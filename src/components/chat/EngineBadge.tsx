import type { CSSProperties } from "react";
import type { EngineStatus } from "../../mygo";

/** Engine state row (T16): a status dot plus words — the dot is never the
 * only carrier of the phase. */
export function EngineBadge({ status }: { readonly status?: EngineStatus }) {
  const phase = status?.phase ?? "";
  const ready = phase === "ready";
  const starting = phase === "starting";
  const glow = ready ? "var(--live)" : phase === "offline" ? "var(--fault)" : "var(--idle)";
  const label = ready ? "已连接 · 内置引擎" : starting ? "正在启动" : "离线";
  return (
    <span className="engine-badge" data-phase={phase || "unknown"} data-testid="engine-badge">
      <span className="status-dot" data-halo aria-hidden style={{ "--glow": glow } as CSSProperties} />
      <span className="engine-label">
        {label}
        {!ready && status?.message ? ` · ${status.message}` : ""}
      </span>
    </span>
  );
}
