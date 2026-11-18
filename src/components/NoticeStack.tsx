import { useEffect } from "react";
import { useAppStore } from "../stores/appStore";

/** Non-blocking notices from Go (legacy import fallback, watcher limits).
 * The legacy app had no global toast; notices are inline status cards. */
export function NoticeStack() {
  const notices = useAppStore((state) => state.notices);
  const dismiss = useAppStore((state) => state.dismissNotice);

  useEffect(() => {
    if (notices.length === 0) return;
    const timer = window.setTimeout(() => {
      for (const notice of notices) dismiss(notice.id);
    }, 8000);
    return () => window.clearTimeout(timer);
  }, [notices, dismiss]);

  if (notices.length === 0) return null;

  return (
    <div className="notice-stack" role="status" aria-live="polite">
      {notices.map((notice) => (
        <div key={notice.id} className="surface-card notice" data-tone={notice.level}>
          <span>{notice.text}</span>
          <button type="button" className="btn-quiet" onClick={() => dismiss(notice.id)}>
            知道了
          </button>
        </div>
      ))}
    </div>
  );
}
