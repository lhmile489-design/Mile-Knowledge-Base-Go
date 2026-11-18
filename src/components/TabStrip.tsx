import { PushPin, X } from "@phosphor-icons/react";
import { useDocStore, isDirty, tabTitle } from "../stores/docStore";
import { useAppStore } from "../stores/appStore";

/** Document tabs (PRD FR-DOC): horizontal strip, dirty dot, double-click pins,
 * close button. Closing a dirty tab asks first. */
export function TabStrip({ onCloseRequest }: { onCloseRequest: (path: string) => void }) {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const tabs = useDocStore((state) => state.tabs);
  const docs = useDocStore((state) => state.docs);
  const activePath = useDocStore((state) => state.activePath);
  const setActive = useDocStore((state) => state.setActive);
  const togglePin = useDocStore((state) => state.togglePin);

  if (tabs.length === 0) return null;

  return (
    <div className="tab-strip" role="tablist" aria-label="打开的文档">
      {tabs.map((tab) => {
        const doc = docs[tab.path];
        const dirty = isDirty(doc);
        return (
          <div
            key={tab.path}
            role="tab"
            aria-selected={activePath === tab.path}
            className="tab-item"
            title={tab.path}
            onClick={() => setActive(tab.path)}
            onDoubleClick={() => togglePin(tab.path)}
          >
            {tab.pinned ? <PushPin size={11} weight="fill" aria-label="已置顶" /> : null}
            <span className="tab-label">{tabTitle(tab.path)}</span>
            {dirty ? (
              <>
                <span className="tab-dirty" aria-hidden />
                <span className="sr-only">未保存</span>
              </>
            ) : null}
            <button
              type="button"
              className="icon-btn tab-close"
              aria-label={`关闭 ${tabTitle(tab.path)}`}
              onClick={(event) => {
                event.stopPropagation();
                void onCloseRequest(tab.path);
              }}
              disabled={!vaultId}
            >
              <X size={11} aria-hidden />
            </button>
          </div>
        );
      })}
    </div>
  );
}
