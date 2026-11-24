import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const renderDocument = vi.fn();
const openExternal = vi.fn();
const read = vi.fn();

vi.mock("../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    renderApi: { document: (...args: unknown[]) => renderDocument(...args) },
    windowApi: { openExternal: (...args: unknown[]) => openExternal(...args) },
    docApi: {
      read: (...args: unknown[]) => read(...args),
      noteOpened: vi.fn(async () => undefined),
      write: vi.fn(),
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

const { ReaderView } = await import("./ReaderView");
const { useDocStore } = await import("../stores/docStore");

const html = [
  '<h1>vue</h1>',
  '<p><a data-kind="doc" data-status="ok" data-target="notes/a.md" href="#">好链接</a></p>',
  '<p><a data-kind="wiki" data-status="missing" data-target="nowhere" href="#">坏链接</a></p>',
  '<p><a data-kind="external" href="https://example.com/x">外链</a></p>',
].join("");

beforeEach(() => {
  useDocStore.getState().reset();
  renderDocument.mockReset().mockResolvedValue({ html });
  openExternal.mockReset().mockResolvedValue(undefined);
  read.mockReset().mockResolvedValue({ doc: { path: "notes/a.md", text: "", revision: 1, hash: "h", size: 0 } });
});

afterEach(cleanup);

let seq = 0;
const view = (extra: Partial<Parameters<typeof ReaderView>[0]> = {}) => {
  seq += 1;
  return render(<ReaderView vaultId="v1" path={`skills/前端/vue-${seq}/SKILL.md`} revision={1} hash={`h${seq}`} {...extra} />);
};

describe("ReaderView onBeforeNavigate (M9 T04)", () => {
  it("calls it before opening a resolved in-vault link", async () => {
    const order: string[] = [];
    read.mockImplementation(async () => {
      order.push("open");
      return { doc: { path: "notes/a.md", text: "", revision: 1, hash: "h", size: 0 } };
    });
    view({ onBeforeNavigate: () => order.push("before") });
    fireEvent.click(await screen.findByText("好链接"));
    await waitFor(() => expect(order).toEqual(["before", "open"]));
  });

  it("opens the link as before when the prop is not given", async () => {
    view();
    fireEvent.click(await screen.findByText("好链接"));
    await waitFor(() => expect(useDocStore.getState().activePath).toBe("notes/a.md"));
  });

  it("does not call it for unresolved and external links", async () => {
    const before = vi.fn();
    view({ onBeforeNavigate: before });
    fireEvent.click(await screen.findByText("坏链接"));
    fireEvent.click(screen.getByText("外链"));
    await waitFor(() => expect(openExternal).toHaveBeenCalledWith("https://example.com/x"));
    expect(before).not.toHaveBeenCalled();
    expect(useDocStore.getState().activePath).toBe("");
  });
});
