import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import type { AppSettings } from "../../lib/transport";

const patch = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {}
  return {
    TransportError,
    settingsApi: { patch: (...args: unknown[]) => patch(...args), get: vi.fn() },
    vaultApi: { list: vi.fn() },
    events: {
      settingsChanged: { on: vi.fn() },
      vaultChanged: { on: vi.fn() },
      appNotice: { on: vi.fn() },
    },
  };
});

const { GeneralSection } = await import("./GeneralSection");
const { AccentPicker } = await import("../AccentPicker");
const { useAppStore } = await import("../../stores/appStore");

function settingsWith(
  accent: AppSettings["accent"],
  effects: AppSettings["effects"] = { motion: "std", blur: "std" },
): AppSettings {
  return {
    version: 1,
    theme: "light",
    closeBehavior: "ask",
    accent,
    effects,
    layout: {
      sidebarWidth: 232,
      docWidth: 560,
      docCollapsed: false,
      sidebarCollapsed: false,
      agnesControlsCollapsed: false,
      agnesHistoryOpen: true,
      agnesComposerHeight: 96,
      skillsKbCollapsed: false,
      skillsToolsCollapsed: false,
    },
    recentVaults: [],
  } as AppSettings;
}

const noop = () => {};

beforeEach(() => {
  patch.mockReset();
  patch.mockImplementation(async (body: { accent: AppSettings["accent"] }) => ({
    settings: settingsWith(body.accent),
  }));
  useAppStore.setState({ settings: settingsWith({ preset: "blue" }), notices: [] });
});

afterEach(cleanup);

describe("GeneralSection accent row", () => {
  it("adds a 主题色 row with the six presets and the current one checked", () => {
    render(<GeneralSection onClose={noop} />);
    const group = screen.getByRole("radiogroup", { name: "主题色" });
    expect(within(group).getAllByRole("radio")).toHaveLength(6);
    expect(screen.getByTestId("settings-accent-preset-blue").getAttribute("aria-checked")).toBe("true");
  });

  it("applies immediately and keeps the title-bar dot in sync", async () => {
    render(
      <>
        <AccentPicker />
        <GeneralSection onClose={noop} />
      </>,
    );
    expect(screen.getByTestId("accent-dot").getAttribute("aria-label")).toBe("主题色：蓝");
    fireEvent.click(screen.getByTestId("settings-accent-preset-green"));
    expect(patch).toHaveBeenCalledWith({ accent: { preset: "green" } });
    await waitFor(() => {
      expect(screen.getByTestId("accent-dot").getAttribute("aria-label")).toBe("主题色：绿");
    });
  });

  it("sends a custom color with its cache from the settings row", () => {
    render(<GeneralSection onClose={noop} />);
    const input = screen.getByTestId("settings-accent-hex");
    fireEvent.change(input, { target: { value: "#ffff00" } });
    expect(screen.getByTestId("settings-accent-hint").textContent).toContain("实际使用");
    fireEvent.keyDown(input, { key: "Enter" });
    expect(patch).toHaveBeenCalledTimes(1);
    const sent = (patch.mock.calls.at(0)?.[0] ?? {}) as { accent: { preset: string; cache?: unknown } };
    expect(sent.accent.preset).toBe("custom");
    expect(sent.accent.cache).toBeTruthy();
  });
});

