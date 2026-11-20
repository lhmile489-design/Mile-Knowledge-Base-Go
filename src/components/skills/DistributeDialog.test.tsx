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

describe("DistributeDialog", () => {
  it("previews targets, submits the selection and reports results", async () => {
    const onClose = vi.fn();
    const onDone = vi.fn();
    const onResult = vi.fn();
    render(
      <DistributeDialog vaultId="v1" skills={[skill]} tools={tools as never} onClose={onClose} onDone={onDone} onResult={onResult} />,
    );
    await waitFor(() => expect(preview).toHaveBeenCalled());
    fireEvent.click(screen.getByTestId("distribute-target-claude-code"));
    fireEvent.click(screen.getByTestId("distribute-submit"));
    await waitFor(() => expect(distribute).toHaveBeenCalledTimes(1));
    const [vaultId, path, inputs, mode] = distribute.mock.calls[0] as [string, string, unknown, string];
    expect(vaultId).toBe("v1");
    expect(path).toBe("skills/前端/vue");
    expect(inputs).toEqual([{ targetId: "claude-code", allowOverwrite: false }]);
    expect(mode).toBe("link");
    await waitFor(() => expect(onResult).toHaveBeenCalled());
    expect((onResult.mock.calls[0]?.[0] as string[]).join(" ")).toContain("已联接");
    expect(onDone).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("closes on Escape without closing the panel", () => {
    const onClose = vi.fn();
    render(<DistributeDialog vaultId="v1" skills={[skill]} tools={tools as never} onClose={onClose} onDone={vi.fn()} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });
});
