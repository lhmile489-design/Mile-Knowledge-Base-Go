// Opening one in-vault reference shared by inline tags and the evidence radar.
// The probe reads first: opening a fabricated path would clear the current
// document and close its tabs, so a failed probe changes nothing.
import { useDocStore } from "../stores/docStore";
import type { RefKind } from "./chatReferences";
import { docApi, indexApi, TransportError } from "./transport";

export type RefStatus = "ok" | "missing" | "ambiguous";

/**
 * Pick the one document a wikilink target means: an exact path, or a file
 * whose base name equals the target. Zero or several candidates are told
 * apart — the user never gets a silent guess.
 */
export function chooseWikiCandidate(paths: readonly string[], target: string): string | "ambiguous" | null {
  const wanted = target.trim().toLowerCase();
  if (wanted === "") return null;
  const matches = paths.filter((path) => {
    const name = path.split("/").pop() ?? "";
    const base = name.replace(/\.(md|markdown|txt)$/i, "");
    return (
      path.toLowerCase() === wanted ||
      name.toLowerCase() === wanted ||
      base.toLowerCase() === wanted
    );
  });
  const unique = [...new Set(matches)];
  if (unique.length === 0) return null;
  if (unique.length > 1) return "ambiguous";
  return unique[0] ?? null;
}

async function resolvePath(vaultId: string, refKind: RefKind, target: string): Promise<string | "ambiguous" | null> {
  if (refKind === "path") return target;
  const result = await indexApi.search({ vaultId, text: target, limit: 20 });
  return chooseWikiCandidate(result.hits.map((hit) => hit.path), target);
}

/** Probes and opens one reference; the caller renders the returned status. */
export async function openReference(vaultId: string, refKind: RefKind, target: string): Promise<RefStatus> {
  let path: string | "ambiguous" | null;
  try {
    path = await resolvePath(vaultId, refKind, target);
  } catch {
    return "missing";
  }
  if (path === "ambiguous") return "ambiguous";
  if (path === null) return "missing";
  try {
    await docApi.read(vaultId, path);
  } catch (error) {
    if (error instanceof TransportError) return "missing";
    return "missing";
  }
  await useDocStore.getState().open(vaultId, path);
  return "ok";
}
