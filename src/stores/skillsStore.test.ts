import { beforeEach, describe, expect, it, vi } from "vitest";

const registry = vi.fn();
const scan = vi.fn();

vi.mock("../lib/transport", () => {
  class TransportError extends Error {
    readonly code: string;
    constructor(code: string, message: string) {
      super(message);
      this.code = code;
    }
  }
  return {
    TransportError,
    skillsApi: { registry: (...args: unknown[]) => registry(...args), scan: (...args: unknown[]) => scan(...args) },
    settingsApi: { patch: vi.fn(), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { useSkillsStore } = await import("./skillsStore");

const tool = {
  id: "claude-code",
  name: "Claude Code",
  iconKey: "claude-code",
  installDir: "<home>/.claude/skills",
  scanDirs: [],
  readers: [],
  status: "detected",
  official: true,
  enabled: true,
};

const sampleScan = {
  cachedAt: "2026-11-20T09:00:00Z",
  scanning: false,
  kb: [],
  tools: [{ toolId: "claude-code", skills: [] }],
};

beforeEach(() => {
  registry.mockReset();
  scan.mockReset();
  useSkillsStore.setState({ tools: [], scan: null, loading: false, error: null });
});

describe("skillsStore", () => {
  it("loads the registry into the store", async () => {
    registry.mockResolvedValue({ tools: [tool] });
    await useSkillsStore.getState().loadRegistry();
    expect(useSkillsStore.getState().tools).toEqual([tool]);
  });

  it("passes the active vault id to the scan", async () => {
    scan.mockResolvedValue({ scan: sampleScan });
    await useSkillsStore.getState().loadScan("vault-1");
    expect(scan).toHaveBeenCalledWith("vault-1");
    expect(useSkillsStore.getState().scan?.cachedAt).toBe("2026-11-20T09:00:00Z");
  });

  it("keeps the last good scan when a request fails", async () => {
    useSkillsStore.setState({ scan: sampleScan });
    const { TransportError } = await import("../lib/transport");
    scan.mockRejectedValue(new TransportError("io-failure", "扫描失败"));
    await useSkillsStore.getState().loadScan("vault-1");
    expect(useSkillsStore.getState().error).toBe("扫描失败");
    expect(useSkillsStore.getState().scan).toBe(sampleScan);
  });

  it("refresh fetches both and clears loading", async () => {
    registry.mockResolvedValue({ tools: [tool] });
    scan.mockResolvedValue({ scan: { ...sampleScan, scanning: true } });
    await useSkillsStore.getState().refresh("vault-1");
    const state = useSkillsStore.getState();
    expect(state.loading).toBe(false);
    expect(state.tools).toHaveLength(1);
    expect(state.scan?.scanning).toBe(true);
  });

  it("polls a background scan until it settles", async () => {
    vi.useFakeTimers();
    try {
      scan.mockResolvedValueOnce({ scan: { ...sampleScan, scanning: true } });
      scan.mockResolvedValueOnce({ scan: { ...sampleScan, scanning: false, cachedAt: "later" } });
      await useSkillsStore.getState().loadScan("vault-1");
      expect(useSkillsStore.getState().scan?.scanning).toBe(true);
      await vi.advanceTimersByTimeAsync(1300);
      expect(scan).toHaveBeenCalledTimes(2);
      expect(useSkillsStore.getState().scan?.scanning).toBe(false);
      expect(useSkillsStore.getState().scan?.cachedAt).toBe("later");
    } finally {
      vi.useRealTimers();
    }
  });

  it("drops a stale scan response", async () => {
    let resolveFirst: (value: unknown) => void = () => {};
    const first = new Promise((resolve) => {
      resolveFirst = resolve;
    });
    scan.mockReturnValueOnce(first);
    scan.mockResolvedValueOnce({ scan: { ...sampleScan, cachedAt: "newer" } });
    const older = useSkillsStore.getState().loadScan("vault-1");
    await useSkillsStore.getState().loadScan("vault-1");
    resolveFirst({ scan: { ...sampleScan, cachedAt: "older" } });
    await older;
    expect(useSkillsStore.getState().scan?.cachedAt).toBe("newer");
  });
});


describe("skillsStore scan merge (M10.1 P03)", () => {
  function scanOf(skills: Array<{ name: string; path: string }>, cachedAt: string, scanning = false) {
    return {
      cachedAt,
      scanning,
      kb: [{ category: "cat", skills: skills.map((s) => ({ name: s.name, description: "d", path: s.path, category: "cat", distributed: [] })) }],
      tools: [],
    };
  }

  it("keeps the scan object when the payload is unchanged apart from cachedAt", async () => {
    const first = scanOf([{ name: "a", path: "skills/cat/a" }], "t0");
    scan.mockResolvedValueOnce({ scan: first });
    await useSkillsStore.getState().loadScan("v1");
    const kept = useSkillsStore.getState().scan;
    const copy = scanOf([{ name: "a", path: "skills/cat/a" }], "t1");
    scan.mockResolvedValueOnce({ scan: copy });
    await useSkillsStore.getState().loadScan("v1");
    expect(useSkillsStore.getState().scan).toBe(kept);
  });

  it("reuses unchanged skill references when one entry changes", async () => {
    const a = { name: "a", path: "skills/cat/a" };
    const first = scanOf([a, { name: "b", path: "skills/cat/b" }], "t0");
    scan.mockResolvedValueOnce({ scan: first });
    await useSkillsStore.getState().loadScan("v1");
    const firstScan = useSkillsStore.getState().scan;
    const beforeA = (useSkillsStore.getState().scan!.kb[0] as { skills: { name: string; path: string }[] }).skills[0];
    const copy = scanOf([{ name: "a", path: "skills/cat/a" }, { name: "b", path: "skills/cat/b" }], "t1");
    (copy.kb[0]!.skills[1] as { distributed: unknown[] }).distributed = [{ toolId: "x", mode: "link" }];
    scan.mockResolvedValueOnce({ scan: copy });
    await useSkillsStore.getState().loadScan("v1");
    const next = useSkillsStore.getState().scan!;
    expect(next).not.toBe(firstScan);
    expect((next.kb[0] as { skills: unknown[] }).skills[0]).toBe(beforeA);
  });
});
