import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "Mile Knowledge Base",
  identifier: "dev.mile.kb",
  version: "1.3.2",
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://127.0.0.1:5274",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
});
