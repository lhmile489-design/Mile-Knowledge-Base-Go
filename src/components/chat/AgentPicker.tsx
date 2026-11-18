import { useEffect, useRef } from "react";
import { Sparkle } from "@phosphor-icons/react";
import { AGENT_IDS, AGENT_META } from "../../lib/agentMeta";
import { useComposerStore } from "../../stores/composerStore";

/** The reassignment menu: "自动" plus the four agents. The manual choice
 * stays until the user explicitly picks 自动 again. */
export function AgentPicker() {
  const override = useComposerStore((state) => state.override);
  const chooseAgent = useComposerStore((state) => state.chooseAgent);
  const closePicker = useComposerStore((state) => state.closePicker);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onPointerDown = (event: PointerEvent): void => {
      const target = event.target as Node | null;
      if (target && containerRef.current && !containerRef.current.contains(target)) closePicker();
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") closePicker();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [closePicker]);

  return (
    <div className="agent-picker" role="menu" aria-label="发给哪个 Agent" ref={containerRef} data-testid="agent-picker">
      <div className="agent-picker-head">发给哪个 Agent</div>
      <button
        type="button"
        role="menuitemradio"
        aria-checked={override === null}
        className="agent-option"
        data-selected={override === null ? "true" : "false"}
        data-testid="agent-option-auto"
        onClick={() => chooseAgent(null)}
      >
        <span className="agent-option-icon" aria-hidden>
          <Sparkle size={16} />
        </span>
        <span className="agent-option-text">
          <span className="agent-option-name">自动</span>
          <span className="agent-option-scope">按你的说法自动选择处理者</span>
        </span>
        {override === null ? <span className="agent-option-badge">使用中</span> : null}
      </button>

      {AGENT_IDS.map((id) => {
        const meta = AGENT_META[id];
        const Icon = meta.icon;
        const selected = override === id;
        return (
          <button
            key={id}
            type="button"
            role="menuitemradio"
            aria-checked={selected}
            className="agent-option"
            data-selected={selected ? "true" : "false"}
            data-testid={`agent-option-${id}`}
            onClick={() => chooseAgent(id)}
          >
            <span className="agent-option-icon" aria-hidden>
              <Icon size={16} />
            </span>
            <span className="agent-option-text">
              <span className="agent-option-name">{meta.name}</span>
              <span className="agent-option-scope">{meta.scope}</span>
            </span>
            {selected ? <span className="agent-option-badge">使用中</span> : null}
          </button>
        );
      })}
    </div>
  );
}
