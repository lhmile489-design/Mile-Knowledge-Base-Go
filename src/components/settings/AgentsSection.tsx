import { useEffect } from "react";
import { X } from "@phosphor-icons/react";
import type { ProviderDraft } from "../../mygo";
import { AGENT_IDS, AGENT_META, type AgentId } from "../../lib/agentMeta";
import {
  approvalLabel,
  defaultModelLabel,
  dirtyAgents,
  EFFORT_LABEL,
  EFFORT_LEVELS,
  modelChip,
  modelOptions,
  steeringText,
} from "../../lib/agentSettings";
import {
  describeExecutionMode,
  EXECUTION_MODES,
  EXECUTION_MODE_TO_APPROVAL,
  executionModeOfApproval,
} from "../../lib/executionMode";
import { settingsApi } from "../../lib/transport";
import { useSettingsStore } from "../../stores/settingsStore";
import { useState } from "react";
import { SteeringEditor } from "./SteeringEditor";

/** Settings → Agents (T18). Model, effort (disabled, D-002), execution mode
 * and skills apply immediately; steering is a draft saved from the bar. */
export function AgentsSection({ vaultId, onClose }: { readonly vaultId: string; readonly onClose: () => void }) {
  const config = useSettingsStore((state) => state.config);
  const skills = useSettingsStore((state) => state.skills);
  const drafts = useSettingsStore((state) => state.drafts);
  const error = useSettingsStore((state) => state.error);
  const load = useSettingsStore((state) => state.load);
  const setDraft = useSettingsStore((state) => state.setDraft);
  const discardDrafts = useSettingsStore((state) => state.discardDrafts);
  const saveDrafts = useSettingsStore((state) => state.saveDrafts);
  const patchAgent = useSettingsStore((state) => state.patchAgent);
  const toggleSkill = useSettingsStore((state) => state.toggleSkill);

  const [selected, setSelected] = useState<AgentId>("curator");
  const [providers, setProviders] = useState<ProviderDraft[]>([]);

  useEffect(() => {
    void load(vaultId);
    settingsApi
      .providers(vaultId)
      .then((result) => setProviders(result.providers))
      .catch(() => setProviders([]));
  }, [vaultId, load]);

  const agents = config?.agents ?? {};
  const current = agents[selected];
  const meta = AGENT_META[selected];
  const draft = steeringText(drafts, agents, selected);
  const dirty = dirtyAgents(drafts, agents);
  const models = modelOptions(providers);
  const defaultLabel = defaultModelLabel(providers);
  const mode = executionModeOfApproval(current?.approval);

  return (
    <div className="settings-content" data-testid="agents-section">
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">Agents</h1>
          <p className="settings-subtitle">四个 Agent 共用同一套内容区写权限，这里只改风格与运行参数。</p>
          <div className="settings-chips">
            <span className="settings-chip">随当前知识库</span>
          </div>
        </div>
        <button type="button" className="icon-button" aria-label="关闭设置" onClick={onClose}>
          <X size={16} aria-hidden />
        </button>
      </header>

      <div className="settings-body scroll-y">
        <div className="agent-tabs" role="tablist" aria-label="选择 Agent">
          {AGENT_IDS.map((id) => {
            const agentMeta = AGENT_META[id];
            const Icon = agentMeta.icon;
            const agentConfig = agents[id];
            const active = id === selected;
            return (
              <button
                key={id}
                type="button"
                role="tab"
                aria-selected={active}
                className="agent-tab"
                data-selected={active ? "true" : "false"}
                data-testid={`agent-tab-${id}`}
                onClick={() => setSelected(id)}
              >
                <span className="agent-tab-icon" aria-hidden>
                  <Icon size={15} />
                </span>
                <span className="agent-tab-name">{agentMeta.name}</span>
                <span className="agent-tab-latin">{agentMeta.latin}</span>
                <span className="agent-tab-chips">
                  <span className="agent-tab-chip">{approvalLabel(agentConfig?.approval)}</span>
                  <span className="agent-tab-chip">{modelChip(agentConfig?.model)}</span>
                </span>
              </button>
            );
          })}
        </div>

        <div className="agent-banner">
          <strong>
            {meta.name} · {meta.scope}
          </strong>
          <p>
            可写内容区；可建目录/移动/复制/归档/删除；系统区仍禁写。应用配置、事务恢复记录、索引与来源关系对所有 Agent 禁写，下面任何选项与 steering 文本都改不了这条边界。
          </p>
        </div>

        <section className="agent-block" aria-label="运行参数">
          <h3 className="agent-block-title">
            运行 <span className="agent-block-note">与对话输入栏的运行参数是同一份配置，哪边改都会同步</span>
          </h3>
          <div className="agent-row">
            <div className="agent-row-label">
              <strong>模型</strong>
              <span className="agent-row-hint">「跟随默认」用第一个服务的默认模型，换服务时自动跟着变</span>
            </div>
            <select
              className="field agent-select"
              aria-label="模型"
              value={current?.model ?? ""}
              onChange={(event) => void patchAgent(vaultId, selected, { model: event.target.value })}
              data-testid="agent-model"
            >
              <option value="">跟随默认（{defaultLabel}）</option>
              {models.map((model) => (
                <option key={model} value={model}>
                  {model}
                </option>
              ))}
            </select>
          </div>

          <div className="agent-row">
            <div className="agent-row-label">
              <strong>推理强度</strong>
              <span className="agent-row-hint">越高越慢、越贵；「自动」由模型决定</span>
            </div>
            <div className="segmented" role="radiogroup" aria-label="推理强度">
              {EFFORT_LEVELS.map((level) => (
                <button
                  key={level}
                  type="button"
                  className="segmented-item"
                  role="radio"
                  aria-checked={(current?.effort ?? "auto") === level}
                  disabled
                  title="自研引擎不声明推理强度档位"
                  data-testid={`agent-effort-${level}`}
                >
                  {EFFORT_LABEL[level]}
                </button>
              ))}
            </div>
          </div>

          <div className="agent-row">
            <div className="agent-row-label">
              <strong>执行模式</strong>
              <span className="agent-row-hint">{describeExecutionMode(mode).summary}</span>
            </div>
            <div className="segmented" role="radiogroup" aria-label="执行模式" data-testid="agent-mode">
              {EXECUTION_MODES.map((item) => (
                <button
                  key={item}
                  type="button"
                  className="segmented-item"
                  role="radio"
                  aria-checked={mode === item}
                  onClick={() =>
                    void patchAgent(vaultId, selected, { approval: EXECUTION_MODE_TO_APPROVAL[item] })
                  }
                >
                  {describeExecutionMode(item).label}
                </button>
              ))}
            </div>
          </div>
        </section>

        <section className="agent-block" aria-label="Steering">
          <h3 className="agent-block-title">
            Steering <span className="agent-block-note">告诉它怎么说话、先做什么。只影响风格与优先级，改不了权限</span>
          </h3>
          <SteeringEditor
            agent={selected}
            value={draft}
            onChange={(text) => setDraft(selected, text)}
          />
        </section>

        <section className="agent-block" aria-label="可用 Skills">
          <h3 className="agent-block-title">
            可用 Skills <span className="agent-block-note">勾选的技能会出现在这个 Agent 的工具上下文里</span>
          </h3>
          {skills.length === 0 ? (
            <p className="field-hint">索引就绪后会列出 skills/ 下的技能。</p>
          ) : (
            <div className="skills-list" data-testid="skills-list">
              {skills.map((skill) => {
                const checked = (current?.skillPaths ?? []).includes(skill.path);
                return (
                  <label key={skill.path} className="skill-row">
                    <input
                      type="checkbox"
                      className="field-check"
                      checked={checked}
                      onChange={(event) =>
                        void toggleSkill(vaultId, selected, skill.path, event.target.checked)
                      }
                      data-testid={`skill-${skill.path}`}
                    />
                    <span className="skill-name">{skill.name}</span>
                    <span className="skill-path">{skill.path}</span>
                  </label>
                );
              })}
            </div>
          )}
        </section>

        {error ? (
          <p className="settings-error" role="alert">
            {error}
          </p>
        ) : null}
      </div>

      {dirty.length > 0 ? (
        <div className="settings-savebar" role="status" data-testid="steering-savebar">
          <span>
            有未保存的改动：{dirty.map((id) => AGENT_META[id].name).join("、")} 的 steering
          </span>
          <div className="settings-savebar-actions">
            <button type="button" className="btn line" onClick={discardDrafts}>
              放弃
            </button>
            <button type="button" className="btn-primary" onClick={() => void saveDrafts(vaultId)}>
              保存
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}
