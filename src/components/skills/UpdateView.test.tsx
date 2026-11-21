import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const updatePreview = vi.fn();
const discardStage = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {
    readonly code: string;
    constructor(code: string, message: string) {
      super(message);
      this.code = code;
    }
  }
  return {
    TransportError,
    skillsApi: {
      updatePreview: (...args: unknown[]) => updatePreview(...args),
      discardStage: (...args: unknown[]) => discardStage(...args),
    },
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

const { UpdateView } = await import("./UpdateView");
const { TransportError } = await import("../../lib/transport");

const commit = "0123456789abcdef0123456789abcdef01234567";
const latest = "89abcdef0123456789abcdef0123456789abcdef";

const plan = {
  id: "plan-aaaaaaaaaaaaaaaa",
  vaultId: "v1",
  origin: "user",
  risk: "medium",
  actions: [{ id: "a1", kind: "edit", target: "skills/文档/pdf/SKILL.md", text: "# new", reason: "更新技能" }],
};

beforeEach(() => {
  updatePreview.mockReset();
  discardStage.mockReset();
  discardStage.mockResolvedValue({});
  updatePreview.mockResolvedValue({
    stageId: "s-0123456789ab",
    from: commit,
    to: latest,
    files: [
      { path: "SKILL.md", change: "modified", bytes: 4096, binary: false },
      { path: "scripts/fill.py", change: "added", bytes: 2048, binary: false },
      { path: "old.md", change: "removed", bytes: 100, binary: false },
      { path: "assets/fields.png", change: "added", bytes: 38000, binary: true },
    ],
    plan,
  });
});

afterEach(cleanup);

function renderView(onProposed = vi.fn(), onClose = vi.fn()) {
  const view = render(
    <UpdateView vaultId="v1" skillPath="skills/文档/pdf" skillName="pdf" onClose={onClose} onProposed={onProposed} />,
  );
  return { onProposed, onClose, unmount: view.unmount };
}

describe("UpdateView", () => {
  it("lists the diff and proposes the plan", async () => {
    const { onProposed } = renderView();
    await waitFor(() => expect(screen.getByTestId("update-files")).toBeTruthy());
    expect(updatePreview).toHaveBeenCalledWith("v1", { kind: "kb", skillPath: "skills/文档/pdf" });
    expect(screen.getByTestId("update-origin").textContent).toContain("0123456 → 89abcde");
    const files = screen.getByTestId("update-files").textContent ?? "";
    expect(files).toContain("SKILL.md");
    expect(files).toContain("新增");
    expect(files).toContain("移到回收站");
    expect(screen.getByTestId("update-summary").textContent).toBe("修改 1 · 新增 2 · 删除 1");
    fireEvent.click(screen.getByTestId("update-propose"));
    expect(onProposed).toHaveBeenCalledWith(plan, "s-0123456789ab");
    expect(discardStage).not.toHaveBeenCalled();
  });

  it("drops the stage when the view closes without a plan", async () => {
    const view = renderView();
    await waitFor(() => expect(screen.getByTestId("update-files")).toBeTruthy());
    view.unmount();
    expect(discardStage).toHaveBeenCalledWith("s-0123456789ab");
  });

  it("shows the local-edit conflict", async () => {
    updatePreview.mockRejectedValue(new TransportError("skill-conflict", "本地改过，不能一键更新"));
    renderView();
    await waitFor(() => expect(screen.getByTestId("update-error")).toBeTruthy());
    expect(screen.getByTestId("update-error").textContent).toContain("本地改过");
    expect(screen.queryByTestId("update-propose")).toBeNull();
  });

  it("says when there is nothing to write", async () => {
    updatePreview.mockResolvedValue({ from: commit, to: latest, files: [] });
    renderView();
    await waitFor(() => expect(screen.getByTestId("update-empty")).toBeTruthy());
    expect((screen.getByTestId("update-propose") as HTMLButtonElement).disabled).toBe(true);
  });
});
