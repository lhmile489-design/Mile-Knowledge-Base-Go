import { useEffect, useRef, useState } from "react";
import { CaretRight, DotsThree, Lock, PlugsConnected, Sparkle, Star, X } from "@phosphor-icons/react";
import type { ProviderDraft, ProvidersResult } from "../../mygo";
import {
  DEEPSEEK_PRESET,
  keyBadge,
  moveToFront,
  providerAddress,
  providerInitials,
} from "../../lib/providerForm";
import { settingsApi, TransportError } from "../../lib/transport";
import { ProviderForm } from "./ProviderForm";

const CONFIRM_MIN_GAP_MS = 600;

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  if (error instanceof Error) return error.message;
  return String(error);
}

/** Settings → 模型服务 (T17). */
export function ModelServiceSection({
  vaultId,
  data,
  error,
  reload,
  onClose,
}: {
  readonly vaultId: string;
  readonly data: ProvidersResult | null;
  readonly error: string | null;
  readonly reload: () => Promise<void>;
  readonly onClose: () => void;
}) {
  const [form, setForm] = useState<{ initial: ProviderDraft | null; index: number | null } | null>(null);
  const [showKeyInfo, setShowKeyInfo] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [testResults, setTestResults] = useState<Record<string, string>>({});
  const [armed, setArmed] = useState<{ key: string; at: number } | null>(null);
  const [errorLine, setErrorLine] = useState<string | null>(null);
  const [menu, setMenu] = useState<string | null>(null);
  const [keyEditor, setKeyEditor] = useState<{ name: string; envName: string } | null>(null);
  const [keyValue, setKeyValue] = useState("");
  const [keyBusy, setKeyBusy] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  // 菜单：点外面或按 Esc 关闭（与旧版一致）。按压事件在菜单内不能关，
  // 否则菜单项会在 click 之前被卸载，删除和更换 Key 都点不到。
  useEffect(() => {
    if (!menu) return;
    const close = (): void => setMenu(null);
    const onPointerDown = (event: PointerEvent): void => {
      const target = event.target as Node | null;
      if (target && menuRef.current && menuRef.current.contains(target)) return;
      close();
    };
    const onKey = (event: KeyboardEvent): void => {
      if (event.key === "Escape") close();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [menu]);

  const providers = data?.providers ?? [];
  const credentials = data?.credentials ?? [];

  const testProvider = async (provider: ProviderDraft): Promise<void> => {
    if (testing) return;
    setTesting(provider.name);
    setTestResults((previous) => ({ ...previous, [provider.name]: "" }));
    try {
      const result = await settingsApi.fetchModels(provider.kind, provider.baseUrl, provider.apiKeyEnv, "");
      const text = result.failure
        ? `测试失败：${result.failure.message}`
        : `可用 · ${result.models?.length ?? 0} 个模型`;
      setTestResults((previous) => ({ ...previous, [provider.name]: text }));
    } catch (err) {
      setTestResults((previous) => ({ ...previous, [provider.name]: `测试失败：${errorText(err)}` }));
    } finally {
      setTesting(null);
    }
  };

  const deleteProvider = async (provider: ProviderDraft, index: number): Promise<void> => {
    const now = Date.now();
    if (armed && armed.key === provider.name && now - armed.at >= CONFIRM_MIN_GAP_MS) {
      setArmed(null);
      setErrorLine(null);
      try {
        await settingsApi.saveProviders(
          vaultId,
          providers.filter((_, i) => i !== index),
        );
        await reload();
      } catch (err) {
        setErrorLine(errorText(err));
      }
      return;
    }
    setArmed({ key: provider.name, at: now });
    window.setTimeout(() => setArmed((current) => (current?.key === provider.name ? null : current)), 4000);
  };

  // 更多菜单：移到首位立即保存（旧版同），删除仍走 600ms 两步确认。
  const moveProvider = async (index: number): Promise<void> => {
    if (index <= 0) return;
    setMenu(null);
    setErrorLine(null);
    try {
      await settingsApi.saveProviders(vaultId, moveToFront(providers, index));
      await reload();
    } catch (err) {
      setErrorLine(errorText(err));
    }
  };

  const saveKey = async (): Promise<void> => {
    if (!keyEditor) return;
    setKeyBusy(true);
    setErrorLine(null);
    try {
      const result = await settingsApi.setKey(keyEditor.envName, keyValue);
      if (result.failure) throw new Error(result.failure.message);
      setKeyEditor(null);
      setKeyValue("");
      await reload();
    } catch (err) {
      setErrorLine(errorText(err));
    } finally {
      setKeyBusy(false);
    }
  };

  return (
    <div className="settings-content" data-testid="model-service-section">
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">模型服务</h1>
          <p className="settings-subtitle">连接一个 OpenAI 兼容的服务，Agent 才能对话。</p>
          <div className="settings-chips">
            <span className="settings-chip">
              <Lock size={11} aria-hidden />
              API Key 加密存在这台电脑
            </span>
            <span className="settings-chip">
              <PlugsConnected size={11} aria-hidden />
              服务地址与模型随知识库
            </span>
          </div>
        </div>
        <button type="button" className="icon-button" aria-label="关闭设置" onClick={onClose}>
          <X size={16} aria-hidden />
        </button>
      </header>

      <div className="settings-body scroll-y">
        {error ? (
          <div className="status-card" data-tone="fault" role="alert">
            <div className="status-card-head">无法读取模型服务</div>
            <div className="status-card-body">{error}</div>
          </div>
        ) : null}

        {!error && providers.length === 0 && !form ? (
          <div className="settings-empty" data-testid="providers-empty">
            <span className="settings-empty-icon" aria-hidden>
              <Sparkle size={18} />
            </span>
            <h2 className="settings-empty-title">还没有连接模型服务</h2>
            <p className="settings-empty-text">
              连上一个服务后 Agent 才能对话，编辑与检索不受影响。推荐用 DeepSeek 预设：地址和模型已经填好，只差一个 API Key。
            </p>
            <div className="settings-empty-actions">
              <button
                type="button"
                className="btn-primary"
                onClick={() => setForm({ initial: DEEPSEEK_PRESET, index: null })}
                data-testid="add-deepseek-preset"
              >
                用 DeepSeek 预设
              </button>
              <button
                type="button"
                className="btn-quiet"
                onClick={() => setForm({ initial: null, index: null })}
                data-testid="add-custom"
              >
                连接其他服务
              </button>
            </div>
          </div>
        ) : null}

        {!error && providers.length > 0 ? (
          <div className="settings-providers" data-testid="providers-list">
            <div className="settings-providers-head">
              <div>
                <h2 className="settings-section-title">已连接的服务</h2>
                <p className="field-hint">对话默认用第一个可用服务的默认模型，可在 Agents 里按 Agent 单独指定。</p>
              </div>
              <div className="settings-providers-actions">
                <button
                  type="button"
                  className="btn"
                  onClick={() => setForm({ initial: DEEPSEEK_PRESET, index: null })}
                >
                  添加 DeepSeek
                </button>
                <button
                  type="button"
                  className="btn line"
                  onClick={() => setForm({ initial: null, index: null })}
                  data-testid="add-provider"
                >
                  ＋ 添加服务
                </button>
              </div>
            </div>

            {providers.map((provider, index) => {
              const badge = keyBadge(provider, credentials);
              const testText = testResults[provider.name];
              const deleting = armed?.key === provider.name;
              return (
                <div className="provider-row" key={`${provider.name}:${index}`} data-testid="provider-row">
                  <span className="provider-avatar" aria-hidden>
                    {providerInitials(provider.name)}
                  </span>
                  <div className="provider-main">
                    <div className="provider-name-line">
                      <span className="provider-name">{provider.name}</span>
                      <span className="pill" data-tone={badge.tone} data-testid="provider-key-badge">
                        {badge.label}
                      </span>
                      <span className="provider-kind">{provider.kind}</span>
                    </div>
                    <div className="provider-url">{providerAddress(provider.baseUrl)}</div>
                    {provider.defaultModel ? (
                      <div className="provider-default">
                        <Star size={11} weight="fill" aria-hidden />
                        {provider.defaultModel}
                      </div>
                    ) : null}
                    {testText ? (
                      <div className="provider-test" data-testid="provider-test-result">
                        {testText}
                      </div>
                    ) : null}
                    {keyEditor?.name === provider.name && provider.apiKeyEnv ? (
                      <div className="provider-key-editor" data-testid="provider-key-editor">
                        <input
                          type="password"
                          className="field"
                          value={keyValue}
                          placeholder="粘贴新的 API Key"
                          aria-label={`${provider.name} 新的 API Key`}
                          onChange={(event) => setKeyValue(event.target.value)}
                        />
                        <button
                          type="button"
                          className="btn-primary"
                          disabled={keyBusy || keyValue.trim() === ""}
                          onClick={() => void saveKey()}
                        >
                          {keyBusy ? "保存中…" : "保存"}
                        </button>
                        <button
                          type="button"
                          className="btn line"
                          onClick={() => {
                            setKeyEditor(null);
                            setKeyValue("");
                          }}
                        >
                          取消
                        </button>
                      </div>
                    ) : null}
                  </div>
                  <div className="provider-actions">
                    <button
                      type="button"
                      className="btn"
                      disabled={testing !== null}
                      onClick={() => void testProvider(provider)}
                    >
                      {testing === provider.name ? "测试中…" : "测试"}
                    </button>
                    <button
                      type="button"
                      className="btn line"
                      onClick={() => setForm({ initial: provider, index })}
                    >
                      编辑
                    </button>
                    <div className="provider-more" ref={menu === provider.name ? menuRef : undefined}>
                      <button
                        type="button"
                        className="icon-button"
                        aria-label="更多操作"
                        aria-haspopup="menu"
                        aria-expanded={menu === provider.name}
                        onClick={(event) => {
                          event.stopPropagation();
                          setMenu(menu === provider.name ? null : provider.name);
                        }}
                        data-testid="provider-more"
                      >
                        <DotsThree size={16} aria-hidden />
                      </button>
                      {menu === provider.name ? (
                        <div className="provider-menu" role="menu" data-testid="provider-menu">
                          <button
                            type="button"
                            role="menuitem"
                            className="provider-menu-item"
                            disabled={!provider.apiKeyEnv}
                            title={provider.apiKeyEnv ? undefined : "该服务不需要 API key"}
                            onClick={() => {
                              setKeyEditor({ name: provider.name, envName: provider.apiKeyEnv });
                              setKeyValue("");
                              setMenu(null);
                            }}
                          >
                            更换 API Key
                          </button>
                          <button
                            type="button"
                            role="menuitem"
                            className="provider-menu-item"
                            disabled={index === 0}
                            onClick={() => void moveProvider(index)}
                          >
                            移到首位
                          </button>
                          <button
                            type="button"
                            role="menuitem"
                            className="provider-menu-item"
                            onClick={() => void deleteProvider(provider, index)}
                            data-testid="provider-delete"
                          >
                            {deleting ? "确认删除" : "删除"}
                          </button>
                        </div>
                      ) : null}
                    </div>
                  </div>
                </div>
              );
            })}
            {errorLine ? (
              <p className="settings-error" role="alert">
                {errorLine}
              </p>
            ) : null}
          </div>
        ) : null}

        {form ? (
          <ProviderForm
            vaultId={vaultId}
            initial={form.initial}
            index={form.index}
            savedKey={
              form.initial !== null &&
              credentials.some((item) => item.envName === form.initial?.apiKeyEnv && item.isSet)
            }
            onCancel={() => setForm(null)}
            onSaved={async () => {
              setForm(null);
              await reload();
            }}
          />
        ) : null}

        <details
          className="settings-key-info"
          open={showKeyInfo}
          onToggle={(event) => setShowKeyInfo(event.currentTarget.open)}
        >
          <summary>
            <CaretRight size={11} weight="bold" aria-hidden className="caret" />
            API Key 存在哪里，谁能看到
          </summary>
          <p className="field-hint">
            Key 只写入这台电脑的 Windows 凭据管理器，由应用后端读取；知识库里只记录环境变量名，界面上永远不回显密钥本身。
          </p>
        </details>
      </div>
    </div>
  );
}
