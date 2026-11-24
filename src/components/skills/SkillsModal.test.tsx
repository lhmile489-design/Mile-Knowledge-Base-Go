import { useState, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SkillsModal } from "./SkillsModal";

afterEach(cleanup);

type ModalProps = Partial<Parameters<typeof SkillsModal>[0]>;

/** The modal lives inside the skills page panel; the sibling regions (nav,
 * main) must go inert while it is open and the opener gets focus back. */
function Harness({ initiallyOpen = true, children, ...props }: ModalProps & { initiallyOpen?: boolean; children?: ReactNode }) {
  const [open, setOpen] = useState(initiallyOpen);
  return (
    <div className="skills-page" data-testid="page">
      <nav data-testid="probe-nav">
        <button type="button">导航按钮</button>
      </nav>
      <main data-testid="probe-main">
        <button type="button" data-testid="opener" onClick={() => setOpen(true)}>
          打开
        </button>
      </main>
      {open ? (
        <SkillsModal
          title="安装技能"
          step="选择要安装的技能"
          footer={<button type="button">底栏按钮</button>}
          onClose={() => {
            props.onClose?.();
            setOpen(false);
          }}
          {...props}
        >
          {children ?? <input aria-label="地址" />}
        </SkillsModal>
      ) : null}
    </div>
  );
}

const dialog = () => screen.getByRole("dialog");

