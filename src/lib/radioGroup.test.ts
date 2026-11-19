import { describe, expect, it } from "vitest";
import { nextOption } from "./radioGroup";

const modes = ["light", "dark", "system"] as const;

describe("nextOption", () => {
  it("moves forward and wraps", () => {
    expect(nextOption(modes, "light", "ArrowRight")).toBe("dark");
    expect(nextOption(modes, "system", "ArrowRight")).toBe("light");
    expect(nextOption(modes, "light", "ArrowDown")).toBe("dark");
  });

  it("moves backward and wraps", () => {
    expect(nextOption(modes, "light", "ArrowLeft")).toBe("system");
    expect(nextOption(modes, "dark", "ArrowUp")).toBe("light");
  });

  it("jumps to the ends with Home and End", () => {
    expect(nextOption(modes, "dark", "Home")).toBe("light");
    expect(nextOption(modes, "dark", "End")).toBe("system");
  });

  it("leaves other keys and unknown values alone", () => {
    expect(nextOption(modes, "light", "Enter")).toBeNull();
    expect(nextOption(modes, "light", "Tab")).toBeNull();
    expect(nextOption(modes, "bogus" as (typeof modes)[number], "ArrowRight")).toBeNull();
  });
});
