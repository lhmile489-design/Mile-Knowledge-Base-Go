// In-vault reference recognition for chat answers (ported from
// chatReferences.ts). Syntax only: a match never proves the file exists —
// existence is decided by the click-time read.
export type RefKind = "path" | "wiki";

export interface RefMatch {
  readonly start: number;
  readonly end: number;
  readonly refKind: RefKind;
  /** path: vault-relative path; wiki: the target left of "|". */
  readonly target: string;
  readonly label: string;
}

const REF_EXTENSIONS = ["md", "markdown", "txt"];

/** Rejects absolute paths, drive letters, backslashes and dot segments. */
export function isPlausibleRelativePath(value: string): boolean {
  if (value === "" || value.startsWith("/") || value.startsWith("\\")) return false;
  if (value.includes("\\")) return false;
  if (/^[A-Za-z]:/.test(value)) return false;
  return !value.split("/").some((segment) => segment === "." || segment === "..");
}

const PATH_TOKEN = new RegExp(
  String.raw`(?<![\w/.\\:@-])((?:[\w㐀-鿿.-]+/)*[\w㐀-鿿.-]+\.(?:${REF_EXTENSIONS.join("|")}))(?![\w/-])`,
  "gi",
);

const WIKI_TOKEN = /\[\[([^[\]|\n]+)(?:\|([^[\]\n]+))?\]\]/g;

/** Whole text is one vault path (inline code case). */
export function asPathReference(text: string): string | null {
  const candidate = text.trim();
  if (candidate === "" || candidate !== text) return null;
  const exact = new RegExp(`^${PATH_TOKEN.source}$`, "i");
  if (!exact.test(candidate)) return null;
  return isPlausibleRelativePath(candidate) ? candidate : null;
}

/** All references in one plain-text run, in order and non-overlapping. */
export function findReferences(text: string): RefMatch[] {
  const found: RefMatch[] = [];
  for (const match of text.matchAll(new RegExp(WIKI_TOKEN.source, "g"))) {
    const target = (match[1] ?? "").trim();
    if (target === "") continue;
    const start = match.index ?? 0;
    found.push({
      start,
      end: start + match[0].length,
      refKind: "wiki",
      target,
      label: (match[2] ?? target).trim() || target,
    });
  }
  for (const match of text.matchAll(new RegExp(PATH_TOKEN.source, "gi"))) {
    const start = match.index ?? 0;
    const end = start + match[0].length;
    if (found.some((hit) => start < hit.end && end > hit.start)) continue;
    if (!match[1] || !isPlausibleRelativePath(match[1])) continue;
    found.push({ start, end, refKind: "path", target: match[1], label: match[1] });
  }
  return found.sort((a, b) => a.start - b.start);
}

/** One evidence entry: real references, real counts, no invented scores. */
export interface EvidenceItem {
  readonly refKind: RefKind;
  readonly target: string;
  readonly label: string;
  readonly count: number;
  readonly index: number;
}

/** Collects references from agent lines only (user pastes are not evidence). */
export function collectEvidence(
  lines: readonly { readonly role: string; readonly text: string }[],
): EvidenceItem[] {
  const order: string[] = [];
  const byKey = new Map<string, { refKind: RefKind; target: string; label: string; count: number }>();
  for (const line of lines) {
    if (line.role !== "agent") continue;
    for (const ref of findReferences(line.text)) {
      const key = `${ref.refKind}:${ref.target}`;
      const existing = byKey.get(key);
      if (existing) existing.count += 1;
      else {
        byKey.set(key, { refKind: ref.refKind, target: ref.target, label: ref.label, count: 1 });
        order.push(key);
      }
    }
  }
  return order.map((key, i) => {
    const item = byKey.get(key)!;
    return { ...item, index: i + 1 };
  });
}
