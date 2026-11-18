import { useEffect, useRef, useState } from "react";
import { CaretRight, Gear, PushPin } from "@phosphor-icons/react";
import type { ProvidersResult, SharedContext, TaskItem } from "../../mygo";
import { AGENT_IDS, AGENT_META } from "../../lib/agentMeta";
import {
  credentialState,
  findProviderForModel,
  openTasks,
  permissionBoundary,
  providerHost,
} from "../../lib/sessionInfo";
import { agentApi } from "../../lib/agentApi";
import { settingsApi } from "../../lib/transport";
import { useComposerStore } from "../../stores/composerStore";
import { EngineBadge } from "./EngineBadge";

const TASK_STATUS_LABEL: Record<string, string> = {
  queued: "排队中",
  running: "进行中",
  blocked: "阻塞",
  done: "已完成",
  cancelled: "已取消",
};

/** Session info popover (T16, 340px): engine, model diagnostics, the write
 * boundary, the four duties and the shared context. */
export function SessionInfo({
  vaultId,
  onClose,
}: {
  readonly vaultId: string | null;
  readonly onClose: () => void;
}) {
  const session = useComposerStore((state) => state.session);
  const ref = useRef<HTMLDivElement | null>(null);
  const [providers, setProviders] = useState<ProvidersResult | null>(null);
  const [context, setContext] = useState<SharedContext | null>(null);
  const [openModels, setOpenModels] = useState(false);
  const [openDuties, setOpenDuties] = useState(false);
  const [contextError, setContextError] = useState(false);

  useEffect(() => {
    if (!vaultId) return;
    settingsApi
      .providers(vaultId)
      .then((result) => setProviders(result))
      .catch(() => setProviders(null));
    agentApi
      .contextSnapshot(vaultId)
      .then((result) => setContext(result.context ?? null))
      .catch(() => setContextError(true));
    const off = agentApi.onContextChanged((next) => setContext(next));
    return off;
  }, [vaultId]);

  useEffect(() => {
    const onPointerDown = (event: PointerEvent): void => {
      const target = event.target as Node | null;
      if (target && ref.current && !ref.current.contains(target)) onClose();
    };
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [onClose]);

  const engine = session?.engine;
  const model = session?.session?.model ?? "";
  const provider = findProviderForModel(providers?.providers ?? [], model);
  const keyState = credentialState(providers, provider);
  const tasks = openTasks(context?.taskBoard ?? []);
  const pins = context?.pinnedReferences ?? [];

  const unpin = async (referenceId: string): Promise<void> => {
    try {
      const result = await agentApi.contextUnpin(referenceId);
      if (result.context) setContext(result.context);
    } catch {
      // The context frame (E09) will correct the list anyway.
    }
  };

  return (
    <div className="session-info" role="dialog" aria-label="会话信息" ref={ref} data-testid="session-info">
      <section className="info-section">
        <h4 className="info-heading">引擎</h4>
        <EngineBadge status={engine} />
      </section>

      <section className="info-section">
        <h4 className="info-heading">模型</h4>
        <p className="info-mono">{model === "" ? "引擎默认" : model}</p>
        <button
          type="button"
          className="info-toggle"
          aria-expanded={openModels}
          data-testid="info-models-toggle"
          onClick={() => setOpenModels(!openModels)}
        >
          <CaretRight size={11} weight="bold" aria-hidden className="caret" style={{ transform: openModels ? "rotate(90deg)" : undefined }} />
          模型与凭据
        </button>
        {openModels ? (
          <div className="info-body" data-testid="info-providers">
            {(providers?.providers ?? []).length === 0 ? (
              <p className="info-note">还没有配置模型服务。</p>
            ) : (
              (providers?.providers ?? []).map((item) => {
                const state = credentialState(providers, item);
                const current = provider?.name === item.name;
                return (
                  <div key={item.name} className="info-provider" data-current={current ? "true" : "false"}>
                    <span className="info-provider-name">
                      {item.name}
                      {current ? <span className="info-current">当前</span> : null}
                    </span>
                    <span className="info-provider-meta">
                      {item.kind} · {providerHost(item.baseUrl)} · {item.models.length} 个模型
                    </span>
                    <span className="info-provider-meta">
                      {state === "saved" ? "key 已保存" : state === "missing" ? "key 未保存" : "无需 key"}
                    </span>
                  </div>
                );
              })
            )}
          </div>
        ) : null}
      </section>

      <section className="info-section">
        <h4 className="info-heading">文件读写边界</h4>
        <p className="info-note">
          Agent 的文件读写都经知识库范围检查与事务恢复，并按下列权限边界逐条校验。应用的配置、事务恢复记录与索引不接受任何 Agent 写入。
        </p>
        <button
          type="button"
          className="info-toggle"
          aria-expanded={openDuties}
          data-testid="info-duties-toggle"
          onClick={() => setOpenDuties(!openDuties)}
        >
          <CaretRight size={11} weight="bold" aria-hidden className="caret" style={{ transform: openDuties ? "rotate(90deg)" : undefined }} />
          四职责与权限边界
          <span className="info-toggle-note">自动选择，可改派</span>
        </button>
        {openDuties ? (
          <div className="info-body" data-testid="info-duties">
            {permissionBoundary().map((line) => (
              <p key={line} className="info-note info-boundary">
                {line}
              </p>
            ))}
            {AGENT_IDS.map((id) => (
              <div key={id} className="info-duty">
                <strong>{AGENT_META[id].name}</strong>
                <span>{AGENT_META[id].scope}</span>
              </div>
            ))}
          </div>
        ) : null}
      </section>

      <section className="info-section">
        <h4 className="info-heading">共享上下文</h4>
        {contextError ? <p className="info-note">共享上下文不可用。</p> : null}
        {context ? (
          <p className="info-note">
            索引版本 {context.vault.indexRevision} · 内容版本 {context.vault.revision}
          </p>
        ) : null}
        {tasks.length === 0 && pins.length === 0 ? (
          <p className="info-note">暂无未完成任务或固定引用。</p>
        ) : null}
        {tasks.length > 0 ? (
          <div className="info-body">
            {tasks.map((task: TaskItem) => (
              <div key={task.id} className="info-task">
                <span className="pill" data-tone={task.status === "blocked" ? "warn" : "info"}>
                  {TASK_STATUS_LABEL[task.status] ?? task.status}
                </span>
                <span className="info-task-owner">{AGENT_META[task.owner as keyof typeof AGENT_META]?.name ?? task.owner}</span>
                <span className="info-task-goal">{task.goal}</span>
              </div>
            ))}
          </div>
        ) : null}
        {pins.length > 0 ? (
          <div className="info-body">
            {pins.map((pin) => (
              <div key={pin.id} className="info-pin">
                <PushPin size={11} aria-hidden />
                <span className="info-pin-path">{pin.path}</span>
                <span className="info-note">版本 {pin.revision}</span>
                <button
                  type="button"
                  className="icon-button"
                  aria-label={`取消固定 ${pin.path}`}
                  title="取消固定"
                  onClick={() => void unpin(pin.id)}
                >
                  ×
                </button>
              </div>
            ))}
          </div>
        ) : null}
      </section>

      <footer className="info-footer">
        <button type="button" className="btn line" disabled title="设置">
          <Gear size={13} aria-hidden />
          设置
        </button>
      </footer>
    </div>
  );
}
