#!/usr/bin/env node
// V-SCOPE-2 (M10 T02, 技术设计 21.3): the global material tokens may only be
// DEFINED in src/styles/effects.css; every other file only references them.
// Component styles must not hardcode a backdrop-filter blur value or a
// duration above 240 ms (outside @supports feature queries), otherwise the
// user's blur / motion levels would silently stop applying.
//
// Plain line/brace scanner, same trade-off as the M8 checker it replaces:
// enough for our stylesheets, and it would need a real CSS parser before
// being reused elsewhere.
//
// Usage:
//   node scripts/check-effect-tokens.mjs             check the real tree
//   node scripts/check-effect-tokens.mjs --self-test run the built-in cases
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const ROOT = process.cwd();
const SRC = join(ROOT, "src");
const DEFINITIONS = "src/styles/effects.css";
const MAX_LITERAL_MS = 240;
const TOKENS = [
  "--glass-blur",
  "--glass-blur-strong",
  "--glass-blur-foot",
  "--blur-scrim",
  "--blur-scrim-soft",
  "--glass-sat",
  "--glass-panel",
  "--glass-card",
  "--glass-card-hover",
  "--glass-strong",
  "--glass-edge",
  "--sheet",
  "--field-k",
  "--field-a",
  "--field-b",
  "--field-c",
  "--dur-enter",
  "--dur-slide",
  "--dur-hover",
  "--stagger",
  "--lift",
  "--enter-blur",
  "--enter-y",
  "--ease-out-expo",
  "--ease-spring-soft",
];

/** Blanks out comments while keeping line numbers intact. */
function stripComments(text) {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "))
    .replace(/(^|[^:])\/\/[^\n]*/g, (line) => line.replace(/[^\n]/g, " "));
}

function lineOf(text, index) {
  return text.slice(0, index).split("\n").length;
}

/** Problems for one file's text; exported shape reused by the self-test. */
export function inspect(rel, rawText) {
  const text = stripComments(rawText);
  const problems = [];
  let definitions = 0;
  let references = 0;

  for (const token of TOKENS) {
    let from = 0;
    for (;;) {
      const index = text.indexOf(token, from);
      if (index < 0) break;
      from = index + token.length;
      const rest = text.slice(from);
      if (/^\s*:/.test(rest)) {
        definitions += 1;
        if (rel !== DEFINITIONS) {
          problems.push(`${rel}:${lineOf(text, index)} ${token} must only be defined in ${DEFINITIONS}`);
        }
      } else {
        references += 1;
      }
    }
  }

  const blur = /backdrop-filter\s*:\s*[^;{}]*blur\(\s*[0-9.]+/g;
  for (const match of text.matchAll(blur)) {
    const line = lineOf(text, match.index ?? 0);
    const row = text.split("\n")[line - 1] ?? "";
    if (row.includes("@supports")) continue; // feature query, not a style value
    problems.push(`${rel}:${line} hardcoded backdrop blur; use the tier tokens`);
  }

  // effects.css is where the duration tokens are defined, so its literals are
  // the source of truth and are exempt from the duration rule.
  if (rel !== DEFINITIONS) {
    const durations = /\b(\d{2,4})ms\b/g;
    for (const match of text.matchAll(durations)) {
      if (Number(match[1]) <= MAX_LITERAL_MS) continue;
      const line = lineOf(text, match.index ?? 0);
      problems.push(`${rel}:${line} hardcoded ${match[1]}ms; durations above ${MAX_LITERAL_MS}ms must use --dur-* tokens`);
    }
  }

  return { problems, definitions, references };
}

function walk(dir) {
  const files = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) files.push(...walk(path));
    else if (name.endsWith(".css") || name.endsWith(".tsx")) files.push(path);
  }
  return files;
}

function checkTree() {
  const problems = [];
  let definitions = 0;
  let references = 0;
  for (const file of walk(SRC)) {
    const rel = relative(ROOT, file).replaceAll("\\", "/");
    const result = inspect(rel, readFileSync(file, "utf8"));
    problems.push(...result.problems);
    definitions += result.definitions;
    references += result.references;
  }
  if (problems.length > 0) {
    console.error("effect token check: FAIL");
    for (const problem of problems) console.error(`  ${problem}`);
    return false;
  }
  console.log(`effect token check: ok (${definitions} definitions in ${DEFINITIONS}, ${references} references)`);
  return true;
}

function selfTest() {
  const cases = [
    ["definition in effects.css", DEFINITIONS, ":root { --glass-blur: 18px; }", true],
    ["definition elsewhere", "src/styles/x.css", ".a { --glass-blur: 18px; }", false],
    ["reference elsewhere", "src/styles/x.css", ".a { backdrop-filter: blur(var(--glass-blur)); }", true],
    ["hardcoded blur", "src/styles/x.css", ".a { backdrop-filter: blur(18px); }", false],
    ["hardcoded webkit blur", "src/styles/x.css", ".a { -webkit-backdrop-filter: blur(8px); }", false],
    ["feature query", "src/styles/x.css", "@supports not (backdrop-filter: blur(2px)) { .a { color: red; } }", true],
    ["long duration", "src/styles/x.css", ".a { transition: opacity 520ms; }", false],
    ["long duration in effects.css is a token", DEFINITIONS, ":root { --dur-enter: 520ms; }", true],
    ["duration token", "src/styles/x.css", ".a { transition: opacity var(--dur-enter); }", true],
    ["240ms literal allowed", "src/styles/x.css", ".a { transition: opacity 240ms; }", true],
    ["comment ignored", "src/styles/x.css", "/* blur(9px) 520ms --glass-blur: 1px */", true],
  ];
  let failed = 0;
  for (const [name, rel, text, ok] of cases) {
    const { problems } = inspect(rel, text);
    const got = problems.length === 0;
    if (got !== ok) {
      failed += 1;
      console.error(`self-test FAIL: ${name} (expected ${ok ? "ok" : "problem"}, got ${got ? "ok" : problems.join("; ")})`);
    }
  }
  if (failed > 0) return false;
  console.log(`effect token self-test: ok (${cases.length} cases)`);
  return true;
}

const ok = process.argv.includes("--self-test") ? selfTest() : checkTree() && selfTest();
process.exit(ok ? 0 : 1);
