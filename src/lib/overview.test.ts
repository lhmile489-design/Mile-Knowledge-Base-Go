import { describe, expect, it } from "vitest";
import type { KbHealthReport } from "../mygo";
import { emptyBucket } from "./chatStream";
import {
  agentStatus,
  draftsCount,
  healthCounts,
  healthSummary,
  indexDetail,
  indexLabel,
  pendingCount,
  relativeTime,
} from "./overview";

const NOW = new Date("2026-11-19T12:00:00Z");

describe("index tile", () => {
  it("maps phases to the legacy labels", () => {
    expect(indexLabel("idle")).toBe("尚未建立");
    expect(indexLabel("scanning")).toBe("正在建立");
    expect(indexLabel("ready")).toBe("已就绪");
    expect(indexLabel("error")).toBe("建立失败");
    expect(indexLabel("bogus")).toBe("尚未建立");
  });

  it("shows progress while scanning and age when ready", () => {
    expect(indexDetail({ phase: "scanning", scanned: 12 } as never, NOW)).toBe("· 已处理 12");
    expect(
      indexDetail({ phase: "ready", lastBuiltAt: "2026-11-19T11:58:00Z" } as never, NOW),
    ).toBe("· 2 分钟前");
    expect(indexDetail(null, NOW)).toBe("");
  });

  it("formats relative time across boundaries", () => {
    expect(relativeTime("2026-11-19T11:59:40Z", NOW)).toBe("刚刚");
    expect(relativeTime("2026-11-19T11:00:30Z", NOW)).toBe("59 分钟前");
    expect(relativeTime("2026-11-19T09:30:00Z", NOW)).toBe("2 小时前");
    expect(relativeTime("2026-11-17T12:00:00Z", NOW)).toBe("2 天前");
    expect(relativeTime("", NOW)).toBe("");
    expect(relativeTime("not-a-date", NOW)).toBe("");
  });
});

describe("health tile", () => {
  const report = {
    unresolvedLinks: [{}, {}],
    orphans: [{}],
    duplicates: [],
    untitled: [{}],
  } as unknown as KbHealthReport;

  it("counts the four lists", () => {
    expect(healthCounts(report)).toEqual({ unresolved: 2, orphans: 1, duplicates: 0, untitled: 1, total: 4 });
    expect(healthCounts(null).total).toBe(0);
  });

  it("uses the legacy sentences", () => {
    expect(healthSummary(report, true)).toBe("断链 2 · 孤立 1 · 重名 0 · 缺标题 1");
    expect(healthSummary(report, false)).toBe("索引未就绪 · 断链 2 · 孤立 1 · 重名 0 · 缺标题 1");
    expect(healthSummary({} as KbHealthReport, true)).toBe("断链、孤立、重名、缺标题都是 0");
    expect(healthSummary(null, true)).toBe("正在读取");
  });
});

describe("drafts, pending and agent status", () => {
  it("counts files only and treats a missing directory as zero", () => {
    expect(
      draftsCount([
        { kind: "file" } as never,
        { kind: "directory" } as never,
        { kind: "file" } as never,
      ]),
    ).toBe(2);
    expect(draftsCount(null)).toBe(0);
  });

  it("sums pending approvals across agents", () => {
    const a = emptyBucket();
    const b = { ...emptyBucket(), pendingPermissions: [{ requestId: "x" }] } as never;
    expect(pendingCount({ curator: a, scout: b, distiller: a, general: a })).toBe(1);
  });

  it("derives one agent's status word", () => {
    expect(agentStatus(emptyBucket(), "offline")).toBe("不可用");
    expect(agentStatus(emptyBucket(), "starting")).toBe("启动中");
    expect(agentStatus(emptyBucket(), "ready")).toBe("未启动");
    expect(agentStatus({ ...emptyBucket(), busy: true } as never, "ready")).toBe("正在工作");
    expect(
      agentStatus({ ...emptyBucket(), pendingPermissions: [{ requestId: "x" }] } as never, "ready"),
    ).toBe("等待你确认");
    const used = { ...emptyBucket(), transcript: { lines: [{}] } } as never;
    expect(agentStatus(used, "ready")).toBe("就绪");
  });
});
