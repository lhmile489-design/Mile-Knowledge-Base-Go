import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";

const log = vi.fn();
const restore = vi.fn();
const readGithub = vi.fn();
const pickZip = vi.fn();
const discardStage = vi.fn();
const installPreview = vi.fn();
const install = vi.fn();

vi.mock("../../lib/transport", () => {
  class TransportError extends Error {
    readonly code: string;
    readonly detail?: string;
    constructor(code: string, message: string, detail?: string) {
      super(message);
      this.code = code;
      this.detail = detail;
    }
  }
  return {
    TransportError,
    skillsApi: {
      log: (...args: unknown[]) => log(...args),
      restore: (...args: unknown[]) => restore(...args),
      readGithub: (...args: unknown[]) => readGithub(...args),
      pickZip: (...args: unknown[]) => pickZip(...args),
      discardStage: (...args: unknown[]) => discardStage(...args),
      installPreview: (...args: unknown[]) => installPreview(...args),
      install: (...args: unknown[]) => install(...args),
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
const { TransportError } = await import("../../lib/transport");
import type { SkillStage, SkillTool } from "../../lib/transport";

const sha = "0123456789abcdef0123456789abcdef01234567";

const stage: SkillStage = {
  stageId: "s-0123456789ab",
  kind: "github" as const,
  origin: { owner: "anthropics", repo: "skills", ref: "main", path: "", commit: sha },
  label: "anthropics/skills",
  candidates: [
    { id: "skills/pdf", name: "pdf", description: "PDF 处理", files: 3, bytes: 2048, state: "ok" },
    { id: "skills/docx", name: "docx", description: "Word 文档", files: 2, bytes: 1024, state: "ok" },
    { id: "skills/old", name: "old", description: "", files: 1, bytes: 10, state: "exists" },
    { id: "skills/huge", name: "huge", description: "", files: 1, bytes: 900000, state: "too-large" },
  ],
  skipped: [{ path: "skills/link", reason: "symlink" }],
  expiresAt: "2026-11-21T10:30:00Z",
};

const tools: SkillTool[] = [
  {
    id: "agents",
    name: "Agents 共用目录",
    iconKey: "agents",
    installDir: "C:\\home\\.agents\\skills",
    scanDirs: [],
    readers: [],
    status: "shared",
    official: true,
    enabled: true,
  },
  {
    id: "claude-code",
    name: "Claude Code",
    iconKey: "claude-code",
    installDir: "C:\\home\\.claude\\skills",
    scanDirs: [],
    readers: [],
    status: "detected",
    official: true,
    enabled: true,
  },
  {
    id: "cursor",
    name: "Cursor",
    iconKey: "cursor",
    installDir: "",
    scanDirs: [],
    readers: [],
    status: "missing",
    official: true,
    enabled: true,
  },
];

function renderView(overrides: Partial<{ onBack: () => void; onChanged: () => void }> = {}) {
  const onBack = overrides.onBack ?? vi.fn();
  const onChanged = overrides.onChanged ?? vi.fn();
  render(<InstallView vaultId="v1" tools={tools} onBack={onBack} onChanged={onChanged} />);
  return { onBack, onChanged };
}

async function openGithubStage() {
  fireEvent.click(screen.getByTestId("install-github"));
  fireEvent.change(screen.getByTestId("install-github-input"), { target: { value: "anthropics/skills" } });
  fireEvent.click(screen.getByTestId("install-github-read"));
  await waitFor(() => expect(screen.getByTestId("install-stage")).toBeTruthy());
}

beforeEach(() => {
  for (const mock of [log, restore, readGithub, pickZip, discardStage, installPreview, install]) mock.mockReset();
  log.mockResolvedValue({ records: [] });
  discardStage.mockResolvedValue({});
  readGithub.mockResolvedValue({ stage });
  installPreview.mockResolvedValue({ items: [] });
  install.mockResolvedValue({
    items: [
      { targetId: "skills/pdf", toolId: "kb", result: "copied" },
      { targetId: "skills/docx", toolId: "kb", result: "copied" },
    ],
  });
});

afterEach(cleanup);

describe("InstallView sources", () => {
  it("shows the three sources and opens the GitHub form", () => {
    renderView();
    expect(screen.getByTestId("install-local")).toBeTruthy();
    expect((screen.getByTestId("install-github") as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByTestId("install-zip")).toBeTruthy();
    fireEvent.click(screen.getByTestId("install-github"));
    expect(screen.getByTestId("install-github-form")).toBeTruthy();
    expect(screen.getByTestId("install-github-input")).toBeTruthy();
  });

  it("asks for an address before reading", () => {
    renderView();
    fireEvent.click(screen.getByTestId("install-github"));
    fireEvent.click(screen.getByTestId("install-github-read"));
    expect(screen.getByTestId("install-input-problem").textContent).toContain("请粘贴");
    expect(readGithub).not.toHaveBeenCalled();
  });

  it("shows an inline problem for an invalid address", async () => {
    readGithub.mockRejectedValue(new TransportError("invalid-input", "只支持 github.com 的地址"));
    renderView();
    fireEvent.click(screen.getByTestId("install-github"));
    fireEvent.change(screen.getByTestId("install-github-input"), { target: { value: "https://example.com/a/b" } });
    fireEvent.click(screen.getByTestId("install-github-read"));
    await waitFor(() => expect(screen.getByTestId("install-input-problem").textContent).toContain("github.com"));
    expect(screen.getByTestId("install-github-form")).toBeTruthy();
  });

  it("opens the local-folder wizard and returns", () => {
    renderView();
    fireEvent.click(screen.getByTestId("install-local"));
    expect(screen.getByTestId("skill-wizard")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "关闭导入向导" }));
    expect(screen.queryByTestId("skill-wizard")).toBeNull();
    expect(screen.getByTestId("install-local")).toBeTruthy();
  });
});

describe("InstallView candidates", () => {
  it("lists candidates, keeps unusable ones disabled and installs into the vault", async () => {
    const { onChanged } = renderView();
    await openGithubStage();

    expect(screen.getByTestId("install-origin").textContent).toContain("anthropics/skills");
    expect(screen.getByTestId("install-origin").textContent).toContain("分支 main");
    expect(screen.getByTestId("install-skipped").textContent).toContain("跳过 1 个链接");
    expect((screen.getByTestId("install-candidate-skills/pdf") as HTMLInputElement).checked).toBe(true);
    expect((screen.getByTestId("install-candidate-skills/old") as HTMLInputElement).disabled).toBe(true);
    expect((screen.getByTestId("install-candidate-skills/huge") as HTMLInputElement).disabled).toBe(true);

    fireEvent.change(screen.getByTestId("install-category"), { target: { value: "文档" } });
    fireEvent.click(screen.getByTestId("install-submit"));
    await waitFor(() => expect(install).toHaveBeenCalled());
    expect(install.mock.calls[0]).toEqual([
      "v1",
      "s-0123456789ab",
      ["skills/pdf", "skills/docx"],
      { kind: "kb", category: "文档" },
    ]);
    await waitFor(() => expect(screen.getByTestId("install-result").textContent).toContain("pdf → 知识库：已安装"));
    expect(onChanged).toHaveBeenCalled();
    expect(screen.queryByTestId("install-stage")).toBeNull();
  });

  it("disables the install button until a category is usable", async () => {
    renderView();
    await openGithubStage();
    expect((screen.getByTestId("install-submit") as HTMLButtonElement).disabled).toBe(true);
    expect(screen.getByTestId("install-summary").textContent).toContain("先填分类再安装");
    fireEvent.change(screen.getByTestId("install-category"), { target: { value: "a/b" } });
    expect((screen.getByTestId("install-submit") as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(screen.getByTestId("install-category"), { target: { value: "文档" } });
    expect((screen.getByTestId("install-submit") as HTMLButtonElement).disabled).toBe(false);
    expect(screen.getByTestId("install-summary").textContent).toContain("写入知识库 2 个目录");
  });

  it("installs into a tool with overwrite and previews the existing target", async () => {
    installPreview.mockResolvedValue({
      items: [{ targetId: "claude-code", toolId: "claude-code", name: "pdf", action: "exists", detail: "目标已存在" }],
    });
    install.mockResolvedValue({ items: [{ targetId: "skills/pdf", toolId: "claude-code", result: "copied" }] });
    renderView();
    await openGithubStage();

    fireEvent.click(screen.getByTestId("install-target-tools"));
    await waitFor(() => expect(screen.getByTestId("install-tool-targets")).toBeTruthy());
    expect(screen.getByTestId("install-tool-claude-code").textContent).toContain("目标已存在");
    expect(screen.queryByTestId("install-tool-cursor")).toBeNull();

    const tile = screen.getByTestId("install-tool-claude-code");
    const overwriteBox = tile.querySelector<HTMLInputElement>('input[type="checkbox"]');
    expect(overwriteBox).toBeTruthy();
    fireEvent.click(overwriteBox as HTMLInputElement);
    await waitFor(() => expect(installPreview.mock.calls.length).toBeGreaterThan(1));
    fireEvent.click(screen.getByTestId("install-submit"));
    await waitFor(() => expect(install).toHaveBeenCalled());
    expect(install.mock.calls[0]?.[3]).toEqual({
      kind: "tools",
      targets: [{ targetId: "claude-code", allowOverwrite: true }],
    });
  });

  it("drops the stage when the user backs out", async () => {
    renderView();
    await openGithubStage();
    fireEvent.click(screen.getByTestId("install-stage-back"));
    await waitFor(() => expect(discardStage).toHaveBeenCalledWith("s-0123456789ab"));
    expect(screen.getByTestId("install-github")).toBeTruthy();
  });
});

describe("InstallView errors and progress", () => {
  it("shows the archive rejection details", async () => {
    pickZip.mockRejectedValue(
      new TransportError("unsafe-archive", "压缩包里有不安全的路径，整包已拒绝", "../evil/SKILL.md"),
    );
    renderView();
    fireEvent.click(screen.getByTestId("install-zip"));
    await waitFor(() => expect(screen.getByTestId("install-error")).toBeTruthy());
    expect(screen.getByTestId("install-error").textContent).toContain("../evil/SKILL.md");
    expect(screen.getByTestId("install-error").textContent).toContain("没有写入任何文件");
  });

  it("cancels a GitHub read and returns to the form", async () => {
    readGithub.mockImplementation(
      (_vault: string, _input: string, signal?: AbortSignal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
        }),
    );
    renderView();
    fireEvent.click(screen.getByTestId("install-github"));
    fireEvent.change(screen.getByTestId("install-github-input"), { target: { value: "anthropics/skills" } });
    fireEvent.click(screen.getByTestId("install-github-read"));
    await waitFor(() => expect(screen.getByTestId("install-progress")).toBeTruthy());
    fireEvent.click(screen.getByTestId("install-cancel-read"));
    await waitFor(() => expect(screen.getByTestId("install-notice").textContent).toContain("已取消"));
    expect(screen.getByTestId("install-github-form")).toBeTruthy();
  });

  it("reads a zip and shows its stage", async () => {
    pickZip.mockResolvedValue({ stage: { ...stage, kind: "zip", origin: undefined, label: "company-skills.zip" } });
    renderView();
    fireEvent.click(screen.getByTestId("install-zip"));
    await waitFor(() => expect(screen.getByTestId("install-stage")).toBeTruthy());
    expect(screen.getByTestId("install-origin").textContent).toContain("company-skills.zip");
  });
});
