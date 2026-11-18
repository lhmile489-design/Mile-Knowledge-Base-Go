/**
 * Turns a wikilink target into the path of the document to open or create
 * (legacy shared/wikiNavigation.ts newNotePathFor): a target with `/` is
 * relative to the vault root, otherwise it sits next to the current document;
 * a missing extension becomes `.md`; illegal targets return null.
 */
const ILLEGAL = /[<>:"|?*\\]/;

export function newNotePath(target: string, currentPath: string): string | null {
  const clean = target.split("?")[0]!.split("#")[0]!.trim().replace(/\\/g, "/");
  if (clean === "" || clean.startsWith("/") || /^[A-Za-z]:/.test(clean)) return null;
  const segments = clean.split("/");
  for (const segment of segments) {
    if (segment === "" || segment === "." || segment === "..") return null;
    if (ILLEGAL.test(segment) || segment.endsWith(".") || segment.endsWith(" ")) return null;
    if (segment.length > 200) return null;
  }
  const dir = currentPath.includes("/") ? currentPath.slice(0, currentPath.lastIndexOf("/")) : "";
  const joined = clean.includes("/") || dir === "" ? clean : `${dir}/${clean}`;
  return /\.[A-Za-z0-9]+$/.test(joined) ? joined : `${joined}.md`;
}

/** The initial text of a note created from a wikilink. */
export function newNoteText(path: string): string {
  const name = path.slice(path.lastIndexOf("/") + 1).replace(/\.[A-Za-z0-9]+$/, "");
  return `# ${name}\n`;
}

/** Finds the `[[target]]` or `[[target|alias]]` containing a document offset. */
export function wikilinkAt(text: string, pos: number): string | null {
  const start = text.lastIndexOf("[[", pos);
  if (start < 0) return null;
  const end = text.indexOf("]]", start + 2);
  if (end < 0 || end < pos) return null;
  const inner = text.slice(start + 2, end);
  if (inner.includes("[") || inner.includes("]")) return null;
  const pipe = inner.indexOf("|");
  const target = (pipe >= 0 ? inner.slice(0, pipe) : inner).trim();
  return target === "" ? null : target;
}
