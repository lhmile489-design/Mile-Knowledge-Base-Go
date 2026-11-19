import { describe, expect, it } from "vitest";
import type { CategoryConfig, FileEntry } from "../mygo";
import {
  aliasOf,
  buildCategoryRows,
  defaultIndexedFor,
  isMeaningfulCategory,
  withIndexed,
  withoutDirectory,
} from "./categoriesTable";
import { categoryAliasDraftId } from "./settingsDrafts";

function entry(name: string, kind = "directory"): FileEntry {
  return { vaultId: "v", path: name, name, kind, area: "other" } as FileEntry;
}

function category(over: Partial<CategoryConfig> = {}): CategoryConfig {
  return { directory: "projects", alias: "", area: null, indexed: true, order: 0, ...over };
}

describe("buildCategoryRows", () => {
  it("lists real directories with defaults and appends leftovers", () => {
    const rows = buildCategoryRows(
      [entry("projects"), entry(".mile-kb"), entry("readme.md", "file")],
      [category({ directory: "Projects", alias: "项目", indexed: false }), category({ directory: "gone", alias: "旧" })],
    );
    expect(rows.map((row) => [row.directory, row.alias, row.indexed, row.present])).toEqual([
      ["projects", "项目", false, true],
      [".mile-kb", "", false, true],
      ["gone", "旧", true, false],
    ]);
  });

  it("only counts directories, never files", () => {
    const rows = buildCategoryRows([entry("a.md", "file")], []);
    expect(rows).toEqual([]);
  });
});

describe("withIndexed", () => {
  it("replaces the row case-insensitively and keeps the rest", () => {
    const next = withIndexed(
      [category({ directory: "Projects", alias: "项目" }), category({ directory: "raws" })],
      { directory: "projects", alias: "项目", area: null, indexed: false, order: 3, present: true },
      false,
    );
    expect(next).toHaveLength(2);
    expect(next[0]).toMatchObject({ directory: "raws" });
    expect(next[1]).toMatchObject({ directory: "projects", indexed: false, alias: "项目", order: 3 });
  });
});

describe("isMeaningfulCategory", () => {
  it("drops rows equal to the defaults so toggling back equals deleting", () => {
    expect(isMeaningfulCategory(category())).toBe(false);
    expect(isMeaningfulCategory(category({ indexed: false }))).toBe(true);
    expect(isMeaningfulCategory(category({ alias: "项目" }))).toBe(true);
    expect(isMeaningfulCategory(category({ area: "projects" }))).toBe(true);
    // 点号目录默认不进检索，把它拨到「没配过」时也不落盘。
    expect(isMeaningfulCategory(category({ directory: ".mile-kb", indexed: false }))).toBe(false);
    expect(isMeaningfulCategory(category({ directory: ".mile-kb", indexed: true }))).toBe(true);
  });

  it("keeps the default rule in one place", () => {
    expect(defaultIndexedFor("projects")).toBe(true);
    expect(defaultIndexedFor(".obsidian")).toBe(false);
  });
});

describe("withoutDirectory and aliasOf", () => {
  it("removes by case-insensitive name", () => {
    const left = withoutDirectory([category({ directory: "Projects" }), category({ directory: "raws" })], "projects");
    expect(left.map((item) => item.directory)).toEqual(["raws"]);
  });

  it("prefers the draft over the saved alias", () => {
    const row = { ...category({ alias: "已保存" }), present: true };
    expect(aliasOf(row, {}, categoryAliasDraftId)).toBe("已保存");
    expect(aliasOf(row, { [categoryAliasDraftId("projects")]: "草稿" }, categoryAliasDraftId)).toBe("草稿");
  });
});
