import { useEffect, useMemo, useRef, useState } from "react";
import { MagnifyingGlass, PencilSimple, PushPin, Trash } from "@phosphor-icons/react";
import { AGENT_META, isAgentId } from "../../lib/agentMeta";
import { groupHistory, rowMeta } from "../../lib/conversationGroups";
import { useAppStore } from "../../stores/appStore";
import { useHistoryStore } from "../../stores/historyStore";

const CONFIRM_MIN_GAP_MS = 600;
const CONFIRM_RESET_MS = 4000;

/** History rail (T15): search, pinned/day groups, row actions and the footer.
 * Deleting and clearing need the two-step confirmation (600ms gap). */
export function HistoryRail({ vaultId }: { readonly vaultId: string | null }) {
  const open = useHistoryStore((state) => state.open);
  const index = useHistoryStore((state) => state.index);
  const viewer = useHistoryStore((state) => state.viewer);
  const searchText = useHistoryStore((state) => state.searchText);
  const hits = useHistoryStore((state) => state.hits);
  const setSearch = useHistoryStore((state) => state.setSearch);
  const openViewer = useHistoryStore((state) => state.openViewer);
  const rename = useHistoryStore((state) => state.rename);
  const togglePin = useHistoryStore((state) => state.togglePin);
  const remove = useHistoryStore((state) => state.remove);
  const clearAll = useHistoryStore((state) => state.clearAll);

  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [armed, setArmed] = useState<Record<string, number | null>>({});
  const gates = useRef<Record<string, number | null>>({});
  const mounted = useRef(true);
  useEffect(() => () => { mounted.current = false; }, []);

  const groups = useMemo(() => groupHistory(index?.conversations ?? [], new Date()), [index]);
  const total = index?.conversations.length ?? 0;
  const now = useMemo(() => new Date(), [index]);

  if (!open) return null;

  /** Returns true when the action may fire now (armed and past the 600ms gap). */
  const twoStep = (key: string): boolean => {
    const current = Date.now();
    const previous = gates.current[key] ?? null;
    if (previous === null) {
      gates.current[key] = current;
      setArmed({ ...gates.current });
      // PRD FR-AI-PERM: the armed state resets after four seconds.
      window.setTimeout(() => {
        if (mounted.current && gates.current[key] === current) {
          gates.current[key] = null;
          setArmed({ ...gates.current });
        }
      }, CONFIRM_RESET_MS);
      return false;
    }
    if (current - previous < CONFIRM_MIN_GAP_MS) return false;
    gates.current[key] = null;
    setArmed({ ...gates.current });
    return true;
  };

  const searching = searchText.trim() !== "";
  const startRename = (id: string, title: string): void => {
    setRenamingId(id);
    setDraft(title);
  };
  const commitRename = async (id: string): Promise<void> => {
    const title = draft.trim();
    setRenamingId(null);
    if (!vaultId || title === "") return;
    await rename(vaultId, id, title);
  };

  return (
    <aside className="history-rail" aria-label="对话历史" data-testid="history-rail">
      <div className="history-search">
        <MagnifyingGlass size={13} aria-hidden />
        <input
          className="history-search-input"
          value={searchText}
          placeholder="搜索历史对话"
          aria-label="搜索历史对话"
          onChange={(event) => setSearch(vaultId, event.target.value)}
          data-testid="history-search"
        />
      </div>

      <div className="history-list scroll-y">
        {searching ? (
          hits.length === 0 ? (
            <p className="history-empty">没有匹配的对话。</p>
          ) : (
            <ul className="history-hits" data-testid="history-hits">
              {hits.map((hit) => (
                <li key={hit.conversationId}>
                  <button
                    type="button"
                    className="history-row"
                    data-testid="history-hit"
                    onClick={() => void openViewer(vaultId, hit.conversationId)}
                  >
                    <span className="history-row-title">{hit.title}</span>
                    <span className="history-row-meta">{hit.excerpt}</span>
                  </button>
                </li>
              ))}
            </ul>
          )
        ) : total === 0 ? (
          <p className="history-empty">还没有历史对话。发出一条消息后会出现在这里。</p>
        ) : (
          groups.map((group) => (
            <section className="history-group" key={group.id}>
              <h3 className="history-group-label">{group.label}</h3>
              <ul>
                {group.items.map((meta) => {
                  const row = rowMeta(meta, now);
                  const agent = row.agent && isAgentId(row.agent) ? AGENT_META[row.agent] : null;
                  const active = viewer?.id === meta.id;
                  const deleting = armed[meta.id] != null;
                  return (
                    <li key={meta.id} className="history-row-wrap">
                      {renamingId === meta.id ? (
                        <input
                          className="field history-rename"
                          autoFocus
                          value={draft}
                          aria-label="重命名对话"
                          onChange={(event) => setDraft(event.target.value)}
                          onBlur={() => void commitRename(meta.id)}
                          onKeyDown={(event) => {
                            if (event.key === "Enter") void commitRename(meta.id);
                            if (event.key === "Escape") setRenamingId(null);
                          }}
                        />
                      ) : (
                        <button
                          type="button"
                          className="history-row"
                          data-selected={active ? "true" : "false"}
                          data-testid="history-row"
                          onClick={() => void openViewer(vaultId, meta.id)}
                        >
                          <span className="history-row-title">
                            {meta.pinned ? <PushPin size={11} weight="fill" aria-label="已置顶" /> : null}
                            {meta.title}
                          </span>
                          <span className="history-row-meta">
                            {agent ? agent.name : "—"}
                            {row.time ? ` · ${row.time}` : ""}
                          </span>
                        </button>
                      )}
                      <div className="history-row-actions">
                        <button
                          type="button"
                          className="icon-button"
                          aria-label={meta.pinned ? "取消置顶" : "置顶"}
                          title={meta.pinned ? "取消置顶" : "置顶"}
                          onClick={() => vaultId && void togglePin(vaultId, meta.id, !meta.pinned)}
                        >
                          <PushPin size={12} weight={meta.pinned ? "fill" : "regular"} aria-hidden />
                        </button>
                        <button
                          type="button"
                          className="icon-button"
                          aria-label="重命名"
                          title="重命名"
                          onClick={() => startRename(meta.id, meta.title)}
                        >
                          <PencilSimple size={12} aria-hidden />
                        </button>
                        <button
                          type="button"
                          className={`icon-button${deleting ? " history-confirm" : ""}`}
                          aria-label={deleting ? "确认删除" : "删除"}
                          title={deleting ? "再点一次确认删除" : "删除"}
                          onClick={() => {
                            if (!vaultId) return;
                            if (twoStep(`del:${meta.id}`)) void remove(vaultId, meta.id);
                          }}
                          data-testid="history-delete"
                        >
                          <Trash size={12} aria-hidden />
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
            </section>
          ))
        )}
      </div>

      <div className="history-footer">
        <span>共 {total} 条</span>
        <button
          type="button"
          className={`history-clear${armed["clear"] != null ? " history-confirm" : ""}`}
          disabled={total === 0}
          data-testid="history-clear"
          onClick={() => {
            if (!vaultId) return;
            if (twoStep("clear")) void clearAll(vaultId);
          }}
        >
          {armed["clear"] != null ? "确认清空" : "清空全部"}
        </button>
      </div>
    </aside>
  );
}
