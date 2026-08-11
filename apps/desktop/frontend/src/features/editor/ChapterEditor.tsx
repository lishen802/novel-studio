import { useEffect, useRef, useState } from "react";
import {
  RevisionConflictError,
  StudioClientError,
  type ChapterView,
  type StudioClient,
} from "../../lib/studio-client";
import { RecoveryDialog } from "./RecoveryDialog";

type ChapterEditorProps = {
  chapter: ChapterView;
  client: StudioClient;
  onChapterUpdated?: (chapter: ChapterView) => void;
  onFlushAvailable?: (flush: (() => Promise<void>) | null) => void;
  onFilesystemHealthChanged?: () => Promise<void>;
  readOnly?: boolean;
};

type SaveState = "committed" | "drafting" | "drafted" | "conflict" | "error";
type CommitRecoveryState = "none" | "saved" | "failed";

export function ChapterEditor({
  chapter,
  client,
  onChapterUpdated,
  onFlushAvailable,
  onFilesystemHealthChanged,
  readOnly = false,
}: ChapterEditorProps) {
  const [body, setBody] = useState(chapter.body);
  const [revision, setRevision] = useState(chapter.revision);
  const [saveState, setSaveState] = useState<SaveState>("committed");
  const [commitRecoveryState, setCommitRecoveryState] =
    useState<CommitRecoveryState>("none");
  const [reloading, setReloading] = useState(false);
  const [showRecovery, setShowRecovery] = useState(chapter.hasRecoveryDraft);
  const [recoveryDeferred, setRecoveryDeferred] = useState(false);
  const bodyRef = useRef(body);
  const revisionRef = useRef(revision);
  const dirtyRef = useRef(false);
  const draftTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const commitTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const committing = useRef(false);
  const commitQueued = useRef(false);
  const reloadingRef = useRef(false);
  const editVersion = useRef(0);
  const reloadRequest = useRef(0);
  const activeCommit = useRef<Promise<void>>(Promise.resolve());
const commitNow = useRef<() => Promise<void>>(async () => undefined);
  const flushDraft = useRef<() => Promise<void>>(async () => undefined);

  const clearTimers = () => {
    if (draftTimer.current) clearTimeout(draftTimer.current);
    if (commitTimer.current) clearTimeout(commitTimer.current);
    draftTimer.current = null;
    commitTimer.current = null;
  };

  const scheduleSaves = (nextBody: string) => {
    if (readOnly) return;
    clearTimers();
    editVersion.current++;
    dirtyRef.current = true;
    setCommitRecoveryState("none");
    setSaveState(reloadingRef.current ? "conflict" : "drafting");

    draftTimer.current = setTimeout(async () => {
      const draftBody = bodyRef.current;
      const draftRevision = revisionRef.current;
      try {
        await client.saveDraft(chapter.id, draftBody, draftRevision);
        if (bodyRef.current === draftBody && !committing.current) {
          setSaveState("drafted");
        }
      } catch {
        setSaveState("error");
      }
    }, 200);

    commitTimer.current = setTimeout(() => {
      void commitNow.current();
    }, 750);

    bodyRef.current = nextBody;
  };

  commitNow.current = () => {
    if (readOnly) return Promise.resolve();
    if (committing.current) {
      commitQueued.current = true;
      return activeCommit.current;
    }

    committing.current = true;
    const operation = (async () => {
      const committedBody = bodyRef.current;
      const baseRevision = revisionRef.current;
      try {
        const updated = await client.commitChapter(
          chapter.id,
          committedBody,
          baseRevision,
        );
        revisionRef.current = updated.revision;
        setRevision(updated.revision);
        onChapterUpdated?.(updated);
        if (bodyRef.current === committedBody) {
          dirtyRef.current = false;
          setSaveState("committed");
          setCommitRecoveryState("none");
        } else {
          try {
            await client.saveDraft(
              chapter.id,
              bodyRef.current,
              updated.revision,
            );
            setSaveState("drafting");
          } catch {
            setSaveState("error");
          }
        }
      } catch (error) {
        dirtyRef.current = true;
        if (isProjectFilesystemError(error)) {
          void onFilesystemHealthChanged?.();
        }
        if (!(error instanceof RevisionConflictError)) {
          try {
            await client.saveDraft(chapter.id, committedBody, baseRevision);
            setCommitRecoveryState("saved");
          } catch {
            setCommitRecoveryState("failed");
          }
        }
        setSaveState(error instanceof RevisionConflictError ? "conflict" : "error");
      } finally {
        committing.current = false;
        if (commitQueued.current) {
          commitQueued.current = false;
          void commitNow.current();
        }
      }
    })();
    activeCommit.current = operation;
    return operation;
  };

  flushDraft.current = async () => {
    if (readOnly) return;
    clearTimers();
    while (committing.current) {
      await activeCommit.current;
    }
    if (!dirtyRef.current) return;

    const draftBody = bodyRef.current;
    const draftRevision = revisionRef.current;
    try {
      await client.saveDraft(chapter.id, draftBody, draftRevision);
      if (bodyRef.current === draftBody && revisionRef.current === draftRevision) {
        setSaveState("drafted");
      }
    } catch (error) {
      setSaveState("error");
      throw error;
    }
  };

  useEffect(() => {
    clearTimers();
    setBody(chapter.body);
    bodyRef.current = chapter.body;
    setRevision(chapter.revision);
    revisionRef.current = chapter.revision;
    dirtyRef.current = false;
    setSaveState("committed");
    setCommitRecoveryState("none");
    setShowRecovery(!readOnly && chapter.hasRecoveryDraft);
    setRecoveryDeferred(false);

    return () => {
      clearTimers();
      if (!readOnly && dirtyRef.current && !onFlushAvailable) {
        void flushDraft.current().catch(() => undefined);
      }
    };
    // A changed ID means the author switched chapters. Same-chapter commits must
    // not replace newer local input when the parent updates its revision.
  }, [chapter.id, client, readOnly]);

  useEffect(() => {
    const registeredFlush = flushDraft.current;
    onFlushAvailable?.(registeredFlush);
    return () => onFlushAvailable?.(null);
  }, [chapter.id, onFlushAvailable]);

  const reload = async () => {
    const editVersionAtRequest = editVersion.current;
    const request = ++reloadRequest.current;
    reloadingRef.current = true;
    setReloading(true);
    try {
      const current = await client.openChapter(chapter.id);
      if (
        request !== reloadRequest.current ||
        editVersion.current !== editVersionAtRequest
      ) {
        setSaveState("conflict");
        return;
      }
      clearTimers();
      setBody(current.body);
      bodyRef.current = current.body;
      setRevision(current.revision);
      revisionRef.current = current.revision;
      dirtyRef.current = false;
      setSaveState("committed");
      onChapterUpdated?.(current);
    } catch {
      setSaveState("conflict");
    } finally {
      if (request === reloadRequest.current) {
        reloadingRef.current = false;
        setReloading(false);
      }
    }
  };

  const restoreDraft = () => {
    const recovered = chapter.recoveryBody ?? chapter.body;
    setShowRecovery(false);
    setRecoveryDeferred(false);
    setBody(recovered);
    bodyRef.current = recovered;
    scheduleSaves(recovered);
  };

  const discardDraft = async () => {
    await client.discardDraft(chapter.id, chapter.revision);
    setShowRecovery(false);
    setRecoveryDeferred(false);
    onChapterUpdated?.({
      ...chapter,
      recoveryBody: undefined,
      recoveryUpdatedAt: undefined,
      hasRecoveryDraft: false,
    });
  };

  return (
    <section className="chapter-editor" aria-labelledby="active-chapter-title">
      <header className="chapter-editor__header">
        <div>
          <p className="eyebrow">正文编辑</p>
          <h2 id="active-chapter-title">{chapter.title}</h2>
        </div>
        <p className={`save-status save-status--${saveState}`} aria-live="polite">
          {saveState === "drafting" && "草稿保存中"}
          {saveState === "drafted" && `草稿已保存 · 版本 ${revision}`}
          {saveState === "committed" && `已提交 · 版本 ${revision}`}
          {saveState === "conflict" && "版本冲突 · 本地文本已保留"}
          {saveState === "error" && "保存失败 · 本地文本已保留"}
        </p>
      </header>

      {saveState === "conflict" && (
        <div className="editor-alert" role="alert">
          <span>检测到版本冲突，请比较本地文本或重新加载最新版本。</span>
          <button type="button" disabled={reloading} onClick={() => void reload()}>
            {reloading ? "正在加载…" : "重新加载服务器版本"}
          </button>
        </div>
      )}

      {commitRecoveryState !== "none" && (
        <div className="editor-alert" role="alert">
          <span>
            {commitRecoveryState === "saved"
              ? "恢复草稿已安全保存"
              : "无法保存恢复草稿；请复制文本"}
          </span>
          <div className="editor-alert__actions">
            <button type="button" onClick={() => void commitNow.current()}>
              重试保存
            </button>
            <button
              type="button"
              className="secondary-action"
              onClick={() => void navigator.clipboard.writeText(bodyRef.current)}
            >
              复制文本
            </button>
          </div>
        </div>
      )}

      {recoveryDeferred && (
        <div className="editor-alert recovery-deferred" role="status">
          <span>恢复草稿仍被安全保留；决定前正文只读。</span>
          <button type="button" onClick={() => setShowRecovery(true)}>
            处理恢复草稿
          </button>
        </div>
      )}

      <textarea
        autoFocus
        aria-label="章节正文"
        className="chapter-editor__textarea"
        value={body}
        readOnly={readOnly || showRecovery || recoveryDeferred}
        onChange={(event) => {
          if (readOnly) return;
          const nextBody = event.target.value;
          setBody(nextBody);
          bodyRef.current = nextBody;
          scheduleSaves(nextBody);
        }}
        placeholder="从这一段开始写下故事……"
        spellCheck={false}
      />

      {showRecovery && (
        <RecoveryDialog
          chapter={chapter}
          onRestore={restoreDraft}
          onDiscard={discardDraft}
          onLater={() => {
            setShowRecovery(false);
            setRecoveryDeferred(true);
          }}
        />
      )}
    </section>
  );
}

function isProjectFilesystemError(error: unknown): boolean {
  return (
    error instanceof StudioClientError &&
    new Set([
      "PROJECT_FILESYSTEM_PREFLIGHT_FAILED",
      "PROJECT_PATH_TOO_LONG",
      "PROJECT_SHARING_VIOLATION",
      "PROJECT_CHECKPOINT_FAILED",
      "PROJECT_ATOMIC_REPLACE_FAILED",
      "PROJECT_FILESYSTEM_BUSY",
      "PROJECT_FILESYSTEM_READ_ONLY",
      "PROJECT_FILESYSTEM_DISK_FULL",
      "PROJECT_FILESYSTEM_IO",
    ]).has(error.code)
  );
}
