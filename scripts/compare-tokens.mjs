// Compares the token declarations of the legacy tokens.css with the ported one.
// The port is verbatim in M0, so any difference means someone edited values by
// hand instead of going through DESIGN.md. Run: node scripts/compare-tokens.mjs
import { readFileSync } from "node:fs";

const LEGACY =
  "E:/WebApp/Mile-Project/Mile-Knowledge-Base-master/src/renderer/src/assets/tokens.css";
const PORTED = "src/styles/tokens.css";

/** @param {string} text */
function declarations(text) {
  /** @type {Map<string, string[]>} */
  const out = new Map();
  const re = /(--[a-z0-9-]+)\s*:\s*([^;]+);/gi;
  let m;
  while ((m = re.exec(text)) !== null) {
    const name = m[1];
    const value = m[2].replace(/\s+/g, " ").trim();
    const list = out.get(name) ?? [];
    list.push(value);
    out.set(name, list);
  }
  return out;
}

const legacy = declarations(readFileSync(LEGACY, "utf8"));
const ported = declarations(readFileSync(PORTED, "utf8"));

let failures = 0;
for (const [name, values] of legacy) {
  const other = ported.get(name);
  if (!other) {
    console.error(`missing in port: ${name}`);
    failures += 1;
    continue;
  }
  if (JSON.stringify(values) !== JSON.stringify(other)) {
    console.error(`value differs: ${name}\n  legacy: ${values.join(" | ")}\n  ported: ${other.join(" | ")}`);
    failures += 1;
  }
}
for (const name of ported.keys()) {
  if (!legacy.has(name)) {
    console.error(`extra in port: ${name}`);
    failures += 1;
  }
}

console.log(`tokens: ${legacy.size} legacy declarations, ${ported.size} ported, ${failures} differences`);
process.exit(failures === 0 ? 0 : 1);
