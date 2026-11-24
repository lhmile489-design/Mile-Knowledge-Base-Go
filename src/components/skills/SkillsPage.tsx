import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useAppStore } from "../../stores/appStore";
import { useSkillsStore } from "../../stores/skillsStore";
import { SkillsNav, type SkillsSelection } from "./SkillsNav";

/** The standalone skills page (M8 T03 skeleton; views land in T04–T07). It is
 * mounted only while `page === "skills"`, so every entry starts fresh and the
 * scan is refreshed then. The main area stays empty until the kb/tools views
 * land; the navigation is the deliverable of this task. */
export function SkillsPage() {
  const vaultId = useAppStore((state) => state.activeVaultId);
  const scan = useSkillsStore((state) => state.scan);
  const tools = useSkillsStore((state) => state.tools);
  const [selection, setSelection] = useState<SkillsSelection>({ kind: "kb", category: null });
  const shellRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    void useSkillsStore.getState().refresh(vaultId);
  }, [vaultId]);

  // A vault switch can drop the selected category or tool; fall back to all.
  useEffect(() => {
    if (!scan) return;
    setSelection((selection) => {
      if (selection.kind === "kb" && selection.category && !scan.kb.some((group) => group.category === selection.category)) {
        return { kind: "kb", category: null };
      }
      if (selection.kind === "tools" && selection.toolId && !scan.tools.some((group) => group.toolId === selection.toolId)) {
        return { kind: "tools", toolId: null };
      }
      return selection;
    });
  }, [scan]);

  // Entering the page puts focus on the current navigation item (UI 稿 11.1);
  // the page unmounts on leave, so mount is the right moment.
  useLayoutEffect(() => {
    shellRef.current?.querySelector<HTMLElement>('[aria-current="page"]')?.focus();
  }, []);

  return (
    <section className="skills-page" data-testid="skills-page" ref={shellRef}>
      <SkillsNav
        selection={selection}
        onSelect={setSelection}
        scan={scan}
        tools={tools}
        hasVault={Boolean(vaultId)}
      />
      <main className="skills-main" data-testid="skills-main" aria-label="技能内容" />
    </section>
  );
}
