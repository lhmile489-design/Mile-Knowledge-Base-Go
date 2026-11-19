import { useEffect, useMemo } from "react";
import { X } from "@phosphor-icons/react";
import {
  aliasOf,
  buildCategoryRows,
  isMeaningfulCategory,
  withIndexed,
  withoutDirectory,
  type CategoryRow,
} from "../../lib/categoriesTable";
import { categoryAliasDraftId } from "../../lib/settingsDrafts";
import { useAppStore } from "../../stores/appStore";
import { useSettingsStore } from "../../stores/settingsStore";
import { useTreeStore } from "../../stores/treeStore";
import { iconFor } from "../treeIcons";
import { SettingGroup } from "./SettingRow";

/** Settings → 分类与检索 (T06). Rows are the vault's real top-level
 * directories plus leftover configurations; the include-in-search switch
 * applies immediately (M65 + D-013), the display name is a draft. */
export function CategoriesSection({ onClose }: { readonly onClose: () => void }) {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const config = useSettingsStore((state) => state.config);
  const drafts = useSettingsStore((state) => state.drafts);
  const setAliasDraft = useSettingsStore((state) => state.setAliasDraft);
  const clearAliasDraft = useSettingsStore((state) => state.clearAliasDraft);
  const setCategories = useSettingsStore((state) => state.setCategories);
  const treeVaultId = useTreeStore((state) => state.vaultId);
  const rootEntries = useTreeStore((state) => state.nodes[""]);
  const rootLoading = useTreeStore((state) => state.loading[""]);
  const loadTree = useTreeStore((state) => state.load);

  useEffect(() => {
    if (!vaultId || treeVaultId !== vaultId || rootEntries || rootLoading) return;
    void loadTree("");
  }, [vaultId, treeVaultId, rootEntries, rootLoading, loadTree]);

  const rows = useMemo(
    () => buildCategoryRows(rootEntries ?? [], config?.categories ?? []),
    [rootEntries, config?.categories],
  );

  const commitIndexed = (row: CategoryRow, indexed: boolean): void => {
    const categories = config?.categories ?? [];
    const next = withIndexed(categories, row, indexed).filter(isMeaningfulCategory);
    void setCategories(vaultId, next);
  };

  const clearMissing = (row: CategoryRow): void => {
    // 草稿一起清掉，否则保存时会把已清除目录的显示名又写回去。
    clearAliasDraft(row.directory);
    void setCategories(vaultId, withoutDirectory(config?.categories ?? [], row.directory));
  };

  const isSystemRow = (row: CategoryRow): boolean =>
    row.area === "system" || row.directory.startsWith(".");

  return (
    <div className="settings-content" data-testid="settings-categories">
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">分类与检索</h1>
          <p className="settings-subtitle">列表来自当前知识库的真实一级目录。图标按目录名自动判定。</p>
          <div className="settings-chips">
            <span className="settings-chip">随当前知识库</span>
          </div>
        </div>
        <button type="button" className="icon-button" aria-label="关闭设置" onClick={onClose}>
          <X size={16} aria-hidden />
        </button>
      </header>

      <div className="settings-body scroll-y">
        <div className="settings-stack">
          <SettingGroup
            title="一级目录"
            description="图标按目录名自动判定，不单独配置"
            footnote="关掉「进检索」后，该目录下的文件不进全文检索，链接与健康检查不受影响；系统区始终不进检索。显示名只改界面上的叫法，不改磁盘上的目录名，输完走底部保存条统一保存。"
            testid="settings-categories-group"
          >
            <div className="cat-head" aria-hidden="true">
              <span />
              <span>目录</span>
              <span>显示名</span>
              <span style={{ textAlign: "center" }}>进检索</span>
            </div>

            {rows.length === 0 ? (
              <p className="field-hint px-3 py-2">这个知识库还没有一级目录。</p>
            ) : null}

            {rows.map((row) =>
              row.present ? (
                <div className="cat-row" key={row.directory} data-testid={`category-row-${row.directory}`}>
                  <span className="cat-icon" aria-hidden>
                    {(() => {
                      const entry = rootEntries?.find((item) => item.name === row.directory);
                      const Icon = iconFor(entry?.icon ?? "folder");
                      return <Icon size={14} />;
                    })()}
                  </span>

                  <span className="cat-dir">
                    <span className="cat-dir-name">{row.directory}</span>
                  </span>

                  <input
                    className="field"
                    value={aliasOf(row, drafts, categoryAliasDraftId)}
                    placeholder={row.directory}
                    onChange={(event) => setAliasDraft(row.directory, event.target.value)}
                    aria-label={`${row.directory} 的显示名`}
                    data-testid={`category-alias-${row.directory}`}
                  />

                  <span className="cat-ix">
                    <button
                      type="button"
                      role="switch"
                      aria-checked={row.indexed}
                      aria-label={`${row.directory} 进检索`}
                      className="switch"
                      disabled={isSystemRow(row)}
                      title={isSystemRow(row) ? "系统区不进检索" : undefined}
                      onClick={() => commitIndexed(row, !row.indexed)}
                      data-testid={`category-indexed-${row.directory}`}
                    />
                  </span>
                </div>
              ) : (
                <div
                  className="cat-row"
                  data-missing="true"
                  key={row.directory}
                  data-testid={`category-row-${row.directory}`}
                >
                  <span className="cat-icon" aria-hidden>
                    {(() => {
                      const Icon = iconFor("folder");
                      return <Icon size={14} />;
                    })()}
                  </span>

                  <span className="cat-dir">
                    <span className="cat-dir-name">{row.directory}</span>
                    <span className="pill" data-tone="warn">
                      目录已不存在
                    </span>
                  </span>

                  <span className="cat-missing-note">配置还留着</span>

                  <span className="cat-ix">
                    <button
                      type="button"
                      className="btn danger-quiet"
                      aria-label={`清除 ${row.directory} 的残留配置`}
                      onClick={() => clearMissing(row)}
                      data-testid={`category-clear-${row.directory}`}
                    >
                      清除
                    </button>
                  </span>
                </div>
              ),
            )}
          </SettingGroup>
        </div>
      </div>
    </div>
  );
}
