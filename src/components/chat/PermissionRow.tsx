import { Warning } from "@phosphor-icons/react";
import { useAgentStore } from "../../stores/agentStore";

/** Pending approval cards. T11 renders the minimal action row so a manual
 * test can complete a write; T14 replaces it with the full card (diff preview,
 * 60-line clip, Go-confirmed removal). Cards leave the list when the Go side
 * pushes the approved/rejected/expired frame — never on the IPC reply. */
export function PermissionRow() {
  const pending = useAgentStore((state) => state.buckets[state.agent].pendingPermissions);
  const approve = useAgentStore((state) => state.approve);
  const reject = useAgentStore((state) => state.reject);
  if (pending.length === 0) return null;

  return (
    <>
      {pending.map((permission) => (
        <div
          key={permission.requestId}
          className="status-card"
          data-tone="warn"
          role="group"
          aria-label="需要你的确认"
          data-testid="permission-row"
        >
          <div className="status-card-head">
            <Warning size={13} weight="bold" aria-hidden />
            需要你的确认
          </div>
          <div className="status-card-body">
            {permission.detail}
            {permission.targetPath ? `（${permission.targetPath}）` : ""}
          </div>
          <div className="status-card-actions">
            <button type="button" className="btn line" onClick={() => void approve(permission.requestId)}>
              允许
            </button>
            <button type="button" className="btn line" onClick={() => void reject(permission.requestId)}>
              拒绝
            </button>
          </div>
        </div>
      ))}
    </>
  );
}
