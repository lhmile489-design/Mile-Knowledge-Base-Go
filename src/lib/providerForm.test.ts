import { describe, expect, it } from "vitest";
import type { ProviderDraft } from "../mygo";
import {
  DEEPSEEK_PRESET,
  keyBadge,
  parseModelLines,
  providerAddress,
  providerInitials,
  upsertProvider,
} from "./providerForm";

function provider(partial: Partial<ProviderDraft> = {}): ProviderDraft {
  return {
    name: "fake",
    kind: "openai",
    baseUrl: "https://api.example.com",
    models: ["m1"],
    defaultModel: "m1",
    apiKeyEnv: "FAKE_KEY",
    ...partial,
  };
}

describe("provider form helpers", () => {
  it("parses one model per line, dropping blanks and duplicates", () => {
    expect(parseModelLines(" a\n\nb\r\n a \n")).toEqual(["a", "b"]);
    expect(parseModelLines("")).toEqual([]);
  });

  it("initials take the first two characters", () => {
    expect(providerInitials("deepseek")).toBe("DE");
    expect(providerInitials("工具")).toBe("工具");
    expect(providerInitials("  ")).toBe("");
  });

  it("key badges distinguish no-key, saved and missing", () => {
    expect(keyBadge(provider({ apiKeyEnv: "" }), [])).toEqual({ label: "无需密钥", tone: "idle" });
    expect(keyBadge(provider(), [{ envName: "FAKE_KEY", isSet: true }])).toEqual({
      label: "密钥已保存",
      tone: "live",
    });
    expect(keyBadge(provider(), [])).toEqual({ label: "密钥未保存", tone: "warn" });
  });

  it("upsert replaces by index or appends", () => {
    const list = [provider({ name: "a" }), provider({ name: "b" })];
    const replaced = upsertProvider(list, provider({ name: "B2" }), 1);
    expect(replaced.map((item) => item.name)).toEqual(["a", "B2"]);
    expect(upsertProvider(list, provider({ name: "c" }), null)).toHaveLength(3);
    expect(upsertProvider(list, provider({ name: "x" }), 9)).toHaveLength(3);
  });

  it("the DeepSeek preset matches the Go preset", () => {
    expect(DEEPSEEK_PRESET.name).toBe("deepseek");
    expect(DEEPSEEK_PRESET.apiKeyEnv).toBe("DEEPSEEK_API_KEY");
    expect(DEEPSEEK_PRESET.models).toEqual(["deepseek-v4-flash", "deepseek-v4-pro"]);
    expect(providerAddress("https://api.deepseek.com/")).toBe("api.deepseek.com");
  });
});
