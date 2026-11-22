import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const restorePreview = vi.fn();
const restoreManifest = vi.fn();
const rebuildManifest = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      restorePreview: (vaultId: string) => restorePreview(vaultId),
      restoreManifest: (vaultId: string, selections: unknown, requestId: string, signal?: AbortSignal) =>
        restoreManifest(vaultId, selections, requestId, signal),
      rebuildManifest: (vaultId: string) => rebuildManifest(vaultId),
    },
  };
});

vi.mock("../../mygo", () => ({
  events: {
    skillStageProgress: { on: vi.fn(() => () => undefined) },
  },
}));

const { RestoreView } = await import("./RestoreView");

const createItem = {
  id: "r-111111111111",
  source: "kb",
  name: "vue",
  skillPath: "skills/前端/vue",
  mode: "link",
  target: { kind: "builtin", toolId: "claude-code" },
  targetLabel: "Claude Code",
  targetId: "claude-code",
  state: "create",
  selectable: true,
  defaultSelected: true,
};

const conflictItem = {
  id: "r-222222222222",
  source: "kb",
  name: "caveman",
  skillPath: "skills/前端/caveman",
  mode: "copy",
  target: { kind: "builtin", toolId: "cursor" },
  targetLabel: "Cursor",
  targetId: "cursor",
  state: "conflict",
  reason: "目标位置已有同名目录",
  selectable: true,
  defaultSelected: false,
};

const githubItem = {
  id: "r-333333333333",
  source: "github",
  name: "pdf",
  origin: { owner: "acme", repo: "skills", ref: "main", path: "skills/pdf", commit: "a".repeat(40) },
  target: { kind: "builtin", toolId: "claude-code" },
  targetLabel: "Claude Code",
  targetId: "claude-code",
  state: "create",
  needsNetwork: true,
  selectable: true,
  defaultSelected: true,
};

const existsItem = {
  id: "r-444444444444",
  source: "kb",
  name: "docx",
  skillPath: "skills/文档/docx",
  mode: "copy",
  target: { kind: "builtin", toolId: "claude-code" },
  targetLabel: "Claude Code",
  state: "exists",
  reason: "已分发",
  selectable: false,
  defaultSelected: false,
};

function previewOf(overrides: Record<string, unknown> = {}) {
  return {
    preview: {
      status: "ok",
      items: [createItem, githubItem, conflictItem, existsItem],
      pending: 3,
      localOnly: [{ name: "old", targetLabel: "Kiro" }],
      unmatched: [],
      warnings: [],
      canRebuild: true,
      ...overrides,
    },
  };
}

function renderView() {
  const onBack = vi.fn();
  const onChanged = vi.fn();
  const onResult = vi.fn();
  render(<RestoreView vaultId="v1" onBack={onBack} onChanged={onChanged} onResult={onResult} />);
  return { onBack, onChanged, onResult };
}

beforeEach(() => {
  restorePreview.mockReset();
  restoreManifest.mockReset();
  rebuildManifest.mockReset();
});

afterEach(cleanup);

