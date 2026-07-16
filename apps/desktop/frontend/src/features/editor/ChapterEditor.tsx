import { useEffect, useRef, useState } from "react";
import {
  RevisionConflictError,
  type ChapterView,
  type StudioClient,
} from "../../lib/studio-client";

type ChapterEditorProps = {
  chapter: ChapterView;
  client: StudioClient;
  onChapterUpdated?: (chapter: ChapterView) => void;
};

type SaveState = "committed" | "drafting" | "drafted" | "conflict" | "error";

export function ChapterEditor({
  chapter,
  client,
  onChapterUpdated,
}: ChapterEditorProps) {
  const [body, setBody] = useState(chapter.body);
  const [revision, setRevision] = useState(chapter.revision);
  const [saveState, setSaveState] = useState<SaveState>("committed");
  const [showRecovery, setShowRecovery] = useState(chapter.hasRecoveryDraft);
  const bodyRef = useRef(body);
  const revisionRef = useRef(revision);
  const dirtyRef = useRef(false);
  const draftTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const commitTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const committing = useRef(false);
  const commitQueued = useRef(false);
  const commitNow = useRef<() => Promise<void>>(async () => undefined);

  const clearTimers = () => {
    if (draftTimer.current) clearTimeout(draftTimer.current);
    if (commitTimer.current) clearTimeout(commitTimer.current);
    draftTimer.current = null;
    commitTimer.current = null;
  };

  const scheduleSaves = (nextBody: string) => {
    clearTimers();
    dirtyRef.current = true;
    setSaveState("drafting");

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

  commitNow.current = async () => {
    if (committing.current) {
      commitQueued.current = true;
      return;
    }

    committing.current = true;
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
      setSaveState(error instanceof RevisionConflictError ? "conflict" : "error");
    } finally {
      committing.current = false;
      if (commitQueued.current) {
        commitQueued.current = false;
        void commitNow.current();
      }
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
    setShowRecovery(chapter.hasRecoveryDraft);

    return () => {
      clearTimers();
      if (dirtyRef.current) {
        void client.saveDraft(
          chapter.id,
          bodyRef.current,
          revisionRef.current,
        );
      }
    };
    // A changed ID means the author switched chapters. Same-chapter commits must
    // not replace newer local input when the parent updates its revision.
  }, [chapter.id, client]);

  const reload = async () => {
    const current = await client.openChapter(chapter.id);
    setBody(current.body);
    bodyRef.current = current.body;
    setRevision(current.revision);
    revisionRef.current = current.revision;
    dirtyRef.current = false;
    setSaveState("committed");
    onChapterUpdated?.(current);
  };

  const restoreDraft = () => {
    const recovered = chapter.recoveryBody ?? chapter.body;
    setShowRecovery(false);
    setBody(recovered);
    bodyRef.current = recovered;
    scheduleSaves(recovered);
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
          <button type="button" onClick={() => void reload()}>
            重新加载服务器版本
          </button>
        </div>
      )}

      <textarea
        autoFocus
        aria-label="章节正文"
        className="chapter-editor__textarea"
        value={body}
        onChange={(event) => {
          const nextBody = event.target.value;
          setBody(nextBody);
          bodyRef.current = nextBody;
          scheduleSaves(nextBody);
        }}
        placeholder="从这一段开始写下故事……"
        spellCheck={false}
      />

      {showRecovery && (
        <div className="dialog-backdrop">
          <section className="project-dialog recovery-dialog" role="dialog" aria-label="发现恢复草稿">
            <p className="eyebrow">安全恢复</p>
            <h2>发现恢复草稿</h2>
            <p>上次关闭前有一份尚未正式提交的文本。请选择使用哪一版继续。</p>
            <div className="dialog-actions">
              <button type="button" className="secondary-action" onClick={() => setShowRecovery(false)}>
                保留已提交版
              </button>
              <button type="button" className="primary-action" onClick={restoreDraft}>
                恢复草稿
              </button>
            </div>
          </section>
        </div>
      )}
    </section>
  );
}
