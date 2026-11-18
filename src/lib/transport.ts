// transport is the only module that talks to the generated bindings.
// Components and stores import from here, never from ../mygo directly.
import { isCallError } from "mygo-runtime";
import {
  DocService,
  IndexService,
  RenderService,
  SettingsService,
  VaultService,
  WindowService,
  events,
  type AppSettings,
  type IndexStatus,
  type SearchHit,
  type SearchQuery,
  type SearchResult,
  type ChooseVaultResult,
  type PickFolderResult,
  type ReadResult,
  type RecentDocumentsResult,
  type RenderResult,
  type SettingsResult,
  type TreeResult,
  type VaultListResult,
  type WriteOutcome,
} from "../mygo";

export type { AppSettings, IndexStatus, SearchHit, SearchQuery, SearchResult, ChooseVaultResult, PickFolderResult, ReadResult, RecentDocumentsResult, RenderResult, SettingsResult, TreeResult, VaultListResult, WriteOutcome };

/** TransportError carries a stable code; unexpected failures get `unknown`. */
export class TransportError extends Error {
  readonly code: string;
  constructor(code: string, message: string) {
    super(message);
    this.name = "TransportError";
    this.code = code;
  }
}

function unwrap<T extends { failure?: { code: string; message: string } }>(value: T): T {
  if (value?.failure) {
    throw new TransportError(value.failure.code, value.failure.message);
  }
  return value;
}

async function guard<T>(work: () => Promise<T>): Promise<T> {
  try {
    return await work();
  } catch (error) {
    if (error instanceof TransportError) throw error;
    if (isCallError(error)) {
      throw new TransportError("unknown", `${error.method}: ${error.message}`);
    }
    throw new TransportError("unknown", String(error));
  }
}

export const settingsApi = {
  get: () => guard(async () => unwrap(await SettingsService.get())),
  patch: (patch: Parameters<typeof SettingsService.patch>[0]) =>
    guard(async () => unwrap(await SettingsService.patch(patch))),
};

export const vaultApi = {
  list: () => guard(async () => unwrap(await VaultService.list())),
  open: (vaultId: string) => guard(async () => unwrap(await VaultService.open(vaultId))),
  forget: (vaultId: string) => guard(async () => unwrap(await VaultService.forget(vaultId))),
  retry: (vaultId: string) => guard(async () => unwrap(await VaultService.retry(vaultId))),
  choose: () => guard(async () => unwrap(await VaultService.choose())),
  pickFolder: () => guard(async () => unwrap(await VaultService.pickFolder())),
  createVault: (token: string, confirmNonEmpty: boolean) =>
    guard(async () => unwrap(await VaultService.createVault(token, confirmNonEmpty))),
};

export const docApi = {
  tree: (vaultId: string, path: string) => guard(async () => unwrap(await DocService.tree(vaultId, path))),
  read: (vaultId: string, path: string) => guard(async () => unwrap(await DocService.read(vaultId, path))),
  write: (vaultId: string, path: string, expectedRevision: number, text: string) =>
    guard(async () => unwrap(await DocService.write(vaultId, path, expectedRevision, text))),
  recent: (vaultId: string) => guard(async () => unwrap(await DocService.recentDocuments(vaultId))),
  noteOpened: (vaultId: string, path: string) =>
    guard(async () => unwrap(await DocService.noteDocumentOpened(vaultId, path))),
};

export const renderApi = {
  document: (vaultId: string, path: string, revision: number) =>
    guard(async () => unwrap(await RenderService.renderDocument(vaultId, path, revision))),
};

export const indexApi = {
  search: (query: SearchQuery) => guard(async () => unwrap(await IndexService.search(query))),
  status: (vaultId: string) => guard(async () => unwrap(await IndexService.status(vaultId))),
  rebuild: (vaultId: string) => guard(async () => unwrap(await IndexService.rebuild(vaultId))),
};

export const windowApi = {
  ready: () => guard(async () => WindowService.ready()),
  minimize: () => guard(async () => WindowService.minimize()),
  toggleMaximize: () => guard(async () => WindowService.toggleMaximize()),
  close: () => guard(async () => WindowService.close()),
  openExternal: (url: string) => guard(async () => WindowService.openExternal(url)),
  pickVaultDir: () => guard(async () => unwrap(await WindowService.pickVaultDir())),
  reportDocument: (vaultId: string, path: string, revision: number, dirty: boolean) =>
    guard(async () => WindowService.reportDocument(vaultId, path, revision, dirty)),
};

export { events };
