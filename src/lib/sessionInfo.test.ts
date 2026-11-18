import { describe, expect, it } from "vitest";
import type { ProviderDraft, ProvidersResult, TaskItem } from "../mygo";
import {
  credentialState,
  findProviderForModel,
  openTasks,
  permissionBoundary,
  providerHost,
} from "./sessionInfo";

function provider(partial: Partial<ProviderDraft> = {}): ProviderDraft {
  return {
    name: "fake",
    kind: "openai",
    baseUrl: "https://api.example.com/v1",
    models: ["m1", "m2"],
    defaultModel: "m1",
    apiKeyEnv: "FAKE_KEY",
    ...partial,
  };
}

function task(id: string, status: string): TaskItem {
  return {
    id,
    owner: "curator",
    status,
    goal: "目标",
    inputs: [],
    constraints: [],
    outputs: [],
    createdBy: "user",
    createdAt: "t",
    updatedAt: "t",
  };
}

describe("provider diagnostics", () => {
  it("finds the provider listing the current model", () => {
    const providers = [provider(), provider({ name: "other", models: ["m9"] })];
    expect(findProviderForModel(providers, "m2")?.name).toBe("fake");
    expect(findProviderForModel(providers, "m9")?.name).toBe("other");
    expect(findProviderForModel(providers, "missing")).toBeNull();
    expect(findProviderForModel(providers, "")).toBeNull();
  });

  it("credential state follows the env name and the saved credentials", () => {
    const result = {
      providers: [provider()],
      credentials: [{ envName: "FAKE_KEY", isSet: true }],
    } as ProvidersResult;
    expect(credentialState(result, provider())).toBe("saved");
    expect(
      credentialState({ providers: [], credentials: [] } as ProvidersResult, provider()),
    ).toBe("missing");
    expect(credentialState(result, provider({ apiKeyEnv: "" }))).toBe("none");
    expect(credentialState(null, null)).toBe("none");
  });

  it("hosts are shown without the scheme", () => {
    expect(providerHost("https://api.example.com/v1")).toBe("api.example.com");
    expect(providerHost("not a url")).toBe("not a url");
  });
});

describe("shared context panel", () => {
  it("only unfinished tasks are listed", () => {
    const tasks = [task("a", "queued"), task("b", "done"), task("c", "blocked"), task("d", "cancelled")];
    expect(openTasks(tasks).map((item) => item.id)).toEqual(["a", "c"]);
  });

  it("the permission boundary names the system areas", () => {
    const lines = permissionBoundary();
    expect(lines).toHaveLength(3);
    expect(lines[1]).toContain("禁写");
    expect(lines[2]).toContain("永久删除");
  });
});
