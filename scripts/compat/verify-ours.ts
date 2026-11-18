// Reads what the Go side wrote with the legacy classes and restores it.
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { TransactionService } from "./src/main/services/TransactionService";
import { parseProvenance, serializeProvenance } from "./src/shared/provenance";

const vault = process.argv[2]!;
const txID = process.argv[3]!;
const service = new TransactionService();
const manifest = await service.read(vault, txID);
const result = await service.restore(vault, "compat", txID);
const content = await readFile(join(vault, "raws", "a.md"), "utf8");
const raw = await readFile(join(vault, ".mile-kb", "provenance.json"), "utf8");
const stable = serializeProvenance(parseProvenance(raw)) === raw;
console.log(
  JSON.stringify({
    status: manifest.status,
    restored: result.restored,
    content,
    provenanceStable: stable,
  }),
);
