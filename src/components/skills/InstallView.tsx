import { useEffect, useState } from "react";
import { Archive, ArrowLeft, FolderOpen, GithubLogo, FileZip } from "@phosphor-icons/react";
import { skillsApi, type SkillLogRecord } from "../../lib/transport";
import { SkillImportWizard } from "../SkillImportWizard";

const OP_LABEL: Record<string, string> = {
  distribute: "分发",
  overwrite: "覆盖",
  uninstall: "卸载",
  adopt: "收编",
  restore: "恢复",
  clean: "清理",
};

/** 安装视图 (M4 T12; UI 稿 install-*): local folder (existing wizard, embedded)
 * plus the recent operation records with restore. GitHub and zip arrive in M5. */
export function InstallView({
  vaultId,
  onBack,
  onChanged,
}: {
  readonly vaultId: string;
  readonly onBack: () => void;
  readonly onChanged: () => void;
}) {
  const [wizard, setWizard] = useState(false);
  const [records, setRecords] = useState<SkillLogRecord[] | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      const value = await skillsApi.log(30);
      setRecords(value.records);
    } catch {
      setRecords([]);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const restore = async (record: SkillLogRecord) => {
    if (!record.backupId) return;
    setBusy(true);
    setNotice(null);
    try {
      await skillsApi.restore(record.backupId);
      setNotice(`已恢复 ${record.skill || "技能"}`);
      await load();
      onChanged();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  };

  if (wizard) {
    return (
      <SkillImportWizard
        embedded
        onClose={() => {
          setWizard(false);
          void load();
          onChanged();
        }}
      />
    );
  }

  return (
    <div className="install-view">
      <header className="install-head">
        <button type="button" className="icon-button" aria-label="返回技能列表" onClick={onBack}>
          <ArrowLeft size={14} aria-hidden />
        </button>
        <strong>安装</strong>
      </header>
      <p className="field-hint">选择一个来源。装进知识库，或直接装进工具（M5 起支持更多来源）。</p>
      <div className="install-sources">
        <button type="button" className="install-source" onClick={() => setWizard(true)} data-testid="install-local">
          <FolderOpen size={20} aria-hidden />
          <span>本地文件夹</span>
        </button>
        <button type="button" className="install-source" disabled>
          <GithubLogo size={20} aria-hidden />
          <span>GitHub</span>
          <em>下个版本</em>
        </button>
        <button type="button" className="install-source" disabled>
          <FileZip size={20} aria-hidden />
          <span>zip 文件</span>
          <em>下个版本</em>
        </button>
      </div>

      <section className="install-recent">
        <h3 className="kb-group-title">最近记录</h3>
        {records === null ? (
          <p className="skills-dialog-line">读取中…</p>
        ) : records.length === 0 ? (
          <p className="skills-empty">还没有技能操作记录。</p>
        ) : (
          <ul className="install-records" data-testid="install-records">
            {records.map((record) => (
              <li key={record.id} className="install-record">
                <span className="install-record-op">{OP_LABEL[record.op] ?? record.op}</span>
                <span className="install-record-skill">{record.skill || "—"}</span>
                <span className="install-record-loc" title={record.location}>
                  {record.location}
                </span>
                <span className="install-record-time">{record.at}</span>
                {record.restorable && record.backupId ? (
                  <button
                    type="button"
                    className="btn-quiet"
                    disabled={busy}
                    onClick={() => void restore(record)}
                    data-testid={`install-restore-${record.id}`}
                  >
                    <Archive size={12} aria-hidden />
                    恢复
                  </button>
                ) : (
                  <span />
                )}
              </li>
            ))}
          </ul>
        )}
        {notice ? (
          <p className="skills-dialog-line" data-testid="install-notice">
            {notice}
          </p>
        ) : null}
      </section>
    </div>
  );
}