describe("GeneralSection effects rows (M10 T04)", () => {
  const LEVEL_LABELS = ["关", "弱", "标准", "强"];

  beforeEach(() => {
    delete document.documentElement.dataset.motion;
    delete document.documentElement.dataset.blur;
    patch.mockImplementation(async (body: { effects?: AppSettings["effects"] }) => ({
      settings: settingsWith({ preset: "blue" }, body.effects),
    }));
    useAppStore.setState({
      settings: settingsWith({ preset: "blue" }, { motion: "low", blur: "high" }),
      notices: [],
      effectsDegraded: false,
      systemReducedMotion: false,
    });
  });

  it("puts 动画 and 模糊 right after 主题色, each as a four-level radio group", () => {
    render(<GeneralSection onClose={noop} />);
    const labels = [...document.querySelectorAll(".setting-row-label")].map((node) => node.firstChild?.textContent);
    expect(labels.slice(0, 4)).toEqual(["主题", "主题色", "动画", "模糊"]);
    for (const [name, id] of [["动画", "effects-motion"], ["模糊", "effects-blur"]] as const) {
      const group = within(screen.getByTestId(id)).getByRole("radiogroup", { name });
      expect(within(group).getAllByRole("radio").map((node) => node.textContent)).toEqual(LEVEL_LABELS);
    }
  });

  it("checks the stored level and keeps a single tab stop per group", () => {
    render(<GeneralSection onClose={noop} />);
    expect(screen.getByTestId("effects-motion-low").getAttribute("aria-checked")).toBe("true");
    expect(screen.getByTestId("effects-blur-high").getAttribute("aria-checked")).toBe("true");
    expect(screen.getByTestId("effects-motion-low").tabIndex).toBe(0);
    expect(screen.getByTestId("effects-motion-std").tabIndex).toBe(-1);
  });

  it("applies a level at once and sends the full pair", async () => {
    render(<GeneralSection onClose={noop} />);
    fireEvent.click(screen.getByTestId("effects-motion-high"));
    await waitFor(() => {
      expect(document.documentElement.dataset.motion).toBe("high");
      expect(patch).toHaveBeenCalledWith({ effects: { motion: "high", blur: "high" } });
      expect(screen.getByTestId("effects-motion-high").getAttribute("aria-checked")).toBe("true");
    });
  });

  it("does not write when the checked level is clicked again", async () => {
    render(<GeneralSection onClose={noop} />);
    fireEvent.click(screen.getByTestId("effects-blur-high"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(patch).not.toHaveBeenCalled();
  });

  it("moves with the arrow keys and keeps the focus on the new level", async () => {
    render(<GeneralSection onClose={noop} />);
    const group = within(screen.getByTestId("effects-blur")).getByRole("radiogroup", { name: "模糊" });
    fireEvent.keyDown(group, { key: "ArrowLeft" });
    expect(document.activeElement).toBe(screen.getByTestId("effects-blur-std"));
    await waitFor(() => {
      expect(patch).toHaveBeenCalledWith({ effects: { motion: "low", blur: "std" } });
      expect(screen.getByTestId("effects-blur-std").getAttribute("aria-checked")).toBe("true");
    });
  });

  it("shows no note by default", () => {
    render(<GeneralSection onClose={noop} />);
    expect(screen.queryByTestId("effects-motion-note")).toBeNull();
    expect(screen.queryByTestId("effects-blur-note")).toBeNull();
  });

  it("explains a system-wide animation switch-off without changing the shown level", () => {
    useAppStore.setState({ systemReducedMotion: true });
    render(<GeneralSection onClose={noop} />);
    const note = screen.getByTestId("effects-motion-note");
    expect(note.textContent).toBe("系统已关闭动画效果，当前按「关」显示");
    expect(note.getAttribute("data-tone")).toBe("info");
    expect(screen.getByTestId("effects-motion-low").getAttribute("aria-checked")).toBe("true");
    for (const label of LEVEL_LABELS) {
      const radio = within(screen.getByTestId("effects-motion")).getByRole("radio", { name: label });
      expect((radio as HTMLButtonElement).disabled).toBe(false);
    }
    expect(screen.queryByTestId("effects-blur-note")).toBeNull();
  });

  it("explains an automatic blur switch-off for this run and still lets the user choose", async () => {
    useAppStore.setState({ effectsDegraded: true });
    render(<GeneralSection onClose={noop} />);
    const note = screen.getByTestId("effects-blur-note");
    expect(note.textContent).toBe("当前设备滚动掉帧，已自动关闭模糊（本次运行）");
    expect(note.getAttribute("data-tone")).toBe("warn");
    expect(screen.getByTestId("effects-blur-high").getAttribute("aria-checked")).toBe("true");
    fireEvent.click(screen.getByTestId("effects-blur-low"));
    await waitFor(() => {
      expect(patch).toHaveBeenCalledWith({ effects: { motion: "low", blur: "low" } });
    });
    expect(screen.queryByTestId("effects-motion-note")).toBeNull();
  });
});
