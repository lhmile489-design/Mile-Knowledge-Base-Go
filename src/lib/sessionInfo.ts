// Session-info helpers (T16): provider diagnostics, open tasks and the
// permission boundary copy. Pure — the popover renders whatever these say.
import type { ProviderDraft, ProvidersResult, TaskItem } from "../mygo";

export type CredentialState = "none" | "saved" | "missing";

/** The provider that lists the current model, if any. */
export function findProviderForModel(
  providers: readonly ProviderDraft[],
  model: string,
): ProviderDraft | null {
  if (model === "") return null;
  return providers.find((provider) => provider.models.includes(model)) ?? null;
}

/** Whether the key the provider needs is present; an empty env name needs no key. */
export function credentialState(
  result: ProvidersResult | null,
  provider: ProviderDraft | null,
): CredentialState {
  if (!provider || provider.apiKeyEnv === "") return "none";
  const entry = result?.credentials.find((item) => item.envName === provider.apiKeyEnv);
  return entry?.isSet ? "saved" : "missing";
}

const OPEN_STATUSES = new Set(["queued", "running", "blocked"]);

/** Only unfinished tasks occupy the context panel. */
export function openTasks(tasks: readonly TaskItem[]): TaskItem[] {
  return tasks.filter((task) => OPEN_STATUSES.has(task.status));
}

/** The uniform write boundary every agent shares (M2 policy). */
export function permissionBoundary(): readonly string[] {
  return [
    "可读：知识库全文；可写：内容区（skills / raws / archives / projects / tools / 其他目录）",
    "禁写：应用配置、事务恢复记录、索引、来源关系与所有点号系统目录",
    "永久删除需要单独确认；全自动模式也不例外",
  ];
}

export function providerHost(baseUrl: string): string {
  try {
    return new URL(baseUrl).host;
  } catch {
    return baseUrl;
  }
}
