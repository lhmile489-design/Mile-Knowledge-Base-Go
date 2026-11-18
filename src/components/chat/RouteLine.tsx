import { CaretDown } from "@phosphor-icons/react";
import { AGENT_META, type AgentId } from "../../lib/agentMeta";
import { useAgentStore } from "../../stores/agentStore";
import { useComposerStore } from "../../stores/composerStore";
import { AgentPicker } from "./AgentPicker";

/** "发给 X" plus the reason and the permission summary. Typing only updates
 * the preview; it never changes who the current conversation belongs to. */
export function RouteLine() {
  const text = useComposerStore((state) => state.text);
  const preview = useComposerStore((state) => state.preview);
  const override = useComposerStore((state) => state.override);
  const rerouteRequested = useComposerStore((state) => state.rerouteRequested);
  const pickerOpen = useComposerStore((state) => state.pickerOpen);
  const togglePicker = useComposerStore((state) => state.togglePicker);

  const currentTarget = useAgentStore((state) => state.agent);
  const hasConversation = useAgentStore((state) =>
    state.buckets[state.agent].transcript.lines.some((line) => line.role === "user"),
  );

  const decisionAgent = (preview?.agent ?? currentTarget) as AgentId;
  const autoRoute = rerouteRequested || !hasConversation;
  const agent = override ?? (autoRoute ? decisionAgent : currentTarget);
  const meta = AGENT_META[agent];

  let reason: string;
  if (override) {
    reason = `已手动指定 ${meta.name}，直到你改回「自动」`;
  } else if (autoRoute && preview && text.trim() !== "") {
    reason = preview.reason;
  } else if (!hasConversation) {
    reason = "按你的输入自动选择处理者";
  } else {
    reason = "继续当前对话，避免把上一轮上下文断开";
  }

  return (
    <div className="chat-route-line">
      <button
        type="button"
        className="route-target"
        aria-haspopup="menu"
        aria-expanded={pickerOpen}
        data-testid="route-target"
        onClick={togglePicker}
      >
        <span className="route-who">发给 {meta.name}</span>
        <CaretDown size={11} aria-hidden />
      </button>
      <span className="route-reason" data-testid="route-reason">
        {reason}
      </span>
      <span className="route-perm" data-testid="route-perm">
        {meta.scope} · 可写内容区；可建目录/移动/复制/归档/删除；系统区仍禁写
      </span>
      {pickerOpen ? <AgentPicker /> : null}
    </div>
  );
}
