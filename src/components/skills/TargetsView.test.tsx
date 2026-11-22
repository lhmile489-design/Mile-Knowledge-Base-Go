import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const listTargets = vi.fn();
const pickTargetFolder = vi.fn();
const saveTarget = vi.fn();
const removeTarget = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      listTargets: () => listTargets(),
      pickTargetFolder: () => pickTargetFolder(),
      saveTarget: (input: unknown) => saveTarget(input),
      removeTarget: (id: string) => removeTarget(id),
    },
  };
});

const { TargetsView } = await import("./TargetsView");

const custom = {
  id: "t-111111111111",
  kind: "custom",
  name: "Droid Skills",
  path: "<home>/droid-skills",
  exists: true,
  skillCount: 12,
  createdAt: "2026-11-22T09:00:00Z",
};

const project = {
  id: "t-222222222222",
  kind: "project",
  name: "Mile KB Go",
  path: "C:\\…\\Mile-Knowledge-Base-Go",
  installDir: ".claude/skills",
  exists: false,
  skillCount: 18,
  dirs: [
    { rel: ".claude/skills", exists: true, skills: 12 },
    { rel: ".agents/skills", exists: true, skills: 6 },
  ],
  createdAt: "2026-11-22T10:00:00Z",
};

function renderView(overrides: Partial<{ onBack: () => void; onChanged: () => void }> = {}) {
  const onBack = overrides.onBack ?? vi.fn();
  const onChanged = overrides.onChanged ?? vi.fn();
  render(<TargetsView vaultId="v1" onBack={onBack} onChanged={onChanged} onResult={vi.fn()} />);
  return { onBack, onChanged };
}

beforeEach(() => {
  listTargets.mockReset();
  pickTargetFolder.mockReset();
  saveTarget.mockReset();
  removeTarget.mockReset();
});

afterEach(cleanup);

