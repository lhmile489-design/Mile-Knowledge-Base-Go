import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { SkillsSubview } from "./SkillsSubview";

afterEach(cleanup);

describe("SkillsSubview (M8 T06)", () => {
  it("renders the breadcrumb, body and footer, with the current crumb marked", () => {
    render(
      <SkillsSubview testId="probe-view" crumbs={["全部技能", "「vue」", "分发到…"]} onBack={() => {}} footer={<span>底部</span>}>
        <p>内容</p>
      </SkillsSubview>,
    );
    expect(screen.getByTestId("probe-view")).toBeTruthy();
    const crumbs = screen.getByRole("navigation", { name: "位置" });
    expect(crumbs.textContent).toContain("全部技能");
    expect(crumbs.textContent).toContain("「vue」");
    expect(crumbs.textContent).toContain("分发到…");
    expect(crumbs.querySelector(".is-current")?.textContent).toBe("分发到…");
    expect(screen.getByText("内容")).toBeTruthy();
    expect(screen.getByText("底部")).toBeTruthy();
  });

  it("returns through the back button and can disable it while busy", () => {
    const onBack = vi.fn();
    const { rerender } = render(
      <SkillsSubview testId="probe-view" crumbs={["工具技能", "更新"]} onBack={onBack} footer={null}>
        <p>内容</p>
      </SkillsSubview>,
    );
    fireEvent.click(screen.getByRole("button", { name: "返回" }));
    expect(onBack).toHaveBeenCalledTimes(1);

    rerender(
      <SkillsSubview testId="probe-view" crumbs={["工具技能", "更新"]} backDisabled onBack={onBack} footer={null}>
        <p>内容</p>
      </SkillsSubview>,
    );
    expect((screen.getByRole("button", { name: "返回" }) as HTMLButtonElement).disabled).toBe(true);
  });
});
