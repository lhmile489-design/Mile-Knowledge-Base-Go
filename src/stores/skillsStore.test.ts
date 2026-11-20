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
