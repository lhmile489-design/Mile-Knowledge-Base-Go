// Categories & search table (M3 T06). Rows come from the vault's real
// top-level directories plus config entries whose directory is gone, so a
// stale entry can be seen and cleared instead of lingering silently.
import type { CategoryConfig, FileEntry } from "../mygo";

/** One table row: a present directory or a leftover configuration. */
export interface CategoryRow extends CategoryConfig {
  readonly present: boolean;
}

/** The indexed value a directory has with no configuration (数据库设计 3.2). */
export function defaultIndexedFor(directory: string): boolean {
  return !directory.startsWith(".");
}

/** An entry equal to the defaults is not persisted (legacy isMeaningfulCategory). */
export function isMeaningfulCategory(category: CategoryConfig): boolean {
  return (
    category.alias !== "" ||
    category.area !== null ||
    category.indexed !== defaultIndexedFor(category.directory)
  );
}

/** Build the table: real directories first (in tree order), leftovers after. */
export function buildCategoryRows(
  entries: readonly FileEntry[],
  categories: readonly CategoryConfig[],
): CategoryRow[] {
  const configured = new Map(
    categories.map((category) => [category.directory.toLowerCase(), category]),
  );
  const directories = entries.filter((entry) => entry.kind === "directory");
  const rows: CategoryRow[] = directories.map((entry, index) => {
    const existing = configured.get(entry.name.toLowerCase());
    return {
      directory: entry.name,
      alias: existing?.alias ?? "",
      area: existing?.area ?? null,
      indexed: existing?.indexed ?? defaultIndexedFor(entry.name),
      order: existing?.order ?? index,
      present: true,
    };
  });
  const present = new Set(directories.map((entry) => entry.name.toLowerCase()));
  for (const category of categories) {
    if (present.has(category.directory.toLowerCase())) continue;
    rows.push({ ...category, present: false });
  }
  return rows;
}

/** Replace one directory's row with a new indexed value (case-insensitively). */
export function withIndexed(
  categories: readonly CategoryConfig[],
  row: CategoryRow,
  indexed: boolean,
): CategoryConfig[] {
  const rest = categories.filter(
    (category) => category.directory.toLowerCase() !== row.directory.toLowerCase(),
  );
  return [
    ...rest,
    { directory: row.directory, alias: row.alias, area: row.area, indexed, order: row.order },
  ];
}

/** Drop one directory's configuration entirely. */
export function withoutDirectory(
  categories: readonly CategoryConfig[],
  directory: string,
): CategoryConfig[] {
  return categories.filter(
    (category) => category.directory.toLowerCase() !== directory.toLowerCase(),
  );
}

/** The display name shown in the input: draft first, then the saved value. */
export function aliasOf(
  row: CategoryRow,
  drafts: Record<string, string>,
  draftId: (directory: string) => string,
): string {
  return drafts[draftId(row.directory)] ?? row.alias;
}
