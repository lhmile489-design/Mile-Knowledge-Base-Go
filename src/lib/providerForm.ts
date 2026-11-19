// Provider editor helpers (T17). Pure: parsing, initials, key labels and the
// list upsert that M68 receives. Keys are never part of a ProviderDraft.
import type { ProviderCredentialState, ProviderDraft } from "../mygo";

/** Mirrors the Go DeepSeekPreset (providers.go:494-501). */
export const DEEPSEEK_PRESET: ProviderDraft = {
  name: "deepseek",
  kind: "openai",
  baseUrl: "https://api.deepseek.com",
  models: ["deepseek-v4-flash", "deepseek-v4-pro"],
  defaultModel: "deepseek-v4-flash",
  apiKeyEnv: "DEEPSEEK_API_KEY",
  contextWindow: 1_000_000,
};

/** One model per line; trims, drops empties and duplicates, keeps order. */
export function parseModelLines(text: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of text.split("\n")) {
    const model = raw.trim();
    if (model === "" || seen.has(model)) continue;
    seen.add(model);
    out.push(model);
  }
  return out;
}

/** First two characters as the row avatar; empty names stay empty. */
export function providerInitials(name: string): string {
  const chars = [...name.trim()];
  return chars.slice(0, 2).join("").toUpperCase();
}

export type KeyBadge = { readonly label: string; readonly tone: "live" | "warn" | "idle" };

/** Row badge: providers without an env name need no key at all. */
export function keyBadge(
  provider: ProviderDraft,
  credentials: readonly ProviderCredentialState[],
): KeyBadge {
  if (provider.apiKeyEnv === "") return { label: "无需密钥", tone: "idle" };
  const saved = credentials.find((item) => item.envName === provider.apiKeyEnv)?.isSet === true;
  return saved ? { label: "密钥已保存", tone: "live" } : { label: "密钥未保存", tone: "warn" };
}

/** Replace one entry (by index) or append a new provider. */
export function upsertProvider(
  list: readonly ProviderDraft[],
  draft: ProviderDraft,
  index: number | null,
): ProviderDraft[] {
  if (index === null || index < 0 || index >= list.length) return [...list, draft];
  return list.map((item, i) => (i === index ? draft : item));
}

/** The provider's host + path without the scheme, for the row subtitle. */
export function providerAddress(baseUrl: string): string {
  return baseUrl.replace(/^https?:\/\//i, "").replace(/\/+$/, "");
}

/** Move one provider to the front (the more menu's "移到首位"). */
export function moveToFront<T>(list: readonly T[], index: number): T[] {
  if (index <= 0 || index >= list.length) return [...list];
  return [list[index] as T, ...list.slice(0, index), ...list.slice(index + 1)];
}
