import { describe, expect, it } from "vitest";
import { advanceGate, expireGate, initialGate } from "./confirmGate";

describe("advanceGate", () => {
  it("arms on the first click without firing", () => {
    const first = advanceGate(initialGate, 1000);
    expect(first.fire).toBe(false);
    expect(first.state.armedAt).toBe(1000);
  });

  it("ignores a second click inside 600ms (double click)", () => {
    const armed = { armedAt: 1000 };
    expect(advanceGate(armed, 1100).fire).toBe(false);
    expect(advanceGate(armed, 1599).fire).toBe(false);
  });

  it("fires once the gap is at least 600ms, and re-arms from scratch", () => {
    const armed = { armedAt: 1000 };
    const second = advanceGate(armed, 1600);
    expect(second.fire).toBe(true);
    expect(second.state.armedAt).toBeNull();
  });
});

describe("expireGate", () => {
  it("resets after 4s", () => {
    expect(expireGate({ armedAt: 1000 }, 5200).armedAt).toBeNull();
  });
  it("keeps a fresh armed state", () => {
    expect(expireGate({ armedAt: 1000 }, 3000).armedAt).toBe(1000);
  });
});
