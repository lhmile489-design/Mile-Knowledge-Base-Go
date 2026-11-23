import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ProviderDraft, ProvidersResult } from "../../mygo";

const saveProviders = vi.fn();
const reload = vi.fn(async () => undefined);

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    settingsApi: {
      saveProviders: (...args: unknown[]) => saveProviders(...args),
      fetchModels: vi.fn(),
      setKey: vi.fn(),
    },
  };
});

const { ModelServiceSection } = await import("./ModelServiceSection");

const providers: ProviderDraft[] = [
  {
    name: "fake",
    kind: "openai",
    baseUrl: "https://api.example.com",
    models: ["m1"],
    defaultModel: "m1",
    apiKeyEnv: "",
  },
  {
    name: "other",
    kind: "openai",
    baseUrl: "https://api2.example.com",
    models: ["m2"],
    defaultModel: "m2",
    apiKeyEnv: "",
  },
];

function dataWith(list: ProviderDraft[]): ProvidersResult {
  return { providers: list, credentials: [] };
}

function renderSection(list: ProviderDraft[] = providers): void {
  render(
    <ModelServiceSection
      vaultId="v1"
      data={dataWith(list)}
      error={null}
      reload={reload}
      onClose={() => undefined}
    />,
  );
}

beforeEach(() => {
  saveProviders.mockReset();
  saveProviders.mockImplementation(async (_vaultId: string, next: ProviderDraft[]) => dataWith(next));
  reload.mockClear();
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ModelServiceSection provider menu", () => {
  it("keeps the menu open when a menu item is pressed", () => {
    renderSection();
    fireEvent.click(screen.getAllByTestId("provider-more")[0]!);
    expect(screen.getByTestId("provider-menu")).toBeTruthy();

    // A real mouse press fires pointerdown before click; pressing inside the
    // menu must not unmount it, or the click never reaches the item.
    fireEvent.pointerDown(screen.getByTestId("provider-delete"));
    expect(screen.getByTestId("provider-menu")).toBeTruthy();

    fireEvent.pointerDown(document.body);
    expect(screen.queryByTestId("provider-menu")).toBeNull();
  });

  it("deletes a provider after the two-step confirmation", async () => {
    let clock = 1_000;
    vi.spyOn(Date, "now").mockImplementation(() => clock);
    renderSection();
    fireEvent.click(screen.getAllByTestId("provider-more")[0]!);
    fireEvent.pointerDown(screen.getByTestId("provider-delete"));
    fireEvent.click(screen.getByTestId("provider-delete"));
    expect(screen.getByTestId("provider-delete").textContent).toBe("确认删除");

    clock = 2_000;
    fireEvent.pointerDown(screen.getByTestId("provider-delete"));
    fireEvent.click(screen.getByTestId("provider-delete"));
    await waitFor(() => expect(saveProviders).toHaveBeenCalledTimes(1));
    const sent = saveProviders.mock.calls[0]?.[1] as ProviderDraft[];
    expect(sent.map((item) => item.name)).toEqual(["other"]);
    expect(reload).toHaveBeenCalled();
  });
});
