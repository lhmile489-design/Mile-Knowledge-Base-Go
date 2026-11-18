import { useCallback, useEffect, useState } from "react";
import { ArrowSquareOut, ArrowsClockwise } from "@phosphor-icons/react";
import {
  docApi,
  events,
  mutationApi,
  type ProvenanceRecordShape,
} from "../lib/transport";
import { useAppStore } from "../stores/appStore";
import { useDocStore } from "../stores/docStore";
import { useMutationStore } from "../stores/mutationStore";

type Props = { vaultId: string; path: string; revision: number };

const STATE_LABEL: Record<string, string> = {
  "up-to-date": "一致",
  "source-changed": "来源改了",
  "copy-modified": "副本改了",
  "both-changed": "两边都改了",
  "source-missing": "来源缺失",
  "copy-missing": "副本缺失",
  unknown: "未知",
};

/** Copy -> source panel under the document (PRD FR-PROV, screenshot 21). */
export function ProvenancePanel({ vaultId, path, revision }: Props) {
  const [entry, setEntry] = useState<ProvenanceRecordShape | null>(null);
  const [syncError, setSyncError] = useState<string | null>(null);
  const openDoc = useDocStore((state) => state.open);
  const setLayout = useAppStore((state) => state.setLayout);
  const report = useMutationStore((state) => state.report);

  const load = useCallback(async () => {
    try {
      const result = await mutationApi.provenanceState(vaultId, path);
      setEntry(result.entry ?? null);
    } catch {
      setEntry(null);
    }
  }, [vaultId, path]);

  useEffect(() => {
    void load();
  }, [load, revision]);

  // The source is a different file: a change there does not bump the copy's
  // revision, so listen to the vault event and refetch when its folder moves.
  useEffect(() => {
    const off = events.vaultChanged.on((event) => {
      if (event.vaultId !== vaultId || !entry) return;
      const parent = entry.sourcePath.includes("/")
        ? entry.sourcePath.slice(0, entry.sourcePath.lastIndexOf("/"))
        : "";
      if (event.directories.includes("") || event.directories.includes(parent)) {
        void load();
      }
    });
    return () => off();
  }, [vaultId, entry, load]);

  // A finished plan that touched this file refreshes the panel even before the
  // watcher notices.
  useEffect(() => {
    if (!report) return;
    if (report.completed?.some((record) => record.target === path)) {
      void load();
    }
  }, [report, path, load]);

  if (!entry) return null;

  const state = entry.assessment?.state ?? "unknown";
  const canSync = Boolean(entry.assessment?.canSyncFromSource);

  const sync = async () => {
    setSyncError(null);
    try {
      const source = await docApi.read(vaultId, entry.sourcePath);
      if (!source.doc) return;
      const plan = await mutationApi.plan({
        vaultId,
        origin: "user",
        title: `同步 ${path}`,
        actions: [
          {
            id: "sync",
            kind: "edit",
            target: path,
            text: source.doc.text,
            expectedTargetRevision: revision,
            reason: "从来源同步",
          },
        ],
      });
      if (plan.plan) {
        useMutationStore.getState().propose(plan.plan, `同步 ${path} 的来源内容`);
      }
    } catch (error) {
      setSyncError(error instanceof Error ? error.message : String(error));
    }
  };

  return (
    <section className="provenance" data-state={state} aria-label="来源追踪">
      <div className="provenance-row">
        <span className="eyebrow">来源</span>
        <span className="provenance-state">{STATE_LABEL[state] ?? state}</span>
        <span className="provenance-base">基于 rev {entry.baseRevision}</span>
      </div>
      <div className="provenance-row">
        <button
          type="button"
          className="provenance-link"
          onClick={() => {
            void openDoc(vaultId, entry.sourcePath);
            void setLayout({ docCollapsed: false });
          }}
        >
          <ArrowSquareOut size={12} aria-hidden /> {entry.sourcePath}
        </button>
        {canSync ? (
          <button
            type="button"
            className="btn-quiet provenance-sync"
            onClick={() => void sync()}
            title="生成一条写入计划，确认后才覆盖副本；可从操作日志恢复"
          >
            <ArrowsClockwise size={12} aria-hidden /> 从来源同步
          </button>
        ) : null}
      </div>
      <p className="provenance-reason">{entry.assessment?.reason}</p>
      {syncError ? (
        <p className="provenance-reason" role="alert">
          {syncError}
        </p>
      ) : null}
    </section>
  );
}