describe("RestoreView", () => {
  it("lists the groups, preselects creates and counts the selection", async () => {
    restorePreview.mockResolvedValue(previewOf());
    renderView();

    await screen.findByTestId("restore-group-create");
    expect(screen.getByTestId("restore-group-conflict")).toBeTruthy();
    expect(screen.getByTestId("restore-group-exists")).toBeTruthy();
    expect(screen.getByTestId("restore-count").textContent).toContain("已选 2 项");
    expect(screen.getByTestId("restore-count").textContent).toContain("1 项需要联网");
    expect((screen.getByTestId(`restore-check-${createItem.id}`) as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId(`restore-check-${conflictItem.id}`) as HTMLInputElement).checked).toBe(false);
    expect(screen.queryByTestId(`restore-check-${existsItem.id}`)).toBeNull();
    expect(screen.getByTestId("restore-local")).toBeTruthy();
  });

  it("restores the selected items and reports the result", async () => {
    restorePreview.mockResolvedValue(previewOf());
    restoreManifest.mockResolvedValue({
      items: [
        { id: createItem.id, name: "vue", targetLabel: "Claude Code", result: "linked" },
        { id: githubItem.id, name: "pdf", targetLabel: "Claude Code", result: "installed" },
        { id: conflictItem.id, name: "caveman", targetLabel: "Cursor", result: "copied", backupId: "b-1" },
      ],
    });
    const { onChanged } = renderView();

    await screen.findByTestId("restore-group-create");
    fireEvent.click(screen.getByTestId(`restore-check-${conflictItem.id}`));
    expect(screen.getByTestId("restore-count").textContent).toContain("已选 3 项");
    fireEvent.click(screen.getByTestId("restore-run"));

    await screen.findByTestId("restore-result");
    const selections = (restoreManifest.mock.calls[0] as unknown[])[1] as Array<{ id: string; overwrite?: boolean }>;
    expect(selections).toHaveLength(3);
    expect(selections.find((item) => item.id === conflictItem.id)?.overwrite).toBe(true);
    expect(screen.getByTestId("restore-result").textContent).toContain("已还原 3 项");
    expect(screen.getByTestId("restore-result").textContent).toContain("可从操作记录恢复");
    expect(onChanged).toHaveBeenCalled();
  });

  it("keeps the view open and refreshes the preview when a run is cancelled", async () => {
    restorePreview.mockResolvedValue(previewOf());
    restoreManifest.mockImplementation(
      (_vaultId: string, _selections: unknown, _requestId: string, signal?: AbortSignal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
        }),
    );
    renderView();

    await screen.findByTestId("restore-group-create");
    fireEvent.click(screen.getByTestId("restore-run"));
    await waitFor(() => expect(screen.getByTestId("restore-run").textContent).toContain("还原中"));
    fireEvent.click(screen.getByTestId("restore-cancel"));

    await screen.findByTestId("restore-cancelled");
    expect(screen.getByTestId("restore-view")).toBeTruthy();
    expect(screen.getByTestId("restore-cancelled").textContent).toContain("已完成的项已保留");
    await waitFor(() => expect(restorePreview.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it("rebuilds an unreadable manifest after confirmation", async () => {
    restorePreview
      .mockResolvedValueOnce({ preview: { status: "unreadable", items: [], pending: 0, localOnly: [], unmatched: [], warnings: [], canRebuild: true } })
      .mockResolvedValueOnce(previewOf());
    rebuildManifest.mockResolvedValue({ written: 3, skipped: 1, corruptCopy: "skill-distributions.json.corrupt-20261122T090000Z" });
    const { onResult } = renderView();

    await screen.findByTestId("restore-status-warn");
    fireEvent.click(screen.getByTestId("restore-rebuild"));
    fireEvent.click(screen.getByTestId("restore-rebuild-confirm"));

    await waitFor(() => expect(rebuildManifest).toHaveBeenCalledWith("v1"));
    await waitFor(() => expect(restorePreview).toHaveBeenCalledTimes(2));
    expect(onResult).toHaveBeenCalledWith([
      "已重建分发清单：写入 3 项，跳过 1 项",
      "原文件已保留为 skill-distributions.json.corrupt-20261122T090000Z",
    ]);
    await screen.findByTestId("restore-group-create");
  });

  it("refuses to rebuild a future-version manifest", async () => {
    restorePreview.mockResolvedValue({ preview: { status: "version-unknown", items: [], pending: 0, localOnly: [], unmatched: [], warnings: [], canRebuild: false } });
    renderView();

    await screen.findByTestId("restore-status-warn");
    expect(screen.getByTestId("restore-status-warn").textContent).toContain("更新版本");
    expect(screen.queryByTestId("restore-rebuild")).toBeNull();
  });
});
