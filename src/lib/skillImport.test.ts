import { describe, expect, it } from "vitest";
import { categoryProblem, importSummary, SKILL_NAME_MAX, skipLabel } from "./skillImport";

describe("categoryProblem", () => {
  it("accepts ordinary single-segment names", () => {
    expect(categoryProblem("前端")).toBeNull();
    expect(categoryProblem("  react  ")).toBeNull();
    expect(categoryProblem("a".repeat(SKILL_NAME_MAX))).toBeNull();
  });

  it("uses the legacy wording for the rejections", () => {
    expect(categoryProblem("")).toBe("分类名不能为空");
    expect(categoryProblem("   ")).toBe("分类名不能为空");
    expect(categoryProblem(".hidden")).toBe("分类名不能以点开头");
    expect(categoryProblem("a/b")).toBe('分类名不能包含 \\ / : * ? " < > |');
    expect(categoryProblem("a:b")).not.toBeNull();
    expect(categoryProblem("a".repeat(SKILL_NAME_MAX + 1))).toBe(
      `分类名不能超过 ${SKILL_NAME_MAX} 个字符`,
    );
  });
});

describe("skipLabel and summary", () => {
  it("maps the four legacy reasons", () => {
    expect(skipLabel("exists")).toBe("同名技能已存在，未覆盖");
    expect(skipLabel("unknown")).toBe("不在所选来源中");
    expect(skipLabel("too-large")).toBe("文件过多或过大");
    expect(skipLabel("failed")).toBe("复制失败");
    expect(skipLabel("weird")).toBe("weird");
  });

  it("summarises the result", () => {
    expect(importSummary(["a", "b"])).toBe("已导入 2 个技能");
    expect(importSummary([])).toBe("没有导入任何技能");
  });
});
