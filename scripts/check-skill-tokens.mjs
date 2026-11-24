#!/usr/bin/env node
// V-SCOPE-1 (M8 T03): the skills-page pilot tokens may only appear in
// src/styles/skills-page.css, and only inside a `.skills-page` block
// (DESIGN 13.1/13.2). Plain line/brace scanner — enough for our stylesheets;
// it would need a real CSS parser before being reused elsewhere.
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const ROOT = process.cwd();
const SRC = join(ROOT, "src");
const ALLOWED = "src/styles/skills-page.css";
const TOKENS = [
  "--glass-card",
  "--glass-card-hover",
  "--glass-strong",
  "--glass-edge",
  "--glass-blur",
  "--field-a",
  "--field-b",
  "--field-c",
  "--ease-out-expo",
  "--ease-spring-soft",
  "--dur-enter",
  "--dur-slide",
  "--dur-hover",
  "--stagger",
];

function walk(dir) {
  const files = [];
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) files.push(...walk(path));
    else if (name.endsWith(".css") || name.endsWith(".tsx")) files.push(path);
  }
  return files;
}

function tokenHits(text, token) {
  const hits = [];
  let from = 0;
  for (;;) {
    const index = text.indexOf(token, from);
    if (index < 0) break;
    hits.push(index);
    from = index + token.length;
  }
  return hits;
}

function lineOf(text, index) {
  return text.slice(0, index).split("\n").length;
}

/** Scope at the start of each line, from a minimal brace scanner. */
function lineScopes(text) {
  const scopes = [];
  const stack = [];
  let pending = "";
  let line = 0;
  scopes.push([]);
  for (const ch of text) {
    if (ch === "\n") {
      line += 1;
      scopes[line] = stack.slice();
    } else if (ch === "{") {
      stack.push(pending.trim());
      pending = "";
    } else if (ch === "}") {
      stack.pop();
      pending = "";
    } else if (ch === ";") {
      pending = "";
    } else {
      pending += ch;
    }
  }
  return { scopes, lineSelector: text.split("\n").map((row) => row.split("{")[0]) };
}

const files = walk(SRC);
const problems = [];
let allowedHits = 0;

for (const file of files) {
  const rel = relative(ROOT, file).replaceAll("\\", "/");
  const text = readFileSync(file, "utf8").replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "));
  const { scopes, lineSelector } = rel === ALLOWED ? lineScopes(text) : { scopes: [], lineSelector: [] };
  for (const token of TOKENS) {
    for (const index of tokenHits(text, token)) {
      const line = lineOf(text, index); // 1-based
      if (rel !== ALLOWED) {
        problems.push(`${rel}:${line} ${token} only belongs in ${ALLOWED}`);
        continue;
      }
      allowedHits += 1;
      const atStart = (scopes[line - 1] ?? []).some((selector) => selector.includes(".skills-page"));
      const inline = (lineSelector[line - 1] ?? "").includes(".skills-page");
      if (!atStart && !inline) {
        problems.push(`${rel}:${line} ${token} must sit inside a .skills-page block`);
      }
    }
  }
}

if (problems.length > 0) {
  console.error("skills-page token scope: FAIL");
  for (const problem of problems) console.error(`  ${problem}`);
  process.exit(1);
}

console.log(`skills-page token scope: ok (${allowedHits} uses inside ${ALLOWED})`);
