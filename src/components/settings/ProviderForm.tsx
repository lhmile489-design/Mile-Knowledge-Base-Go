import { useState } from "react";
import type { ProviderDraft } from "../../mygo";
import { parseModelLines } from "../../lib/providerForm";
import { settingsApi, TransportError } from "../../lib/transport";

export interface ProviderFormProps {
  readonly vaultId: string;
  readonly initial: ProviderDraft | null;
  /** Index in the saved list; null when adding. */
  readonly index: number | null;
  readonly savedKey: boolean;
  readonly onCancel: () => void;
  readonly onSaved: () => Promise<void>;
}

function errorText(error: unknown): string {
  if (error instanceof TransportError) return error.message;
  if (error instanceof Error) return error.message;
  return String(error);
}

/** Add/edit one provider (T17). The key value, when typed, is sent once via
 * M69 and never displayed again; leaving it empty keeps the saved key. */
export function ProviderForm({ vaultId, initial, index, savedKey, onCancel, onSaved }: ProviderFormProps) {
  const [name, setName] = useState(initial?.name ?? "");
  const [kind, setKind] = useState(initial?.kind ?? "openai");
  const [baseUrl, setBaseUrl] = useState(initial?.baseUrl ?? "");
  const [keyEnv, setKeyEnv] = useState(initial?.apiKeyEnv ?? "");
  const [keyValue, setKeyValue] = useState("");
  const [modelsText, setModelsText] = useState((initial?.models ?? []).join("\n"));
  const [defaultModel, setDefaultModel] = useState(initial?.defaultModel ?? "");
  const [contextWindow, setContextWindow] = useState(
    initial?.contextWindow ? String(initial.contextWindow) : "",
  );
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const models = parseModelLines(modelsText);

  const fetchModels = async (): Promise<void> => {
    if (busy) return;
    setBusy(true);
    setError(null);
    setNote(null);
    try {
      const result = await settingsApi.fetchModels(kind, baseUrl, keyEnv, keyValue);
      if (result.failure) {
        setError(result.failure.message);
      } else {
        const fetched = result.models ?? [];
        setModelsText(fetched.join("\n"));
        if (!fetched.includes(defaultModel)) setDefaultModel(fetched[0] ?? "");
        setNote(`已获取 ${fetched.length} 个模型`);
      }
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  const save = async (): Promise<void> => {
    if (busy) return;
    setError(null);
    setNote(null);
    if (models.length === 0) {
      setError("模型列表不能为空：先填写或拉取模型");
      return;
    }
    const context = contextWindow.trim() === "" ? undefined : Number(contextWindow);
    if (context !== undefined && (!Number.isFinite(context) || context <= 0)) {
      setError("上下文窗口必须是正整数");
      return;
    }
    const draft: ProviderDraft = {
      name: name.trim(),
      kind,
      baseUrl: baseUrl.trim(),
      models,
      defaultModel: models.includes(defaultModel) ? defaultModel : (models[0] ?? ""),
      apiKeyEnv: keyEnv.trim(),
      ...(context !== undefined ? { contextWindow: Math.trunc(context) } : {}),
    };
    setBusy(true);
    try {
      const current = await settingsApi.providers(vaultId);
      const next = index === null ? [...current.providers, draft] : current.providers.map((item, i) => (i === index ? draft : item));
      await settingsApi.saveProviders(vaultId, next);
      if (keyValue.trim() !== "") {
        if (draft.apiKeyEnv === "") {
          setNote("服务已保存；没有填写 key 环境变量名，密钥未保存");
        } else {
          const keyed = await settingsApi.setKey(draft.apiKeyEnv, keyValue.trim());
          if (keyed.failure) setNote(`服务已保存，但密钥保存失败：${keyed.failure.message}`);
        }
      }
      await onSaved();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="settings-form" data-testid="provider-form">
      <div className="field-row">
        <span className="field-label">名称</span>
        <input
          className="field"
          value={name}
          placeholder="例如 deepseek"
          aria-label="服务名称"
          onChange={(event) => setName(event.target.value)}
        />
      </div>

      <div className="field-row">
        <span className="field-label">类型</span>
        <div className="segmented settings-kind" role="radiogroup" aria-label="服务类型">
          {["openai", "anthropic"].map((value) => (
            <button
              key={value}
              type="button"
              className="segmented-item"
              role="radio"
              aria-checked={kind === value}
              onClick={() => setKind(value)}
            >
              {value === "openai" ? "OpenAI 兼容" : "Anthropic（只保存不调用）"}
            </button>
          ))}
        </div>
      </div>

      <div className="field-row">
        <span className="field-label">Base URL</span>
        <input
          className="field field-mono"
          value={baseUrl}
          placeholder="https://api.example.com"
          aria-label="Base URL"
          onChange={(event) => setBaseUrl(event.target.value)}
        />
      </div>

      <div className="field-row">
        <span className="field-label">API Key 环境变量名</span>
        <input
          className="field field-mono"
          value={keyEnv}
          placeholder="例如 DEEPSEEK_API_KEY；留空表示不需要鉴权"
          aria-label="API Key 环境变量名"
          onChange={(event) => setKeyEnv(event.target.value)}
        />
      </div>

      <div className="field-row">
        <span className="field-label">API Key</span>
        <input
          className="field field-mono"
          type="password"
          autoComplete="off"
          value={keyValue}
          placeholder={savedKey ? "已保存；留空表示不修改" : keyEnv === "" ? "当前服务无需密钥" : "粘贴 API Key（只保存一次，不再显示）"}
          aria-label="API Key"
          onChange={(event) => setKeyValue(event.target.value)}
        />
        <p className="field-hint">密钥只写入这台电脑的 Windows 凭据管理器，不写进知识库。</p>
      </div>

      <div className="field-row">
        <span className="field-label">模型列表</span>
        <textarea
          className="field field-mono field-textarea"
          rows={4}
          value={modelsText}
          placeholder="每行一个模型名"
          aria-label="模型列表"
          onChange={(event) => setModelsText(event.target.value)}
        />
        <button type="button" className="btn line settings-fetch" disabled={busy} onClick={() => void fetchModels()}>
          {busy ? "处理中…" : "从服务拉取模型"}
        </button>
      </div>

      <div className="field-row">
        <span className="field-label">默认模型</span>
        <select
          className="field"
          value={models.includes(defaultModel) ? defaultModel : (models[0] ?? "")}
          aria-label="默认模型"
          onChange={(event) => setDefaultModel(event.target.value)}
        >
          {models.length === 0 ? <option value="">（先填写模型）</option> : null}
          {models.map((model) => (
            <option key={model} value={model}>
              {model}
            </option>
          ))}
        </select>
      </div>

      <div className="field-row">
        <span className="field-label">上下文窗口（可选）</span>
        <input
          className="field field-mono"
          value={contextWindow}
          inputMode="numeric"
          placeholder="例如 1000000"
          aria-label="上下文窗口"
          onChange={(event) => setContextWindow(event.target.value)}
        />
      </div>

      {error ? (
        <p className="settings-error" role="alert">
          {error}
        </p>
      ) : null}
      {note && !error ? <p className="field-hint">{note}</p> : null}

      <div className="settings-form-actions">
        <button type="button" className="btn-quiet" disabled={busy} onClick={onCancel}>
          取消
        </button>
        <button type="button" className="btn-primary" disabled={busy} onClick={() => void save()}>
          {busy ? "保存中…" : "保存"}
        </button>
      </div>
    </div>
  );
}
