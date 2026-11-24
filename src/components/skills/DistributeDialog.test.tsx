import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const preview = vi.fn();
const distribute = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      distributePreview: (...args: unknown[]) => preview(...args),
      distribute: (...args: unknown[]) => distribute(...args),
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

const { DistributeDialog } = await import("./DistributeDialog");

const tools = [
  { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
  { id: "claude-code", name: "Claude Code", iconKey: "claude-code", installDir: "<home>/.claude/skills", scanDirs: [], readers: [], status: "detected", official: true, enabled: true },
];
const skill = { name: "vue", description: "", path: "skills/前端/vue", category: "前端", distributed: [] };
const other = { name: "go", description: "", path: "skills/后端/go", category: "后端", distributed: [] };

function renderDialog(overrides: Partial<{ skills: unknown[]; onClose: () => void; onDone: (ok: boolean) => void }> = {}) {
  const onClose = overrides.onClose ?? vi.fn();
  const onDone = overrides.onDone ?? vi.fn();
  render(
    <DistributeDialog
      vaultId="v1"
      skills={(overrides.skills ?? [skill]) as never}
      tools={tools as never}
      onClose={onClose}
      onDone={onDone}
    />,
  );
  return { onClose, onDone };
}

beforeEach(() => {
  preview.mockReset().mockResolvedValue({
    items: [
      { targetId: "agents", toolId: "agents", name: "vue", action: "create", targetPath: "<home>/.agents/skills/vue" },
      { targetId: "claude-code", toolId: "claude-code", name: "vue", action: "create", targetPath: "<home>/.claude/skills/vue" },
    ],
  });
  distribute.mockReset().mockResolvedValue({
    items: [{ targetId: "claude-code", toolId: "claude-code", result: "linked" }],
  });
});

afterEach(cleanup);

describe("DistributeDialog as a modal", () => {
  it("is a dialog titled with the skill, with the skill names as tags", async () => {
    renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    expect(screen.getByRole("dialog", { name: "分发「vue」" })).toBeTruthy();
    expect(screen.getByTestId("distribute-dialog")).toBeTruthy();
    expect(screen.getByTestId("distribute-skills").textContent).toContain("vue");
    expect(screen.getByTestId("skills-modal").hasAttribute("inert")).toBe(false);
  });

  it("lists every skill of a batch and titles it with the count", async () => {
    renderDialog({ skills: [skill, other] });
    expect(screen.getByRole("dialog", { name: "分发 2 个技能" })).toBeTruthy();
    const tags = screen.getByTestId("distribute-skills").textContent ?? "";
    expect(tags).toContain("vue");
    expect(tags).toContain("go");
  });

  it("cancels with the button, the close button and Esc without calling onDone", async () => {
    const { onClose, onDone } = renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-cancel"));
    fireEvent.click(screen.getByTestId("skills-modal-close"));
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(3);
    expect(onDone).not.toHaveBeenCalled();
  });

  it("submits the selection and shows the result inside the dialog until 完成", async () => {
    const { onClose, onDone } = renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    expect(screen.getByTestId("distribute-summary").textContent).toContain("已选 1");
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(distribute).toHaveBeenCalledTimes(1));
    const [vaultId, path, inputs, mode] = distribute.mock.calls[0] as [string, string, unknown, string];
    expect(vaultId).toBe("v1");
    expect(path).toBe("skills/前端/vue");
    expect(inputs).toEqual([{ targetId: "claude-code", allowOverwrite: false }]);
    expect(mode).toBe("link");

    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    expect(screen.getByTestId("distribute-result").textContent).toContain("vue → Claude Code");
    expect(screen.getByTestId("distribute-result").textContent).toContain("已联接");
    expect(screen.getByTestId("distribute-summary").textContent).toContain("成功 1 处");
    // The list refresh and the batch exit wait for 完成, not for the write.
    expect(onDone).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("distribute-done"));
    expect(onDone).toHaveBeenCalledWith(true);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closing from the result step (Esc or the close button) finishes the same way", async () => {
    const { onClose, onDone } = renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onDone).toHaveBeenCalledWith(true);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("cannot be closed while distributing", async () => {
    let finish: (value: unknown) => void = () => undefined;
    distribute.mockReturnValue(new Promise((resolve) => (finish = resolve)));
    const { onClose, onDone } = renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(distribute).toHaveBeenCalled());

    expect(screen.getByTestId("skills-modal-close").getAttribute("aria-disabled")).toBe("true");
    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(screen.getByTestId("skills-modal-close"));
    expect(onClose).not.toHaveBeenCalled();
    expect((screen.getByTestId("distribute-cancel") as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByTestId("distribute-submit") as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByTestId("distribute-target-claude-code").getAttribute("aria-disabled")).toBe("true");

    finish({ items: [{ targetId: "claude-code", toolId: "claude-code", result: "linked" }] });
    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    expect(screen.getByTestId("skills-modal-close").hasAttribute("aria-disabled")).toBe(false);
    expect(onDone).not.toHaveBeenCalled();
  });

  it("shows a failed write as a failed row and reports onDone(false)", async () => {
    distribute.mockRejectedValue(new Error("磁盘已满"));
    const { onDone } = renderDialog();
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    expect(screen.getByTestId("distribute-result").textContent).toContain("磁盘已满");
    expect(screen.getByTestId("distribute-summary").textContent).toContain("失败 1 处");
    fireEvent.click(screen.getByTestId("distribute-done"));
    expect(onDone).toHaveBeenCalledWith(false);
  });

  it("keeps the rows of the skills that were written before a later one failed", async () => {
    distribute
      .mockResolvedValueOnce({ items: [{ targetId: "claude-code", toolId: "claude-code", result: "linked" }] })
      .mockRejectedValueOnce(new Error("目标被占用"));
    const { onDone } = renderDialog({ skills: [skill, other] });
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(screen.getByTestId("distribute-result")).toBeTruthy());
    const text = screen.getByTestId("distribute-result").textContent ?? "";
    expect(text).toContain("vue → Claude Code");
    expect(text).toContain("已联接");
    expect(text).toContain("go");
    expect(text).toContain("目标被占用");
    expect(screen.getByTestId("distribute-summary").textContent).toContain("成功 1 处");
    fireEvent.click(screen.getByTestId("distribute-done"));
    // One skill landed, so the batch selection can end.
    expect(onDone).toHaveBeenCalledWith(true);
  });
});

