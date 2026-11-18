// Typed facade over the generated AgentService / ConversationService bindings
// and the E07–E10 events (T10). Components and stores use this module, never
// the generated client directly.
import { AgentService, ConversationService, events } from "../mygo";
import type {
  AgentListResult,
  AppConfigChangedEvent,
  ConversationChangedEvent,
  ConversationClearResult,
  ConversationExportResult,
  ConversationListResult,
  ConversationMetaResult,
  ConversationReadResult,
  ConversationSearchResult,
  EngineStatus,
  Failure,
  PromptResult,
  RoutePreviewResult,
  SharedContext,
  SharedContextResult,
  StreamFrame,
} from "../mygo";
import { openFrameStream, type FrameStream } from "./stream";
import { TransportError } from "./transport";

export type { FrameStream } from "./stream";

function failureOf(failure: Failure | null | undefined): TransportError | null {
  return failure?.code ? new TransportError(failure.code, failure.message) : null;
}

async function checked<T extends { failure?: Failure }>(work: Promise<T>): Promise<T> {
  const value = await work;
  const error = failureOf(value.failure);
  if (error) throw error;
  return value;
}

async function mustSucceed(work: Promise<Failure>): Promise<void> {
  const error = failureOf(await work);
  if (error) throw error;
}

export const agentApi = {
  // ---- M38–M44 ----
  list: (agent: string, vaultId: string): Promise<AgentListResult> =>
    checked(AgentService.list(agent, vaultId)),
  prompt: (
    agent: string,
    vaultId: string,
    text: string,
    onFrame: (frame: StreamFrame) => void,
  ): FrameStream =>
    openFrameStream<StreamFrame>(
      (channel) => AgentService.prompt(agent, vaultId, text, channel),
      onFrame,
    ),
  cancel: (agent: string): Promise<void> => AgentService.cancel(agent),
  approve: (requestId: string, optionId: string): Promise<void> =>
    mustSucceed(AgentService.approve(requestId, optionId)),
  reject: (requestId: string): Promise<void> => mustSucceed(AgentService.reject(requestId)),
  reset: (vaultId: string): Promise<void> => AgentService.reset(vaultId),
  routePreview: (text: string): Promise<RoutePreviewResult> => AgentService.routePreview(text),

  // ---- M45–M49 shared context ----
  contextSnapshot: (vaultId: string): Promise<SharedContextResult> =>
    checked(AgentService.contextSnapshot(vaultId)),
  contextPin: (vaultId: string, path: string, note: string): Promise<SharedContextResult> =>
    checked(AgentService.contextPin(vaultId, path, note)),
  contextUnpin: (referenceId: string): Promise<SharedContextResult> =>
    checked(AgentService.contextUnpin(referenceId)),
  contextSetTaskStatus: (taskId: string, status: string): Promise<SharedContextResult> =>
    checked(AgentService.contextSetTaskStatus(taskId, status)),
  contextReportDocument: (
    vaultId: string,
    path: string,
    revision: number,
    dirty: boolean,
    selection: string,
  ): Promise<void> =>
    mustSucceed(AgentService.contextReportDocument(vaultId, path, revision, dirty, selection)),

  // ---- M50–M59 conversation history ----
  conversations: {
    list: (vaultId: string): Promise<ConversationListResult> =>
      checked(ConversationService.list(vaultId)),
    get: (vaultId: string, id: string): Promise<ConversationReadResult> =>
      checked(ConversationService.get(vaultId, id)),
    create: (vaultId: string): Promise<ConversationMetaResult> =>
      checked(ConversationService.create(vaultId)),
    rename: (vaultId: string, id: string, title: string): Promise<ConversationMetaResult> =>
      checked(ConversationService.rename(vaultId, id, title)),
    pin: (vaultId: string, id: string, pinned: boolean): Promise<ConversationMetaResult> =>
      checked(ConversationService.pin(vaultId, id, pinned)),
    remove: (vaultId: string, id: string): Promise<void> =>
      mustSucceed(ConversationService.remove(vaultId, id)),
    activate: (vaultId: string, id: string): Promise<ConversationReadResult> =>
      checked(ConversationService.activate(vaultId, id)),
    search: (vaultId: string, query: string): Promise<ConversationSearchResult> =>
      checked(ConversationService.search(vaultId, query)),
    export: (vaultId: string, id: string): Promise<ConversationExportResult> =>
      checked(ConversationService.export(vaultId, id)),
    clear: (vaultId: string): Promise<ConversationClearResult> =>
      checked(ConversationService.clear(vaultId)),
  },

  // ---- E07–E10 ----
  onConversationChanged: (listener: (event: ConversationChangedEvent) => void): (() => void) =>
    events.conversationChanged.on(listener),
  onAppConfigChanged: (listener: (event: AppConfigChangedEvent) => void): (() => void) =>
    events.appConfigChanged.on(listener),
  onContextChanged: (listener: (event: SharedContext) => void): (() => void) =>
    events.contextChanged.on(listener),
  onEngineStatus: (listener: (event: EngineStatus) => void): (() => void) =>
    events.engineStatus.on(listener),
};

export type { PromptResult };
