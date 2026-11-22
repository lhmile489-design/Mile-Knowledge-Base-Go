import { FolderOpen, Globe } from "@phosphor-icons/react";
import type { SkillTool } from "../../lib/transport";

// Brand SVGs ship in src/assets/agents with NOTICE (DESIGN.md 11.2; D-014
// 第 4 条). They are the one exception to "icons are Phosphor".
const FILES = import.meta.glob("../../assets/agents/*.svg", {
  eager: true,
  query: "?url",
  import: "default",
}) as Record<string, string>;

function sourceFor(iconKey: string): string | undefined {
  if (!iconKey) return undefined;
  return FILES[`../../assets/agents/${iconKey}.svg`];
}

/** One tool icon: shared dir → globe, unknown key → letter placeholder. */
export function ToolIcon({
  tool,
  size = 16,
}: {
  readonly tool?: Pick<SkillTool, "id" | "name" | "iconKey">;
  readonly size?: number;
}) {
  if (!tool) return null;
  if (tool.id === "agents") {
    return <Globe size={size} aria-hidden className="tool-icon tool-icon-globe" />;
  }
  if (tool.iconKey === "folder") {
    return <FolderOpen size={size} aria-hidden className="tool-icon tool-icon-folder" />;
  }
  const source = sourceFor(tool.iconKey);
  if (!source) {
    return (
      <span className="tool-icon tool-icon-letter" style={{ width: size, height: size, fontSize: size * 0.62 }} aria-hidden>
        {tool.name.slice(0, 1)}
      </span>
    );
  }
  return <img className="tool-icon" src={source} alt="" aria-hidden width={size} height={size} title={tool.name} />;
}
