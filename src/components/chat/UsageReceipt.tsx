import type { Usage } from "../../mygo";
import { useAgentStore } from "../../stores/agentStore";
import { useComposerStore } from "../../stores/composerStore";
import { formatCacheHit, formatCost, formatTokens } from "../../lib/usageFormat";

function Row({ usage, label }: { readonly usage: Usage; readonly label: string }) {
  const cost = formatCost(usage);
  const cache = formatCacheHit(usage);
  return (
    <div className="usage-row">
      <span className="usage-label">{label}</span>
      <span className="usage-tokens">
        <span title="输入 token">{formatTokens(usage.promptTokens)}</span>
        <span className="usage-sep"> in · </span>
        <span title="输出 token">{formatTokens(usage.completionTokens)}</span>
        <span className="usage-sep"> out</span>
        {usage.reasoningTokens > 0 ? (
          <>
            <span className="usage-sep"> · </span>
            <span title="推理 token（推理模型的主要成本项，单独显示）">
              {formatTokens(usage.reasoningTokens)} 推理
            </span>
          </>
        ) : null}
      </span>
      <span
        className="usage-mono"
        style={cache.known ? undefined : { color: "var(--raw-faint)" }}
        title="缓存命中率"
      >
        {cache.text}
      </span>
      <span
        className="usage-mono"
        style={cost.known ? undefined : { color: "var(--raw-faint)" }}
        title={cost.known ? "估算费用（来自引擎，不由界面计算）" : "本轮未提供费用数据"}
      >
        {cost.text}
      </span>
    </div>
  );
}

/** Usage receipt above the runtime parameters (T14). Numbers come from the
 * engine; missing values stay "—" and cost is never recomputed here. */
export function UsageReceipt() {
  const usage = useAgentStore((state) => state.buckets[state.agent].usage);
  const model = useComposerStore((state) => state.session?.session?.model ?? "");

  if (!usage) return null;

  return (
    <div className="usage-receipt" data-testid="usage-receipt" aria-label="用量与费用">
      {model ? <div className="usage-model">{model}</div> : null}
      <Row usage={usage.turn} label="本轮" />
      <Row usage={usage.cumulative} label="累计" />
    </div>
  );
}