describe("SkillsModal (M9 T01)", () => {
  it("renders an accessible dialog with title, step, head slot, body and footer", () => {
    render(
      <Harness head={<div data-testid="probe-head">来源分段</div>}>
        <p>正文内容</p>
      </Harness>,
    );
    expect(screen.getByTestId("skills-modal")).toBeTruthy();
    const card = dialog();
    expect(card.getAttribute("aria-modal")).toBe("true");
    const labelledBy = card.getAttribute("aria-labelledby") ?? "";
    expect(document.getElementById(labelledBy)?.textContent).toBe("安装技能");
    expect(screen.getByText("选择要安装的技能")).toBeTruthy();
    expect(screen.getByTestId("probe-head")).toBeTruthy();
    expect(screen.getByText("正文内容")).toBeTruthy();
    expect(screen.getByRole("button", { name: "底栏按钮" })).toBeTruthy();
    expect(screen.getByTestId("skills-modal-close")).toBeTruthy();
  });

  it("focuses the first input on open, else the first focusable control in the body", () => {
    const { unmount } = render(<Harness />);
    expect(document.activeElement).toBe(screen.getByLabelText("地址"));
    unmount();

    render(
      <Harness>
        <button type="button">正文按钮</button>
      </Harness>,
    );
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "正文按钮" }));
  });

  it("makes the rest of the page inert while open and restores it on close", () => {
    render(<Harness />);
    expect(screen.getByTestId("probe-nav").hasAttribute("inert")).toBe(true);
    expect(screen.getByTestId("probe-main").hasAttribute("inert")).toBe(true);
    expect(screen.getByTestId("skills-modal").hasAttribute("inert")).toBe(false);

    fireEvent.click(screen.getByTestId("skills-modal-close"));
    expect(screen.queryByTestId("skills-modal")).toBeNull();
    expect(screen.getByTestId("probe-nav").hasAttribute("inert")).toBe(false);
    expect(screen.getByTestId("probe-main").hasAttribute("inert")).toBe(false);
  });

  it("does not clear an inert attribute somebody else set", () => {
    const { unmount } = render(<Harness initiallyOpen={false} />);
    const nav = screen.getByTestId("probe-nav");
    nav.setAttribute("inert", "");
    fireEvent.click(screen.getByTestId("opener"));
    fireEvent.click(screen.getByTestId("skills-modal-close"));
    expect(nav.hasAttribute("inert")).toBe(true);
    unmount();
  });

  it("returns focus to the opener after closing", () => {
    render(<Harness initiallyOpen={false} />);
    const opener = screen.getByTestId("opener");
    opener.focus();
    fireEvent.click(opener);
    expect(document.activeElement).toBe(screen.getByLabelText("地址"));
    fireEvent.click(screen.getByTestId("skills-modal-close"));
    expect(document.activeElement).toBe(opener);
  });

  it("keeps Tab and Shift+Tab inside the dialog", () => {
    render(<Harness />);
    const first = screen.getByTestId("skills-modal-close");
    const middle = screen.getByLabelText("地址");
    const last = screen.getByRole("button", { name: "底栏按钮" });

    last.focus();
    expect(fireEvent.keyDown(last, { key: "Tab" })).toBe(false);
    expect(document.activeElement).toBe(first);

    expect(fireEvent.keyDown(first, { key: "Tab", shiftKey: true })).toBe(false);
    expect(document.activeElement).toBe(last);

    // Middle of the dialog: the browser moves focus, the trap stays out of it.
    middle.focus();
    expect(fireEvent.keyDown(middle, { key: "Tab" })).toBe(true);
  });

  it("pulls stray focus back into the dialog on Tab", () => {
    render(<Harness />);
    (document.activeElement as HTMLElement | null)?.blur();
    expect(document.activeElement).toBe(document.body);
    fireEvent.keyDown(document.body, { key: "Tab" });
    expect(dialog().contains(document.activeElement)).toBe(true);
  });

  it("gives Esc to onBack first and closes only when onBack did not handle it", () => {
    const onClose = vi.fn();
    const onBack = vi.fn().mockReturnValueOnce(true).mockReturnValueOnce(false);
    render(<Harness onClose={onClose} onBack={onBack} />);

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onBack).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(document, { key: "Escape" });
    expect(onBack).toHaveBeenCalledTimes(2);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("closes on Esc without onBack and keeps the key away from the page handlers", () => {
    const onClose = vi.fn();
    const pageHandler = vi.fn();
    document.addEventListener("keydown", pageHandler);
    render(<Harness onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(pageHandler).not.toHaveBeenCalled();
    document.removeEventListener("keydown", pageHandler);
  });

  it("ignores Esc while an input method is composing", () => {
    const onClose = vi.fn();
    render(<Harness onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape", isComposing: true });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes from the close button and from a click that starts and ends on the scrim", () => {
    const onClose = vi.fn();
    const { unmount } = render(<Harness onClose={onClose} />);
    fireEvent.click(screen.getByTestId("skills-modal-close"));
    expect(onClose).toHaveBeenCalledTimes(1);
    unmount();

    const again = vi.fn();
    render(<Harness onClose={again} />);
    const scrim = screen.getByTestId("skills-modal");
    // A click inside the card never closes it.
    fireEvent.mouseDown(dialog());
    fireEvent.click(dialog());
    expect(again).not.toHaveBeenCalled();
    // A drag that started inside the card and ended on the scrim does not either.
    fireEvent.mouseDown(dialog());
    fireEvent.click(scrim);
    expect(again).not.toHaveBeenCalled();
    fireEvent.mouseDown(scrim);
    fireEvent.click(scrim);
    expect(again).toHaveBeenCalledTimes(1);
  });

  it("cannot be closed while busy: Esc, close button and scrim are all inert", () => {
    const onClose = vi.fn();
    const onBack = vi.fn();
    const pageHandler = vi.fn();
    document.addEventListener("keydown", pageHandler);
    render(<Harness busy onClose={onClose} onBack={onBack} />);

    const close = screen.getByTestId("skills-modal-close");
    expect(close.getAttribute("aria-disabled")).toBe("true");
    expect(close.getAttribute("title")).toContain("完成后才能关闭");

    fireEvent.keyDown(document, { key: "Escape" });
    fireEvent.click(close);
    const scrim = screen.getByTestId("skills-modal");
    fireEvent.mouseDown(scrim);
    fireEvent.click(scrim);

    expect(onClose).not.toHaveBeenCalled();
    expect(onBack).not.toHaveBeenCalled();
    // The page must not see Esc either (it would navigate away mid-install).
    expect(pageHandler).not.toHaveBeenCalled();
    expect(screen.getByTestId("skills-modal")).toBeTruthy();
    document.removeEventListener("keydown", pageHandler);
  });

  it("moves focus to the new first control when focusKey changes, and not otherwise", () => {
    const { rerender } = render(
      <Harness focusKey="a">
        <input aria-label="第一步输入" />
      </Harness>,
    );
    expect(document.activeElement).toBe(screen.getByLabelText("第一步输入"));

    // Same key: a re-render never steals focus the user moved.
    const footerButton = screen.getByRole("button", { name: "底栏按钮" });
    footerButton.focus();
    rerender(
      <Harness focusKey="a">
        <input aria-label="第一步输入" />
      </Harness>,
    );
    expect(document.activeElement).toBe(footerButton);

    // New key: the old input is gone and focus lands on the new step's input.
    rerender(
      <Harness focusKey="b">
        <input aria-label="第二步输入" />
      </Harness>,
    );
    expect(document.activeElement).toBe(screen.getByLabelText("第二步输入"));
  });

  it("becomes closable again once busy ends", () => {
    const onClose = vi.fn();
    const { rerender } = render(<Harness busy onClose={onClose} />);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();

    rerender(<Harness busy={false} onClose={onClose} />);
    const close = screen.getByTestId("skills-modal-close");
    expect(close.hasAttribute("aria-disabled")).toBe(false);
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
