import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const backups = vi.fn();
const cleanBackups = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    skillsApi: {
      backups: (...args: unknown[]) => backups(...args),
      cleanBackups: (...args: unknown[]) => cleanBackups(...args),
    },
  };
});

const { SkillBackupsRow } = await import("./SkillBackupsRow");

const item = {
  id: "b-aaaaaaaaaaaa",
  toolId: "claude-code",
  originalPath: "<home>/.claude/skills/vue",
  kind: "uninstall",
  createdAt: "2026-11-20T09:00:00Z",
  size: 2048,
  files: 2,
  restorable: true,
};

beforeEach(() => {
  backups.mockReset();
  cleanBackups.mockReset();
  backups.mockResolvedValue({ items: [item] });
  cleanBackups.mockResolvedValue({ items: [] });
});

afterEach(cleanup);

describe("SkillBackupsRow", () => {
  it("shows the total and opens the list", async () => {
    render(<SkillBackupsRow />);
    await waitFor(() => expect(screen.getByTestId("skill-backups-count").textContent).toContain("1 个"));
    fireEvent.click(screen.getByTestId("skill-backups-manage"));
    expect(screen.getByTestId("skill-backups-dialog")).toBeTruthy();
    expect(screen.getByText("<home>/.claude/skills/vue")).toBeTruthy();
  });

  it("cleans the selected backups and refreshes the count", async () => {
    render(<SkillBackupsRow />);
    await waitFor(() => expect(screen.getByTestId("skill-backups-count").textContent).toContain("1 个"));
    fireEvent.click(screen.getByTestId("skill-backups-manage"));
    fireEvent.click(screen.getByTestId("backup-row-b-aaaaaaaaaaaa"));
    fireEvent.click(screen.getByTestId("skill-backups-clean"));
    await waitFor(() => expect(cleanBackups).toHaveBeenCalledWith(["b-aaaaaaaaaaaa"]));
    await waitFor(() => expect(screen.getByTestId("skill-backups-count").textContent).toContain("0 个"));
  });
});
