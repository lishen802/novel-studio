import { FilePlus2, Feather, LibraryBig } from "lucide-react";
import { useState, type FormEvent } from "react";
import type { ChapterView, StudioClient } from "../../lib/studio-client";
import { ChapterEditor } from "../editor/ChapterEditor";
import type { WorkspaceStatus } from "./workspace-store";

type ReadyWorkspace = Extract<WorkspaceStatus, { kind: "ready" }>;

type WorkspaceProps = {
  workspace: ReadyWorkspace;
  error: string | null;
  client: StudioClient;
  onCreateChapter(title: string): Promise<boolean>;
  onSelectChapter(id: string): Promise<void>;
  onChapterUpdated(chapter: ChapterView): void;
  onDismissError(): void;
  onFlushAvailable(flush: (() => Promise<void>) | null): void;
};

export function Workspace({
  workspace,
  error,
  client,
  onCreateChapter,
  onSelectChapter,
  onChapterUpdated,
  onDismissError,
  onFlushAvailable,
}: WorkspaceProps) {
  const [showNewChapter, setShowNewChapter] = useState(false);
  const [chapterTitle, setChapterTitle] = useState("");
  const [creatingChapter, setCreatingChapter] = useState(false);
  const activeChapter = workspace.chapters.find(
    (chapter) => chapter.id === workspace.activeChapterId,
  );
  const openNewChapter = () => {
    onDismissError();
    setShowNewChapter(true);
  };

  const submitChapter = async (event: FormEvent) => {
    event.preventDefault();
    if (!chapterTitle.trim()) return;
    setCreatingChapter(true);
    try {
      const created = await onCreateChapter(chapterTitle);
      if (!created) return;
      setChapterTitle("");
      setShowNewChapter(false);
    } finally {
      setCreatingChapter(false);
    }
  };

  return (
    <main className="workspace-shell">
      <aside className="chapter-sidebar">
        <header className="workspace-brand">
          <span className="workspace-brand__mark" aria-hidden="true">
            <Feather size={20} />
          </span>
          <div>
            <p>织境 Story Studio</p>
            <span>本地创作空间</span>
          </div>
        </header>

        <section className="project-summary">
          <p className="eyebrow">当前作品</p>
          <h1>{workspace.project.title}</h1>
          <p>{workspace.project.genre || "未设置类型"}</p>
        </section>

        <div className="chapter-list__header">
          <span>章节</span>
          <button type="button" aria-label="新建章节" onClick={openNewChapter}>
            <FilePlus2 size={17} />
          </button>
        </div>

        <nav className="chapter-list" aria-label="章节列表">
          {workspace.chapters.length === 0 ? (
            <div className="chapter-empty">
              <LibraryBig size={24} aria-hidden="true" />
              <p>还没有章节</p>
              <button type="button" onClick={openNewChapter}>
                新建第一章
              </button>
            </div>
          ) : (
            workspace.chapters.map((chapter) => (
              <button
                type="button"
                className={chapter.id === workspace.activeChapterId ? "is-active" : ""}
                key={chapter.id}
                onClick={() => void onSelectChapter(chapter.id)}
              >
                <span>{chapter.position.toString().padStart(2, "0")}</span>
                {chapter.title}
              </button>
            ))
          )}
        </nav>
      </aside>

      <section className="workspace-main">
        {error && !showNewChapter && (
          <div className="workspace-alert" role="alert">
            <span>{error}</span>
            <button type="button" onClick={onDismissError}>关闭</button>
          </div>
        )}
        {activeChapter ? (
          <ChapterEditor
            key={activeChapter.id}
            chapter={activeChapter}
            client={client}
            onChapterUpdated={onChapterUpdated}
            onFlushAvailable={onFlushAvailable}
          />
        ) : (
          <div className="editor-empty">
            <Feather size={36} aria-hidden="true" />
            <h2>选择一个章节开始创作</h2>
            <p>章节的短周期草稿会自动保存，正式正文仍由版本提交保护。</p>
          </div>
        )}
      </section>

      {showNewChapter && (
        <div className="dialog-backdrop">
          <form className="project-dialog" role="dialog" aria-label="新建章节" onSubmit={submitChapter}>
            <p className="eyebrow">章节结构</p>
            <h2>新建章节</h2>
            {error && <p role="alert" className="entry-error">{error}</p>}
            <label>
              章节标题
              <input
                autoFocus
                value={chapterTitle}
                onChange={(event) => setChapterTitle(event.target.value)}
              />
            </label>
            <div className="dialog-actions">
              <button
                type="button"
                className="secondary-action"
                disabled={creatingChapter}
                onClick={() => setShowNewChapter(false)}
              >
                取消
              </button>
              <button type="submit" className="primary-action" disabled={creatingChapter}>
                {creatingChapter ? "正在创建…" : "确认新建"}
              </button>
            </div>
          </form>
        </div>
      )}
    </main>
  );
}
