import { formatMonthDayTime } from "../../lib/conversationTime";
import type { RestoreNote as RestoreNoteState } from "../../stores/historyStore";

/** Startup restore note (T15): one line between two hairlines. "开始新对话"
 * is the only exit; the note never blocks anything. */
export function RestoreNote({
  state,
  onStartNew,
}: {
  readonly state: NonNullable<RestoreNoteState>;
  readonly onStartNew: () => void;
}) {
  const when = state.kind === "restored" ? formatMonthDayTime(state.updatedAt) : "";
  return (
    <div className="restore-note" role="status" data-testid="restore-note" data-kind={state.kind}>
      <span className="restore-note-rule" aria-hidden />
      <span>
        {state.kind === "restored"
          ? `已恢复上次对话${when ? ` · ${when}` : ""}`
          : "上次的对话无法恢复"}
      </span>
      <button type="button" onClick={onStartNew} data-testid="restore-start-new">
        开始新对话
      </button>
      <span className="restore-note-rule" aria-hidden />
    </div>
  );
}
