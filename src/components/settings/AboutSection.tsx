import { useEffect, useState } from "react";
import { Check, Copy, Lock, X } from "@phosphor-icons/react";
import type { AboutInfoResult } from "../../mygo";
import { settingsApi, TransportError, windowApi } from "../../lib/transport";
import { EngineBadge } from "../chat/EngineBadge";
import { SettingGroup, SettingRow } from "./SettingRow";

const REPO_URL = "https://github.com/lhmile489-design/Mile-Knowledge-Base-Go";

/** Settings → 关于与诊断 (T07). A troubleshooting view: version, engine state,
 * fixed capabilities and where things live. Paths are redacted; copying a real
 * path is done by Go (M75), never by page code. */
export function AboutSection({ onClose }: { readonly onClose: () => void }) {
  const [about, setAbout] = useState<AboutInfoResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  useEffect(() => {
    settingsApi
      .aboutInfo()
      .then((result) => {
        setAbout(result);
        setError(null);
      })
      .catch((err) => setError(err instanceof TransportError ? err.message : String(err)));
  }, []);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(null), 1500);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const copy = (kind: string): void => {
    void settingsApi
      .copyStoragePath(kind)
      .then(() => setCopied(kind))
      .catch((err) => setError(err instanceof TransportError ? err.message : String(err)));
  };

  const engine = about?.engine;
  const capabilities = about?.capabilities;

  return (
    <div className="settings-content" data-testid="settings-about">
      <header className="settings-content-head">
        <div>
          <h1 className="settings-title">关于与诊断</h1>
          <p className="settings-subtitle">排障用的只读信息：版本、引擎状态与数据位置。</p>
          <div className="settings-chips">
            <span className="settings-chip">只读</span>
          </div>
        </div>
        <button type="button" className="icon-button" aria-label="关闭设置" onClick={onClose}>
          <X size={16} aria-hidden />
        </button>
      </header>

      <div className="settings-body scroll-y">
        <div className="settings-stack">
          {error ? (
            <p className="settings-error" role="alert">
              {error}
            </p>
          ) : null}

          <SettingGroup title="应用信息">
            <SettingRow
              label="版本"
              control={
                <span className="settings-widths" data-testid="about-version">
                  v{about?.version ?? "—"}
                </span>
              }
            />
            <SettingRow
              label="技术栈"
              control={<span className="settings-widths">{about?.techStack ?? "—"}</span>}
            />
            <SettingRow
              label="仓库"
              control={
                <button
                  type="button"
                  className="btn line"
                  onClick={() => void windowApi.openExternal(REPO_URL)}
                  data-testid="about-repo"
                >
                  GitHub
                </button>
              }
            />
          </SettingGroup>

          <SettingGroup title="快速上手">
            <div className="setting-row" data-stacked="true">
              <ol className="settings-steps">
                <li>点击侧栏的知识库按钮，选择一个本地 Markdown 目录。</li>
                <li>文档有三种模式：阅读（默认）、源码、分栏，在文档工具栏切换。</li>
                <li>在「模型服务」里连上一个服务，就能在对话里直接说要做什么，系统会挑合适的 Agent。</li>
                <li>Agent 的每次写操作都弹审批并记在操作日志里，可从日志恢复。</li>
              </ol>
              <p className="field-hint">还没配置服务时编辑器与检索不受影响，AI 对话不可用。</p>
            </div>
          </SettingGroup>

          <SettingGroup title="对话引擎">
            <SettingRow
              label="状态"
              control={
                <span className="settings-engine" data-testid="about-engine">
                  <EngineBadge status={engine} />
                  {about && about.restartCount > 0 ? (
                    <span className="settings-widths">已重启 {about.restartCount} 次</span>
                  ) : null}
                </span>
              }
            />
            {engine && engine.phase !== "ready" ? (
              <div className="setting-row" data-stacked="true">
                <p className="field-hint">
                  引擎不可用时对话不可用，编辑器与检索不受影响。一直离线时检查「模型服务」里的服务与 API key。
                </p>
              </div>
            ) : null}
            {capabilities ? (
              <div className="setting-row" data-stacked="true">
                <details className="svc-details" data-testid="about-capabilities">
                  <summary>
                    <Lock size={12} aria-hidden />
                    引擎能力（固定集合，排障用）
                  </summary>
                  <div className="svc-disclose-body">
                    <SettingRow label="图片输入" control={<span>{capabilities.promptImage ? "支持" : "不支持"}</span>} />
                    <SettingRow label="音频输入" control={<span>{capabilities.promptAudio ? "支持" : "不支持"}</span>} />
                    <SettingRow
                      label="内联上下文"
                      control={<span>{capabilities.promptEmbeddedContext ? "支持" : "不支持"}</span>}
                    />
                    <SettingRow label="扩展面" control={<span>{capabilities.extensionSurface ? "支持" : "不支持"}</span>} />
                    <SettingRow label="切换方法" control={<span>{capabilities.steerMethod ?? "未提供"}</span>} />
                  </div>
                </details>
              </div>
            ) : null}
          </SettingGroup>

          <SettingGroup
            title="存储位置"
            footnote="复制给出的是本机真实路径；凭据由 Windows 凭据管理器保存，没有路径。"
          >
            {(about?.storage ?? []).map((item) => (
              <SettingRow
                key={item.kind}
                label={item.label}
                testid={`about-storage-${item.kind}`}
                control={
                  <span className="settings-storage">
                    <span className="settings-widths settings-path" data-testid={`about-path-${item.kind}`}>
                      {item.display}
                    </span>
                    {item.copyable ? (
                      <button
                        type="button"
                        className="icon-button"
                        aria-label={copied === item.kind ? `已复制${item.label}` : `复制${item.label}`}
                        title={copied === item.kind ? "已复制" : `复制${item.label}`}
                        onClick={() => copy(item.kind)}
                        data-testid={`about-copy-${item.kind}`}
                      >
                        {copied === item.kind ? (
                          <Check size={13} aria-hidden />
                        ) : (
                          <Copy size={13} aria-hidden />
                        )}
                      </button>
                    ) : null}
                  </span>
                }
              />
            ))}
          </SettingGroup>
        </div>
      </div>
    </div>
  );
}
