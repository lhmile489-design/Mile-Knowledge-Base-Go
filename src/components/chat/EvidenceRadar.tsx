import { useId, useMemo, useState } from "react";
import { CaretRight } from "@phosphor-icons/react";
import { useAppStore } from "../../stores/appStore";
import { collectEvidence, type EvidenceItem } from "../../lib/chatReferences";
import { openReference, type RefStatus } from "../../lib/openReference";
import type { TranscriptLine } from "../../lib/transcript";

/** Citation evidence: real references, real counts, first-appearance [n].
 * No match percentages exist, so none are shown. Collapsed by default; the
 * list unmounts when closed so hidden buttons leave the tab order. */
export function EvidenceRadar({ lines }: { readonly lines: readonly TranscriptLine[] }) {
  const evidence = useMemo(() => collectEvidence(lines), [lines]);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const [open, setOpen] = useState(false);
  const [statuses, setStatuses] = useState<Record<string, RefStatus>>({});
  const listId = useId();

  if (evidence.length === 0) return null;

  const openItem = async (item: EvidenceItem): Promise<void> => {
    if (!vaultId) return;
    const key = `${item.refKind}:${item.target}`;
    const status = await openReference(vaultId, item.refKind, item.target);
    setStatuses((previous) => ({ ...previous, [key]: status }));
  };

  const noteFor = (key: string): string | null => {
    const status = statuses[key];
    if (status === "missing") return "找不到";
    if (status === "ambiguous") return "有多个同名";
    return null;
  };

  return (
    <details
      className="evidence"
      data-testid="evidence-radar"
      data-source-count={evidence.length}
      open={open}
      onToggle={(event) => {
        const next = event.currentTarget.open;
        if (next !== open) setOpen(next);
      }}
    >
      <summary data-testid="evidence-toggle" aria-expanded={open} aria-controls={open ? listId : undefined}>
        <CaretRight size={11} weight="bold" aria-hidden className="caret" />
        引用 {evidence.length} 个来源
      </summary>
      {open ? (
        <ul className="evidence-list" id={listId} data-testid="evidence-list">
          {evidence.map((item) => {
            const key = `${item.refKind}:${item.target}`;
            const note = noteFor(key);
            return (
              <li className="evidence-item" key={key}>
                <button
                  type="button"
                  className="chip evidence-chip"
                  data-status={statuses[key] ?? "idle"}
                  data-ref-kind={item.refKind}
                  data-ref-target={item.target}
                  title={item.refKind === "wiki" ? `[[${item.target}]]` : item.target}
                  aria-label={`打开 ${item.target}`}
                  onClick={() => void openItem(item)}
                >
                  <span className="evidence-path">{item.target}</span>
                  <span className="evidence-cite">[{item.index}]</span>
                  {item.count > 1 ? <span className="evidence-count">{item.count} 次</span> : null}
                </button>
                {note ? <span className="evidence-note">{note}</span> : null}
              </li>
            );
          })}
        </ul>
      ) : null}
    </details>
  );
}
