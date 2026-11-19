import { MAX_CATEGORY_ALIAS } from "../../lib/settingsDrafts";

/** Page-level save bar (T04). Lists the dirty drafts and saves them in one
 * action; refuses over-long display names instead of truncating them. */
export function SettingsSaveBar({
  labels,
  error,
  saving,
  onDiscard,
  onSave,
}: {
  readonly labels: string[];
  readonly error: string | null;
  readonly saving: boolean;
  readonly onDiscard: () => void;
  readonly onSave: () => void;
}) {
  if (labels.length === 0 && !error) return null;
  return (
    <div className="settings-savebar" role="status" data-testid="settings-savebar">
      <span className="settings-savebar-text">
        {labels.length > 0 ? `有未保存的改动：${labels.join("、")}` : null}
        {error ? (
          <span className="settings-savebar-error" role="alert" data-testid="settings-savebar-error">
            {error}
          </span>
        ) : null}
      </span>
      <div className="settings-savebar-actions">
        <button type="button" className="btn line" onClick={onDiscard} disabled={saving}>
          放弃
        </button>
        <button
          type="button"
          className="btn-primary"
          onClick={onSave}
          disabled={saving || labels.length === 0}
          data-testid="settings-savebar-save"
          title={`保存（Ctrl+S）；显示名上限 ${MAX_CATEGORY_ALIAS} 字符`}
        >
          {saving ? "保存中…" : "保存"}
        </button>
      </div>
    </div>
  );
}
