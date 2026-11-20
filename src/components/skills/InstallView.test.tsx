import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const log = vi.fn();
const restore = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      log: (...args: unknown[]) => log(...args),
      restore: (...args: unknown[]) => restore(...args),
    },
    settingsApi: { patch: vi.fn(), get: vi.fn() },
    vaultApi: { list: vi.fn(), pickSkillSource: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
      skillsChanged: { on: vi.fn() },
    },
  };
});

const { InstallView } = await import("./InstallView");

beforeEach(() => {
  log.mockReset().mockResolvedValue({
    records: [
      {
        id: "l-1",
        at: "2026-11-20T09:00:00Z",
        op: "uninstall",
        skill: "vue",
        toolId: "claude-code",
        location: "<home>/.claude/skills/vue",
        result: "ok",
        backupId: "b-aaaaaaaaaaaa",
        restorable: true,
      },
      {
        id: "l-2",
        at: "2026-11-20T08:00:00Z",
        op: "distribute",
        skill: "go",
        toolId: "agents",
        location: "<home>/.agents/skills/go",
        result: "ok",
        restorable: false,
      },
    ],
  });
  restore.mockResolvedValue({ restoredTo: "<home>/.claude/skills/vue" });
});

afterEach(cleanup);

describe("InstallView", () => {
  it("shows the sources and the recent records", async () => {
    render(<InstallView vaultId="v1" onBack={vi.fn()} onChanged={vi.fn()} />);
    expect(screen.getByTestId("install-local")).toBeTruthy();
    expect(screen.getByRole("button", { name: /GitHub/ })).toHaveProperty("disabled", true);
    await waitFor(() => expect(screen.getByTestId("install-records")).toBeTruthy());
    expect(screen.getByText("卸载")).toBeTruthy();
    expect(screen.getByText("分发")).toBeTruthy();
  });

  it("restores a backup and refreshes", async () => {
    const onChanged = vi.fn();
    render(<InstallView vaultId="v1" onBack={vi.fn()} onChanged={onChanged} />);
    await waitFor(() => expect(screen.getByTestId("install-restore-l-1")).toBeTruthy());
    fireEvent.click(screen.getByTestId("install-restore-l-1"));
    await waitFor(() => expect(restore).toHaveBeenCalledWith("b-aaaaaaaaaaaa"));
    await waitFor(() => expect(screen.getByTestId("install-notice").textContent).toContain("已恢复"));
    expect(onChanged).toHaveBeenCalled();
  });

  it("opens the embedded local-folder wizard and returns", async () => {
    render(<InstallView vaultId="v1" onBack={vi.fn()} onChanged={vi.fn()} />);
    fireEvent.click(screen.getByTestId("install-local"));
    expect(screen.getByTestId("skill-wizard")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "关闭导入向导" }));
    expect(screen.queryByTestId("skill-wizard")).toBeNull();
    expect(screen.getByTestId("install-local")).toBeTruthy();
  });
});
