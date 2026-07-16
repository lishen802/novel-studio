import { useState } from "react";
import type { ChapterView } from "../../lib/studio-client";

type RecoveryDialogProps = {
  chapter: ChapterView;
  onRestore(): void;
  onDiscard(): Promise<void>;
  onLater(): void;
};

function characterCount(value: string): number {
  return Array.from(value).filter((character) => !/\s/u.test(character)).length;
}

function displayTime(value?: string): string {
  if (!value) return "时间未知";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "时间未知";
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function RecoveryDialog({
  chapter,
  onRestore,
  onDiscard,
  onLater,
}: RecoveryDialogProps) {
  const [confirmDiscard, setConfirmDiscard] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const [discardError, setDiscardError] = useState<string | null>(null);
  const recoveryBody = chapter.recoveryBody ?? "";

  const discard = async () => {
    setDiscarding(true);
    setDiscardError(null);
    try {
      await onDiscard();
    } catch (error) {
      setDiscardError(error instanceof Error ? error.message : "无法丢弃恢复草稿，请重试。");
    } finally {
      setDiscarding(false);
    }
  };

  return (
    <div className="dialog-backdrop">
      <section
        className="project-dialog recovery-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="发现恢复草稿"
      >
        <p className="eyebrow">安全恢复</p>
        <h2>发现恢复草稿</h2>
        <p>上次关闭前有一份尚未正式提交的文本。选择前，两份内容都不会被改写。</p>

        <div className="recovery-comparison">
          <article>
            <h3>已提交版本</h3>
            <p>{displayTime(chapter.updatedAt)}</p>
            <strong>{characterCount(chapter.body)} 字</strong>
          </article>
          <article>
            <h3>恢复草稿</h3>
            <p>{displayTime(chapter.recoveryUpdatedAt)}</p>
            <strong>{characterCount(recoveryBody)} 字</strong>
          </article>
        </div>

        {confirmDiscard && (
          <p className="recovery-warning" role="alert">
            丢弃后无法从自动恢复草稿中找回，但已提交正文不会改变。
          </p>
        )}
        {discardError && <p className="recovery-warning" role="alert">{discardError}</p>}

        <div className="dialog-actions recovery-actions">
          <button
            autoFocus
            type="button"
            className="secondary-action"
            disabled={discarding}
            onClick={onLater}
          >
            稍后决定
          </button>
          {!confirmDiscard ? (
            <button
              type="button"
              className="secondary-action"
              disabled={discarding}
              onClick={() => setConfirmDiscard(true)}
            >
              保留已提交版
            </button>
          ) : (
            <button type="button" className="danger-action" disabled={discarding} onClick={() => void discard()}>
              {discarding ? "正在丢弃…" : "确认丢弃恢复草稿"}
            </button>
          )}
          <button type="button" className="primary-action" disabled={discarding} onClick={onRestore}>
            恢复草稿
          </button>
        </div>
      </section>
    </div>
  );
}
