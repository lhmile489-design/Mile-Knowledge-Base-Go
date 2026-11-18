import { memo } from "react";
import { describePhase } from "../../lib/agentPhase";

/** Breathing indicator. Waiting states never animate: the user is the one
 * who must act, and an animation there would suggest otherwise. */
export const ThinkingIndicator = memo(function ThinkingIndicator({
  phase,
  detail,
}: {
  readonly phase: string;
  readonly detail?: string;
}) {
  const presentation = describePhase(phase);
  return (
    <div
      className="thinking-row chat-thinking"
      data-animate={presentation.animate ? "true" : "false"}
      role="status"
      aria-live="polite"
      data-testid="thinking-indicator"
    >
      <span className="thinking-dots" aria-hidden>
        <span />
        <span />
        <span />
      </span>
      <span className="thinking-label" data-shimmer={presentation.animate ? "true" : undefined}>
        {presentation.label}
        {detail ? ` · ${detail}` : ""}
      </span>
    </div>
  );
});
