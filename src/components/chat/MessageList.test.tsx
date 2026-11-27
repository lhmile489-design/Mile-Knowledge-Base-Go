import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

vi.mock("../overview/OverviewGrid", () => ({ OverviewGrid: () => <div data-testid="overview-stub" /> }));
vi.mock("../../lib/transport", () => ({
  agentApi: {},
  events: {},
  TransportError: class TransportError extends Error {},
}));

const { MessageList } = await import("./MessageList");
const { useAgentStore } = await import("../../stores/agentStore");

function withLines(lines: { id: string; role: "user" | "assistant"; text: string }[]): void {
  const state = useAgentStore.getState();
  useAgentStore.setState({
    buckets: {
      ...state.buckets,
      [state.agent]: { ...state.buckets[state.agent], transcript: { lines, sealed: true } },
    },
  });
}

beforeEach(() => {
  withLines([]);
});

afterEach(cleanup);

describe("MessageList material (M10 T06)", () => {
  it("lets the overview tiles sit on the glass shell instead of an opaque sheet", () => {
    render(<MessageList />);
    expect(screen.getByTestId("overview-stub")).toBeTruthy();
    const body = screen.getByTestId("chat-body");
    expect(body.classList.contains("sheet")).toBe(false);
    expect(body.getAttribute("data-empty")).toBe("true");
  });

  it("is an opaque sheet as soon as there is a message stream", () => {
    withLines([{ id: "u1", role: "user", text: "你好" }]);
    render(<MessageList />);
    expect(screen.queryByTestId("overview-stub")).toBeNull();
    const body = screen.getByTestId("chat-body");
    expect(body.classList.contains("sheet")).toBe(true);
    expect(body.hasAttribute("data-empty")).toBe(false);
  });
});
