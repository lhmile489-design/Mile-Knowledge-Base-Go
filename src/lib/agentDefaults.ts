// The built-in steering defaults (mirrored from Go DefaultAgentConfigs,
// internal/ai/agents.go:104-134). The settings page needs them for the
// compare view and the "恢复默认" draft action, which never saves by itself.
import type { AgentId } from "./agentMeta";

export const STEERING_DEFAULTS: Record<AgentId, string> = {
  curator:
    "先预览后执行：任何移动、归档或批量改动都先给出清单，等我确认。\n" +
    "保持目录规范：新文件放进既有分类，不要自造平行结构。\n" +
    "默认归档而不是永久删除。",
  scout:
    "来源优先：给结论时附上文件路径，让我能自己核对。\n" +
    "区分「知识库里已有的内容」与「你的推断」，不要混在一起说。\n" +
    "没有命中就直说没有，不要用相关内容凑一个答案。",
  distiller:
    "保留来源与不确定性：提炼时标明每条结论出自哪里，存疑的地方明确写出存疑。\n" +
    "事实不自动升级为约束：读到「某项目这样做」不等于「以后都要这样做」。\n" +
    "产出先进草稿区，需要正式化时交接给整理归档。",
  general:
    "不假装专门流程已经跑过：没检索就说没检索，没读文件就说没读。\n" +
    "需要整理、检索或提炼时，建议我改派给对应的 Agent，或直接发起交接。",
};

/** Steering length cap (PRD FR-AI-STEER; the backend truncates, the editor
 * prevents typing past it). */
export const STEERING_MAX_CHARS = 4000;
