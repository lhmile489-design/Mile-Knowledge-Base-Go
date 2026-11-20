// transport is the only module that talks to the generated bindings.
// Components and stores import from here, never from ../mygo directly.
import { isCallError } from "mygo-runtime";
import {
  DocService,
  IndexService,
  MutationService,
  RenderService,
  SkillService,
  SettingsService,
  VaultService,
  WindowService,
  events,
  type AgentPatch,
  type CategoryConfig,
  type ProviderDraft,
  type ProvidersResult,
  type AppSettings,
  type IndexStatus,
  type KbHealthReport,
  type LinkGraphResult,
  type MutationPlan,
  type MutationReport,
  type PlanResult,
  type AuditListResult,
  type TransactionSummary,
  type RestorePreview,
  type RestoreResult,
  type ProvenanceListResult,
  type ProvenanceRecordShape,
  type ProvenanceStateResult,
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
  type SkillScan,
  type SkillScanResult,
  type SkillRegistryResult,
  type SkillTool,
  type SkillCategory,
  type ToolSkill,
  type SkillsChangedEvent,
  type SkillDistributeTarget,
  type SkillPreviewResult,
  type SkillOpResult,
  type SkillUninstallResult,
  type SkillRestoreResult,
  type SkillBackup,
  type SkillBackupsResult,
  type SkillAdoptResult,
  type SkillToolRef,
  type SkillLogRecord,
  type SkillLogResult,
  type KbSkill,
  type KbSkillDistribution,
  type SkillPreviewItem,
  type SkillToolScan,
  type SkillUninstallPreview,
} from "../mygo";

export type { SkillScan, SkillScanResult, SkillRegistryResult, SkillTool, SkillCategory, ToolSkill, SkillsChangedEvent, SkillDistributeTarget, SkillPreviewResult, SkillOpResult };
export type { SkillUninstallResult, SkillRestoreResult, SkillBackup, SkillBackupsResult, SkillAdoptResult, SkillToolRef, SkillLogRecord, SkillLogResult, KbSkill, KbSkillDistribution, SkillPreviewItem };
export type { SkillToolScan, SkillUninstallPreview };

export type { AppSettings, IndexStatus, KbHealthReport, LinkGraphResult, MutationPlan, MutationReport, PlanResult, AuditListResult, TransactionSummary, RestorePreview, RestoreResult, ProvenanceListResult, ProvenanceRecordShape, ProvenanceStateResult, SearchHit, SearchQuery, SearchResult, ChooseVaultResult, PickFolderResult, ReadResult, RecentDocumentsResult, RenderResult, SettingsResult, TreeResult, VaultListResult, WriteOutcome };

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
  appConfigGet: (vaultId: string) => guard(async () => unwrap(await SettingsService.appConfigGet(vaultId))),
  appConfigPatchAgent: (vaultId: string, agent: string, patch: AgentPatch) =>
    guard(async () => unwrap(await SettingsService.appConfigPatchAgent(vaultId, agent, patch))),
  appConfigPatchAliases: (vaultId: string, patches: Record<string, string | null>) =>
    guard(async () => unwrap(await SettingsService.appConfigPatchAliases(vaultId, patches))),
  appConfigSetCategories: (vaultId: string, categories: CategoryConfig[]) =>
    guard(async () => unwrap(await SettingsService.appConfigSetCategories(vaultId, categories))),
  aboutInfo: () => guard(async () => unwrap(await SettingsService.aboutInfo())),
  copyStoragePath: (kind: string) => guard(async () => unwrap(await SettingsService.copyStoragePath(kind))),
  providers: (vaultId: string) => guard(async () => unwrap(await SettingsService.providers(vaultId))),
  skillCatalog: (vaultId: string) => guard(async () => unwrap(await SettingsService.skillCatalog(vaultId))),
  saveProviders: (vaultId: string, providers: ProviderDraft[]) =>
    guard(async () => unwrap(await SettingsService.saveProviders(vaultId, providers))),
  setKey: (envName: string, value: string): Promise<ProvidersResult> =>
    guard(async () => SettingsService.setKey(envName, value)),
  fetchModels: (kind: string, baseUrl: string, apiKeyEnv: string, apiKey: string) =>
    guard(async () => SettingsService.fetchModels(kind, baseUrl, apiKeyEnv, apiKey)),
  patch: (patch: Parameters<typeof SettingsService.patch>[0]) =>
    guard(async () => unwrap(await SettingsService.patch(patch))),
};