describe("TargetsView", () => {
  it("lists registered targets with kind, path and missing state", async () => {
    listTargets.mockResolvedValue({ items: [custom, project] });
    renderView();
    await waitFor(() => expect(screen.getByTestId("target-row-t-111111111111")).toBeTruthy());
    expect(screen.getByText("自定义")).toBeTruthy();
    expect(screen.getByText("项目")).toBeTruthy();
    expect(screen.getByText("<home>/droid-skills")).toBeTruthy();
    expect(screen.getByText("12 个技能")).toBeTruthy();
    expect(screen.getByText("18 个技能 · 2 个子目录")).toBeTruthy();
    expect(screen.getByText("目录不存在")).toBeTruthy();
    expect(screen.getByTestId("targets-count").textContent).toContain("自定义 1 · 项目 1");
  });

  it("creates a custom tool through the folder token", async () => {
    listTargets.mockResolvedValue({ items: [] });
    pickTargetFolder.mockResolvedValue({
      cancelled: false,
      token: "tok-1",
      displayName: "droid-skills",
      path: "<home>/droid-skills",
      dirs: [],
    });
    saveTarget.mockResolvedValue({ target: { ...custom } });
    const onChanged = vi.fn();
    renderView({ onChanged });
    await waitFor(() => expect(screen.getByTestId("targets-empty")).toBeTruthy());

    fireEvent.click(screen.getByTestId("targets-add-custom"));
    fireEvent.change(screen.getByTestId("target-name"), { target: { value: "Droid Skills" } });
    fireEvent.click(screen.getByTestId("target-pick"));
    await waitFor(() => expect(screen.getByTestId("target-picked").textContent).toContain("<home>/droid-skills"));
    fireEvent.click(screen.getByTestId("target-save"));

    await waitFor(() => expect(saveTarget).toHaveBeenCalledTimes(1));
    expect(saveTarget.mock.calls[0]![0]).toEqual({
      id: undefined,
      kind: "custom",
      name: "Droid Skills",
      token: "tok-1",
      installDir: undefined,
    });
    expect(onChanged).toHaveBeenCalled();
  });

  it("registers a project and lets the install subdir be chosen", async () => {
    listTargets.mockResolvedValue({ items: [] });
    pickTargetFolder.mockResolvedValue({
      cancelled: false,
      token: "tok-2",
      displayName: "acme-site",
      path: "D:\\…\\acme-site",
      dirs: [
        { rel: ".claude/skills", exists: true, skills: 12 },
        { rel: ".agents/skills", exists: true, skills: 3 },
      ],
    });
    saveTarget.mockResolvedValue({ target: { ...project, exists: true } });
    renderView();
    await waitFor(() => expect(screen.getByTestId("targets-empty")).toBeTruthy());

    fireEvent.click(screen.getByTestId("targets-add-project"));
    fireEvent.change(screen.getByTestId("target-name"), { target: { value: "acme-site" } });
    fireEvent.click(screen.getByTestId("target-pick"));
    await waitFor(() => expect(screen.getByTestId("target-dirs")).toBeTruthy());
    expect(screen.getByTestId("target-dirs").textContent).toContain(".agents/skills");
    // The first existing dir wins by default.
    expect(screen.getByTestId("target-install-.claude/skills").getAttribute("aria-checked")).toBe("true");
    fireEvent.click(screen.getByTestId("target-install-.agents/skills"));
    fireEvent.click(screen.getByTestId("target-save"));

    await waitFor(() => expect(saveTarget).toHaveBeenCalledTimes(1));
    expect(saveTarget.mock.calls[0]![0]).toMatchObject({
      kind: "project",
      name: "acme-site",
      token: "tok-2",
      installDir: ".agents/skills",
    });
  });

  it("shows a validation failure in place", async () => {
    listTargets.mockResolvedValue({ items: [custom] });
    pickTargetFolder.mockResolvedValue({
      cancelled: false,
      token: "tok-3",
      path: "<home>/x",
      dirs: [],
    });
    saveTarget.mockRejectedValue(new Error("与内置工具「Claude Code」的目录重叠"));
    renderView();
    await waitFor(() => expect(screen.getByTestId("target-row-t-111111111111")).toBeTruthy());

    fireEvent.click(screen.getByTestId("targets-add-custom"));
    fireEvent.change(screen.getByTestId("target-name"), { target: { value: "X" } });
    fireEvent.click(screen.getByTestId("target-pick"));
    await waitFor(() => expect(screen.getByTestId("target-picked").textContent).toContain("<home>/x"));
    fireEvent.click(screen.getByTestId("target-save"));

    await waitFor(() => expect(screen.getByTestId("targets-error").textContent).toContain("重叠"));
  });

  it("confirms removal and reports the record counts", async () => {
    listTargets.mockResolvedValueOnce({ items: [custom] }).mockResolvedValueOnce({ items: [] });
    removeTarget.mockResolvedValue({ removed: true, distributionsRemoved: 3, sourcesRemoved: 1 });
    const onResult = vi.fn();
    const onChanged = vi.fn();
    render(
      <TargetsView vaultId="v1" onBack={vi.fn()} onChanged={onChanged} onResult={onResult} />,
    );
    await waitFor(() => expect(screen.getByTestId("target-row-t-111111111111")).toBeTruthy());

    fireEvent.click(screen.getByTestId("target-remove-t-111111111111"));
    await waitFor(() => expect(screen.getByTestId("target-remove-dialog")).toBeTruthy());
    expect(screen.getByTestId("target-remove-dialog").textContent).toContain("目录里的文件不会删除");
    fireEvent.click(screen.getByTestId("target-remove-confirm"));

    await waitFor(() => expect(removeTarget).toHaveBeenCalledWith("t-111111111111"));
    expect(onChanged).toHaveBeenCalled();
    expect(onResult.mock.calls[0]![0][0]).toContain("删除 3 条分发记录、1 条来源记录");
    await waitFor(() => expect(screen.getByTestId("targets-empty")).toBeTruthy());
  });
});
