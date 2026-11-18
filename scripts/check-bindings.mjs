// Contract check for the agent bindings (PLAN-M2 T10).
//
// The generated client is the only source of truth for what the page can
// call. This script fails when a generated AgentService / ConversationService
// method is never called from src/ (a binding nobody can reach is usually a
// forgotten wire-up), or when src/ calls a method the client does not have.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const root = process.cwd();
const generated = readFileSync(join(root, "src", "mygo.ts"), "utf8");

const services = new Map();
for (const match of generated.matchAll(/export const (\w+) = \{([\s\S]*?)\n\} as const;/g)) {
  const [, name, body] = match;
  if (name !== "AgentService" && name !== "ConversationService") continue;
  const methods = new Set();
  for (const method of body.matchAll(/^ {2}([a-zA-Z0-9_]+): bind</gm)) {
    methods.add(method[1]);
  }
  services.set(name, methods);
}

const files = [];
const walk = (directory) => {
  for (const entry of readdirSync(directory)) {
    const path = join(directory, entry);
    if (statSync(path).isDirectory()) {
      walk(path);
    } else if (/\.(ts|tsx)$/.test(entry) && path !== join(root, "src", "mygo.ts")) {
      files.push(path);
    }
  }
};
walk(join(root, "src"));
const source = files.map((file) => readFileSync(file, "utf8")).join("\n");

let failed = false;
for (const [name, methods] of services) {
  for (const method of methods) {
    if (!new RegExp(`\\b${name}\\.${method}\\s*\\(`).test(source)) {
      console.error(`generated but never called: ${name}.${method}`);
      failed = true;
    }
  }
}
for (const match of source.matchAll(/\b(AgentService|ConversationService)\.([a-zA-Z0-9_]+)\s*\(/g)) {
  const [, name, method] = match;
  if (!services.get(name)?.has(method)) {
    console.error(`call to a missing binding: ${name}.${method}`);
    failed = true;
  }
}

if (failed) {
  process.exit(1);
}
const counts = [...services].map(([name, methods]) => `${name}:${methods.size}`).join(" ");
console.log(`bindings ok (${counts})`);