export const skillsApi = {
  registry: () => guard(async () => unwrap(await SkillService.registry())),
  scan: (vaultId: string) => guard(async () => unwrap(await SkillService.scan(vaultId))),
  distributePreview: (vaultId: string, skillPath: string, targets: SkillDistributeTarget[], mode: string) =>
    guard(async () => unwrap(await SkillService.distributePreview(vaultId, skillPath, targets, mode))),
  distribute: (vaultId: string, skillPath: string, targets: SkillDistributeTarget[], mode: string) =>
    guard(async () => unwrap(await SkillService.distribute(vaultId, skillPath, targets, mode))),
  uninstallPreview: (toolId: string, path: string) =>
    guard(async () => unwrap(await SkillService.uninstallPreview(toolId, path))),
  uninstall: (toolId: string, path: string) =>
    guard(async () => unwrap(await SkillService.uninstall(toolId, path))),
  restore: (backupId: string) => guard(async () => unwrap(await SkillService.restore(backupId))),
  backups: () => guard(async () => unwrap(await SkillService.backups())),
  cleanBackups: (ids: string[]) => guard(async () => unwrap(await SkillService.cleanBackups(ids))),
  adopt: (vaultId: string, toolId: string, path: string, category: string, linkBack: boolean, extras: SkillToolRef[]) =>
    guard(async () => unwrap(await SkillService.adopt(vaultId, toolId, path, category, linkBack, extras))),
  log: (limit = 50) => guard(async () => unwrap(await SkillService.log(limit))),
  reveal: (toolId: string, path: string) => guard(async () => SkillService.reveal(toolId, path)),
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
  pickSkillSource: () => guard(async () => unwrap(await VaultService.pickSkillSource())),
  importSkills: (vaultId: string, token: string, category: string, names: string[]) =>
    guard(async () => unwrap(await VaultService.importSkills(vaultId, token, category, names))),
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
  block: (text: string, kind: string) =>
    guard(async () => unwrap(await RenderService.renderBlock(text, kind))),
};

export const indexApi = {
  search: (query: SearchQuery) => guard(async () => unwrap(await IndexService.search(query))),
  status: (vaultId: string) => guard(async () => unwrap(await IndexService.status(vaultId))),
  rebuild: (vaultId: string) => guard(async () => unwrap(await IndexService.rebuild(vaultId))),
  links: (vaultId: string, path: string) => guard(async () => unwrap(await IndexService.links(vaultId, path))),
  health: (vaultId: string) => guard(async () => unwrap(await IndexService.health(vaultId))),
};

export const mutationApi = {
  plan: (request: Parameters<typeof MutationService.plan>[0]) =>
    guard(async () => unwrap(await MutationService.plan(request))),
  rename: (vaultId: string, path: string, newName: string) =>
    guard(async () => unwrap(await MutationService.rename(vaultId, path, newName))),
  createFile: (vaultId: string, dirPath: string, name: string) =>
    guard(async () => unwrap(await MutationService.createFile(vaultId, dirPath, name))),
  createDir: (vaultId: string, dirPath: string, name: string) =>
    guard(async () => unwrap(await MutationService.createDir(vaultId, dirPath, name))),
  planMove: (vaultId: string, sources: string[], targetDir: string) =>
    guard(async () => unwrap(await MutationService.planMove(vaultId, sources, targetDir))),
  execute: (plan: MutationPlan) => guard(async () => unwrap(await MutationService.execute(plan))),
  auditList: (vaultId: string, planId: string, limit: number) =>
    guard(async () => unwrap(await MutationService.auditList(vaultId, planId, limit))),
  rewindPrepare: (vaultId: string, transactionId: string) =>
    guard(async () => unwrap(await MutationService.rewindPrepare(vaultId, transactionId))),
  rewindCommit: (vaultId: string, transactionId: string) =>
    guard(async () => unwrap(await MutationService.rewindCommit(vaultId, transactionId))),
  provenanceList: (vaultId: string) => guard(async () => unwrap(await MutationService.provenanceList(vaultId))),
  provenanceState: (vaultId: string, path: string) =>
    guard(async () => unwrap(await MutationService.provenanceState(vaultId, path))),
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

// E2E hook: the transport surface the probes drive (same calls the UI makes).
if (typeof window !== "undefined") {
  (window as unknown as { __mkbApi?: unknown }).__mkbApi = {
    skills: skillsApi,
    mutation: mutationApi,
    doc: docApi,
    index: indexApi,
    provenance: {
      list: (vaultId: string) => mutationApi.provenanceList(vaultId),
      state: (vaultId: string, path: string) => mutationApi.provenanceState(vaultId, path),
    },
  };
}