describe("DistributeDialog overwrite confirmation", () => {
  beforeEach(() => {
    preview.mockResolvedValue({
      items: [
        { targetId: "agents", toolId: "agents", name: "vue", action: "create", targetPath: "<home>/.agents/skills/vue" },
        {
          targetId: "claude-code",
          toolId: "claude-code",
          name: "vue",
          action: "exists",
          detail: "目标已存在",
          targetPath: "<home>/.claude/skills/vue",
        },
      ],
    });
  });

  const tickOverwrite = async () => {
    renderDialogForOverwrite();
    const tile = await screen.findByTestId("distribute-target-claude-code");
    await waitFor(() => expect(tile.querySelector('input[type="checkbox"]')).toBeTruthy());
    fireEvent.click(tile.querySelector('input[type="checkbox"]') as HTMLInputElement);
  };
  let handlers: { onClose: () => void; onDone: (ok: boolean) => void };
  const renderDialogForOverwrite = () => {
    handlers = renderDialog();
  };

  it("asks before overwriting: the card names the folder and the backup, and stacks above the dialog", async () => {
    await tickOverwrite();
    const card = await screen.findByRole("alertdialog", { name: "覆盖确认" });
    expect(card.textContent).toContain("覆盖 Claude Code 里的 vue");
    expect(card.textContent).toContain("<home>/.claude/skills/vue");
    expect(card.textContent).toContain("备份");
    expect(screen.getByRole("dialog", { name: "分发「vue」" }).hasAttribute("inert")).toBe(true);
    // Nothing is overwritten until the user says so.
    expect(screen.getByTestId("distribute-target-claude-code").querySelector<HTMLInputElement>('input[type="checkbox"]')?.checked).toBe(false);
  });

  it("「不覆盖这一处」 and Esc both drop the request and keep the dialog open", async () => {
    await tickOverwrite();
    fireEvent.click(await screen.findByTestId("distribute-overwrite-keep"));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.getByRole("dialog", { name: "分发「vue」" })).toBeTruthy();
    expect(handlers.onClose).not.toHaveBeenCalled();

    const tile = screen.getByTestId("distribute-target-claude-code");
    fireEvent.click(tile.querySelector('input[type="checkbox"]') as HTMLInputElement);
    await screen.findByRole("alertdialog");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("alertdialog")).toBeNull();
    // Esc closed only the card; the dialog stays.
    expect(handlers.onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "分发「vue」" })).toBeTruthy();
  });

  it("「备份后覆盖」 selects the target with allowOverwrite and the summary counts it", async () => {
    await tickOverwrite();
    fireEvent.click(await screen.findByTestId("distribute-overwrite-go"));
    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(screen.getByTestId("distribute-summary").textContent).toContain("覆盖 1 处");
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(distribute).toHaveBeenCalled());
    expect((distribute.mock.calls[0] as unknown[])[2]).toEqual([{ targetId: "claude-code", allowOverwrite: true }]);
  });
});

describe("DistributeDialog M6 targets", () => {
  const m6Tools = [
    { id: "agents", name: "通用目录", iconKey: "globe", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
    { id: "claude-code", name: "Claude Code", iconKey: "claude-code", installDir: "<home>/.claude/skills", scanDirs: [], readers: [], status: "detected", kind: "builtin", official: true, enabled: true },
    { id: "t-111111111111", name: "Droid", iconKey: "", installDir: "<home>/droid", scanDirs: [], readers: [], status: "detected", kind: "custom", official: true, enabled: true },
    { id: "t-222222222222", name: "acme-site", iconKey: "folder", installDir: "D:\…\acme-site\.claude\skills", installRel: ".claude/skills", scanDirs: [], readers: [], status: "detected", kind: "project", official: true, enabled: true },
    { id: "t-333333333333", name: "gone-proj", iconKey: "folder", installDir: "D:\…\gone\.claude\skills", installRel: ".claude/skills", scanDirs: [], readers: [], status: "detected", kind: "project", missing: true, official: true, enabled: true },
  ];

  it("lists registered targets with their install notes and disables missing ones", async () => {
    preview.mockResolvedValue({ items: [] });
    render(<DistributeDialog vaultId="v1" skills={[skill] as never} tools={m6Tools as never} onClose={vi.fn()} onDone={vi.fn()} />);
    const projectCell = await screen.findByTestId("distribute-target-t-222222222222");
    expect(projectCell.textContent).toContain("写入 .claude/skills");
    expect(screen.getByTestId("distribute-target-t-111111111111").textContent).toContain("droid");
    const missingCell = screen.getByTestId("distribute-target-t-333333333333");
    expect(missingCell.getAttribute("aria-disabled")).toBe("true");
    expect(missingCell.textContent).toContain("目录不存在");
    // Shared directory first, registered targets after the built-ins.
    const order = [...document.querySelectorAll(".skills-target")].map((node) => node.getAttribute("data-testid"));
    expect(order).toEqual([
      "distribute-target-agents",
      "distribute-target-claude-code",
      "distribute-target-t-111111111111",
      "distribute-target-t-222222222222",
      "distribute-target-t-333333333333",
    ]);
  });
});
