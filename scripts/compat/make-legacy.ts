// Runs the legacy TransactionService against a temp vault so the Go side can
// read and restore its output (read-only use of the legacy sources).
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { TransactionService } from "./src/main/services/TransactionService";
import { serializeProvenance } from "./src/shared/provenance";

const vault = process.argv[2]!;
await mkdir(join(vault, "raws"), { recursive: true });
await mkdir(join(vault, ".mile-kb"), { recursive: true });
await writeFile(join(vault, "raws", "a.md"), "legacy original\n");

const service = new TransactionService();
const tx = await service.begin(vault, "compat", "plan-legacy");
await service.capture(tx, "raws/a.md", join(vault, "raws", "a.md"));
await writeFile(join(vault, "raws", "a.md"), "written by legacy\n");
await service.commitWithAfterImages(tx, vault);

const records = [
  {
    copyPath: "projects/a<b>.md",
    sourcePath: "raws/a.md",
    baseRevision: 1,
    baseHash: "hash",
    copiedAt: "2026-01-01T00:00:00.000Z",
  },
];
await writeFile(
  join(vault, ".mile-kb", "provenance.json"),
  serializeProvenance(records),
);
console.log(tx.id);
