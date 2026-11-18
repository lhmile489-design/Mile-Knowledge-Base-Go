import { memo } from "react";
import type { TranscriptLine } from "../../lib/transcript";
import { lineTimeLabel } from "../../lib/conversationTime";

/** User message: right aligned, 72% max width, plain text (never Markdown). */
export const UserMessage = memo(function UserMessage({ line }: { readonly line: TranscriptLine }) {
  const label = lineTimeLabel(line.at);
  return (
    <div className="msg-user" data-testid="msg-user">
      <span className="msg-user-label">你{label ? ` · ${label}` : ""}</span>
      <div className="msg-user-bubble" data-testid="msg-user-bubble">
        {line.text}
      </div>
    </div>
  );
});
