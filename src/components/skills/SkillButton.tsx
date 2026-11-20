import { useEffect } from "react";
import { Stack } from "@phosphor-icons/react";
import { badgeCount } from "../../lib/skillsView";
import { useAppStore } from "../../stores/appStore";
import { useSkillsStore } from "../../stores/skillsStore";

/** Title-bar 「技能」 pill with the attention badge (PRD FR-SKM-PANEL; UI 稿
 * 9.1). It sits between the vault name and the accent dot. */
export function SkillButton() {
  const open = useAppStore((state) => state.skillsPanelOpen);
  const openPanel = useAppStore((state) => state.openSkillsPanel);
  const closePanel = useAppStore((state) => state.closeSkillsPanel);
  const vaultId = useAppStore((state) => state.activeVaultId);
  const scan = useSkillsStore((state) => state.scan);

  // The badge needs a scan; the first one is cached-first and cheap, and a
  // vault switch invalidates the knowledge-base half. Without a vault the
  // tools half still loads (PRD FR-SKM-PANEL: 无库时工具页照常可用).
  useEffect(() => {
    void useSkillsStore.getState().refresh(vaultId);
  }, [vaultId]);

  const count = badgeCount(scan);
  const label = count > 0 ? `技能，${count} 项需要处理` : "技能";

  return (
    <button
      type="button"
      className="skill-button no-drag"
      data-testid="open-skills"
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={open}
      aria-controls="skills-panel"
      onClick={() => (open ? closePanel() : openPanel())}
    >
      <Stack size={14} aria-hidden />
      <span>技能</span>
      {count > 0 ? (
        <span className="skill-badge" data-testid="skills-badge" aria-hidden>
          {count}
        </span>
      ) : null}
    </button>
  );
}
