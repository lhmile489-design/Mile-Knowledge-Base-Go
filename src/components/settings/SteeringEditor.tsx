import { useEffect, useState } from "react";
import { X } from "@phosphor-icons/react";
import { STEERING_DEFAULTS, STEERING_MAX_CHARS } from "../../lib/agentDefaults";
import { AGENT_META, type AgentId } from "../../lib/agentMeta";

/** Steering editor (T18): draft text, counter, compare-with-default dialog
 * and the reset-to-default draft action. Nothing here saves by itself. */
export function SteeringEditor({
  agent,
  value,
  onChange,
}: {
  readonly agent: AgentId;
  readonly value: string;
  readonly onChange: (text: string) => void;
}) {
  const [compareOpen, setCompareOpen] = useState(false);
  const defaultText = STEERING_DEFAULTS[agent];

  useEffect(() => {
    if (!compareOpen) return;
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") setCompareOpen(false);
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [compareOpen]);

  return (
    <div className="steering-editor">
      <textarea
        className="field field-textarea steering-area"
        maxLength={STEERING_MAX_CHARS}
        value={value}
        aria-label={`${AGENT_META[agent].name} 的 Steering`}
        data-testid="steering-input"
        onChange={(event) => onChange(event.target.value)}
      />
      <div className="steering-count">
        {value.length} / {STEERING_MAX_CHARS}
      </div>
      <div className="steering-actions">
        <button
          type="button"
          className="btn line"
          onClick={() => setCompareOpen(true)}
          data-testid="steering-compare"
        >
          与默认对比
        </button>
        <button
          type="button"
          className="btn line"
          onClick={() => onChange(defaultText)}
          data-testid="steering-reset"
        >
          恢复默认
        </button>
        <span className="steering-hint">改动在底部保存条统一保存</span>
      </div>

      {compareOpen ? (
        <div className="settings-dialog-backdrop" role="presentation" onClick={() => setCompareOpen(false)}>
          <div
            className="settings-dialog"
            role="dialog"
            aria-label={`${AGENT_META[agent].name} 的 Steering 与默认对比`}
            onClick={(event) => event.stopPropagation()}
            data-testid="steering-compare-dialog"
          >
            <div className="settings-dialog-head">
              <strong>与默认对比 · {AGENT_META[agent].name}</strong>
              <button type="button" className="icon-button" aria-label="关闭对比" onClick={() => setCompareOpen(false)}>
                <X size={14} aria-hidden />
              </button>
            </div>
            <div className="steering-compare">
              <div>
                <h4 className="info-heading">内置默认</h4>
                <pre className="steering-pre">{defaultText}</pre>
              </div>
              <div>
                <h4 className="info-heading">当前草稿</h4>
                <pre className="steering-pre">{value}</pre>
              </div>
            </div>
            <p className="field-hint">「恢复默认」只替换草稿，保存条统一保存后才生效。</p>
          </div>
        </div>
      ) : null}
    </div>
  );
}
