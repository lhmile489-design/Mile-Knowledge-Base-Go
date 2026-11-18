import { CaretRight, Warning } from "@phosphor-icons/react";
import {
  currentEffort,
  currentExecutionMode,
  useComposerStore,
} from "../../stores/composerStore";
import { useAgentStore } from "../../stores/agentStore";
import { useAppStore } from "../../stores/appStore";
import { describeExecutionMode, EXECUTION_MODES } from "../../lib/executionMode";
import { usageSummary } from "../../lib/usageFormat";

const EFFORT_LEVELS = ["auto", "disabled", "low", "high", "max"] as const;
const EFFORT_LABEL: Record<string, string> = {
  auto: "自动",
  disabled: "关闭",
  low: "低",
  high: "高",
  max: "最高",
};

/** Runtime parameters bar: model / effort / execution mode, collapsed to a
 * one-line summary. The effort control stays disabled (D-002). */
export function RunParams() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const collapsed = useAppStore((state) => state.settings?.layout?.agnesControlsCollapsed ?? false);
  const session = useComposerStore((state) => state.session);
  const paramsError = useComposerStore((state) => state.paramsError);
  const toggleParams = useComposerStore((state) => state.toggleParams);
  const updateModel = useComposerStore((state) => state.updateModel);
  const updateExecutionMode = useComposerStore((state) => state.updateExecutionMode);
  const usage = useAgentStore((state) => state.buckets[state.agent].usage);

  const info = session?.session;
  const models = info?.availableModels ?? [];
  const model = info?.model ?? "";
  const effort = currentEffort(session);
  const mode = currentExecutionMode(session);
  const description = describeExecutionMode(mode);
  const usageText = usageSummary(usage);
  const summary = [
    model === "" ? "引擎默认" : model,
    EFFORT_LABEL[effort] ?? EFFORT_LABEL.auto,
    description.label,
    usageText,
  ]
    .filter((part): part is string => Boolean(part))
    .join(" · ");

  return (
    <div className="runtime-controls" data-testid="runtime-controls" data-collapsed={collapsed ? "true" : "false"}>
      <button
        type="button"
        className="params-toggle"
        aria-expanded={!collapsed}
        aria-controls="runtime-controls-body"
        title={collapsed ? "展开运行参数" : "折叠运行参数"}
        data-testid="runtime-controls-toggle"
        onClick={toggleParams}
      >
        <CaretRight
          size={11}
          weight="bold"
          aria-hidden
          className="caret"
          style={{ transform: collapsed ? undefined : "rotate(90deg)" }}
        />
        <span className="params-heading">运行参数</span>
        {collapsed ? <span className="params-summary">{summary}</span> : null}
      </button>

      {!collapsed ? (
        <div className="params-body" id="runtime-controls-body">
          <div className="params-row">
            <span className="params-label">模型</span>
            <select
              className="field params-select"
              aria-label="模型"
              value={model}
              disabled={!vaultId || models.length === 0}
              onChange={(event) => {
                if (!vaultId) return;
                void updateModel(vaultId, event.target.value);
              }}
              data-testid="control-model"
            >
              <option value="">引擎默认</option>
              {models.map((option) => (
                <option key={option.modelId} value={option.modelId}>
                  {option.name}
                </option>
              ))}
            </select>
          </div>

          <div className="params-row">
            <span className="params-label">强度</span>
            <div className="segmented min-w-0" role="radiogroup" aria-label="推理强度">
              {EFFORT_LEVELS.map((level) => (
                <button
                  key={level}
                  type="button"
                  className="segmented-item"
                  role="radio"
                  aria-checked={effort === level}
                  disabled
                  title="自研引擎不声明推理强度档位"
                  data-testid={`effort-${level}`}
                >
                  {EFFORT_LABEL[level]}
                </button>
              ))}
            </div>
          </div>

          <div className="params-row">
            <span className="params-label">执行</span>
            <div className="segmented min-w-0" role="radiogroup" aria-label="任务执行模式" data-testid="control-execution-mode">
              {EXECUTION_MODES.map((item) => {
                const active = mode === item;
                return (
                  <button
                    key={item}
                    type="button"
                    className="segmented-item"
                    role="radio"
                    aria-checked={active}
                    disabled={!vaultId}
                    title={describeExecutionMode(item).summary}
                    data-testid={`execution-mode-${item}`}
                    onClick={() => {
                      if (!vaultId || active) return;
                      void updateExecutionMode(vaultId, item);
                    }}
                  >
                    {describeExecutionMode(item).label}
                  </button>
                );
              })}
            </div>
          </div>

          <p className="params-note" data-testid="execution-mode-summary">
            {description.summary}
          </p>

          {description.consequence !== null ? (
            <div className="params-consequence" data-testid="execution-mode-consequence">
              <Warning size={12} weight="bold" aria-hidden />
              <span>{description.consequence}</span>
            </div>
          ) : null}

          {paramsError ? (
            <p className="params-note" style={{ color: "var(--fault)" }} role="alert">
              {paramsError}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
