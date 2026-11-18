/**
 * Search display helpers: tokenising (shared口径 with the Go index), field
 * labels and highlight ranges.
 *
 * Ported from the legacy renderer sources `src/shared/tokenize.ts` and the
 * highlight half of `src/shared/search.ts` (v2.7.0). The Go index
 * (`internal/index`) is the authority for what a token is; this copy exists
 * because highlighting must align to exactly the same tokens, and the only way
 * to guarantee that is to run the same algorithm on the same text. The parity
 * fixture in `internal/index/testdata/tokens.json` is checked from both sides.
 */

/** Upper bound on a single tokenize call; absurd input is truncated. */
const MAX_TOKENIZE_LENGTH = 200_000;

/** CJK ideographs (extension A, unified, compatibility) and kana. */
const CJK = /[぀-ヿ㐀-䶿一-鿿豈-﫿]/;

/** Surrogate code units are never CJK: the legacy class happens to include
 * U+D800..U+DFFF (it ends at U+FAFF), which is how half an emoji used to turn
 * into a "CJK" token. */
function isSurrogate(char: string): boolean {
  const code = char.charCodeAt(0);
  return code >= 0xd800 && code <= 0xdfff;
}

function isCjk(char: string): boolean {
  return !isSurrogate(char) && CJK.test(char);
}

/** Characters that make up an ASCII word; `.`/`-`/`+`/`#` stay inside one. */
function isWordChar(char: string): boolean {
  return /[A-Za-z0-9_]/.test(char);
}

function isInnerWordChar(char: string): boolean {
  return char === '.' || char === '-' || char === '+' || char === '#';
}

/**
 * Lowercase tokens, deduplicated, in order of first appearance.
 *
 * Deliberate difference from the legacy implementation: it walked UTF-16 code
 * units, so an astral character (an emoji) became two lone surrogates and could
 * produce junk tokens built from half a character. The Go index works on code
 * points, so this port does too; the two tokenizers stay identical (see
 * tokenParity.test.ts) and no token can carry a broken surrogate.
 */
export function tokenize(input: string): string[] {
  if (typeof input !== 'string' || input.length === 0) return [];
  const text = input.length > MAX_TOKENIZE_LENGTH ? input.slice(0, MAX_TOKENIZE_LENGTH) : input;

  const chars = Array.from(text);
  const out = new Set<string>();
  let i = 0;

  while (i < chars.length) {
    const char = chars[i] as string;

    if (isCjk(char)) {
      let j = i;
      while (j < chars.length && isCjk(chars[j] as string)) j += 1;
      // Bigram over code points, never across the run boundary.
      if (j - i === 1) {
        out.add(char);
      } else {
        for (let k = i; k + 1 < j; k += 1) {
          out.add((chars[k] as string) + (chars[k + 1] as string));
        }
      }
      i = j;
      continue;
    }

    if (isWordChar(char)) {
      let j = i;
      while (j < chars.length) {
        const current = chars[j] as string;
        if (isWordChar(current)) {
          j += 1;
          continue;
        }
        if (isInnerWordChar(current) && j + 1 < chars.length && isWordChar(chars[j + 1] as string)) {
          j += 1;
          continue;
        }
        break;
      }
      out.add(chars.slice(i, j).join('').toLowerCase());
      i = j;
      continue;
    }

    i += 1;
  }

  return [...out];
}

/** The match fields the index reports, in display order. */
export const MATCH_FIELDS = ['title', 'heading', 'tag', 'path', 'body'] as const;
export type MatchField = (typeof MATCH_FIELDS)[number];

/** User-facing labels; internal identifiers never reach the UI. */
export const MATCH_FIELD_LABEL: Record<MatchField, string> = {
  title: '标题',
  heading: '标题段',
  tag: '标签',
  path: '路径',
  body: '正文',
};

/** `['title','body']` → `标题、正文`. */
export function describeMatchedFields(fields: readonly MatchField[]): string {
  return fields.map((field) => MATCH_FIELD_LABEL[field]).join('、');
}

/** Half-open range `[start, end)` in UTF-16 units, i.e. JS string indices. */
export interface HighlightRange {
  readonly start: number;
  readonly end: number;
}

/**
 * Literal, case-insensitive occurrences of the query tokens. Overlapping or
 * adjacent ranges are merged; the query is never compiled to a regex.
 */
export function highlightRanges(text: string, queryTokens: readonly string[]): HighlightRange[] {
  if (text.length === 0 || queryTokens.length === 0) return [];
  const haystack = text.toLowerCase();
  const raw: HighlightRange[] = [];
  for (const token of queryTokens) {
    const needle = token.toLowerCase();
    if (needle.length === 0) continue;
    let from = 0;
    for (;;) {
      const at = haystack.indexOf(needle, from);
      if (at === -1) break;
      raw.push({ start: at, end: at + needle.length });
      from = at + needle.length;
    }
  }
  if (raw.length === 0) return [];

  raw.sort((a, b) => a.start - b.start || a.end - b.end);
  const merged: HighlightRange[] = [];
  for (const range of raw) {
    const last = merged[merged.length - 1];
    if (last && range.start <= last.end) {
      if (range.end > last.end) {
        merged[merged.length - 1] = { start: last.start, end: range.end };
      }
      continue;
    }
    merged.push(range);
  }
  return merged;
}
