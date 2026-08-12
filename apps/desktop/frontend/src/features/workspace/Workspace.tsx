import { Feather, FilePlus2, LibraryBig } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";
import type {
  AIPanelMode,
  AIPanelSelection,
  ChapterView,
  SnapshotRestoreView,
  StudioClient,
  WorkbenchProjection,
} from "../../lib/studio-client";
import { AIPanel } from "../workbench/AIPanel";
import { ProjectFilesystemRiskBanner } from "./ProjectFilesystemRiskBanner";
import { ProjectNavigation } from "./ProjectNavigation";
import { WorkbenchRouter } from "./WorkbenchRouter";
import type { ProjectPage, WorkspaceStatus } from "./workspace-store";

type ReadyWorkspace = Extract<WorkspaceStatus, { kind: "ready" }>;
type WorkspaceReplacement = Pick<SnapshotRestoreView, "project" | "chapters">;

type WorkspaceProps = {
  workspace: ReadyWorkspace;
  error: string | null;
  client: StudioClient;
  // 当前选中的二级页面，来自 workspace store
  currentPage: ProjectPage;
  // 权威工作台投影，来自 workspace store
  workbench: WorkbenchProjection | null;
  // AI 面板草稿（按 project ID 存储），来自 workspace store
  aiPanelInput: string;
  // 当前待提交的选区意见，来自 workspace store
  pendingSelection: AIPanelSelection | null;
  onCreateChapter(title: string): Promise<boolean>;
  onSelectChapter(id: string): Promise<void>;
  onChapterUpdated(chapter: ChapterView): void;
  onChaptersImported(chapters: ChapterView[]): void;
  onWorkspaceRestored(result: WorkspaceReplacement): void;
  onBeforeVersionOperation(): Promise<void>;
  onDismissError(): void;
  onFlushAvailable(flush: (() => Promise<void>) | null): void;
  onCreateRecoveryCopy(): Promise<void>;
  onRefreshFilesystemHealth(): Promise<void>;
  onSetCurrentPage(page: ProjectPage): void;
  // AI 面板输入变更回调：workspace store 按 project ID 保存未发送输入
  onAIPanelInput(input: string): void;
  // AI 面板选区意见回调：ChapterEditor 触发，写入 pendingSelection
  onAIPanelSelection(selection: AIPanelSelection): void;
  // 清除选区意见：退出 selection_review 模式
  onClearAIPanelSelection(): void;
  // 整书档案生成入口：R2 返回 FEATURE_NOT_AVAILABLE
  onStartWholeBookArchive(): void;
};

export function Workspace({
  workspace,
  error,
  client,
  currentPage,
  workbench,
  aiPanelInput,
  pendingSelection,
  onCreateChapter,
  onSelectChapter,
  onChapterUpdated,
  onChaptersImported,
  onWorkspaceRestored,
  onBeforeVersionOperation,
  onDismissError,
  onFlushAvailable,
  onCreateRecoveryCopy,
  onRefreshFilesystemHealth,
  onSetCurrentPage,
  onAIPanelInput,
  onAIPanelSelection,
  onClearAIPanelSelection,
  onStartWholeBookArchive,
}: WorkspaceProps) {
  const [showNewChapter, setShowNewChapter] = useState(false);
  const [chapterTitle, setChapterTitle] = useState("");
  const [creatingChapter, setCreatingChapter] = useState(false);
  // 用户通过 Cmd+J 切换 AI 面板可见性时的本地覆盖。null 表示遵循 workbench
  // 投影的默认 mode；"expanded" / "hidden" 表示用户已切换。切换画布或打开
  // 应用设置时 Workspace 保持挂载，覆盖值不会丢失。
  const [aiPanelOverride, setAIPanelOverride] = useState<
    "expanded" | "hidden" | null
  >(null);
  const readOnly = workspace.project.accessMode === "read_only";

  // 计算当前生效的 AI 面板 mode：有待提交选区时强制 selection_review；
  // 否则使用用户覆盖值；最后回退到 workbench 投影的默认 mode。
  const workbenchAIMode = workbench?.aiPanel ?? "hidden";
  const effectiveAIMode: AIPanelMode = pendingSelection
    ? "selection_review"
    : (aiPanelOverride ?? workbenchAIMode);

  // Cmd+J 切换 AI 面板：hidden ↔ expanded；selection_review 模式下不切换。
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "j") {
        event.preventDefault();
        setAIPanelOverride((current) => {
          const currentMode = pendingSelection
            ? "selection_review"
            : (current ?? workbenchAIMode);
          if (currentMode === "selection_review") return current;
          if (currentMode === "hidden") return "expanded";
          return "hidden";
        });
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [workbenchAIMode, pendingSelection]);
  const filesystemHealth = workspace.project.filesystemHealth ?? {
    level: "normal" as const,
    category: "" as const,
    actions: [],
  };
  const openNewChapter = () => {
    if (readOnly) return;
    onDismissError();
    setShowNewChapter(true);
  };

  const submitChapter = async (event: FormEvent) => {
    event.preventDefault();
    if (readOnly || !chapterTitle.trim()) return;
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

        {readOnly && (
          <p className="workspace-read-only-notice" role="status">
            只读模式：可查看，不会写入或更新恢复数据。
          </p>
        )}
        <ProjectFilesystemRiskBanner
          health={filesystemHealth}
          onCreateRecoveryCopy={() => void onCreateRecoveryCopy()}
          recoveryCopying={workspace.recoveryCopying}
        />

        <ProjectNavigation
          currentPage={currentPage}
          onSelectPage={onSetCurrentPage}
          pendingReviewCount={workbench?.pendingReviewCount ?? 0}
          recoverableJobCount={workbench?.recoverableJobCount ?? 0}
        />

        {currentPage === "manuscript" && (
          <>
            <div className="chapter-list__header">
              <span>章节</span>
              <button
                type="button"
                aria-label="新建章节"
                disabled={readOnly}
                onClick={openNewChapter}
              >
                <FilePlus2 size={17} />
              </button>
            </div>

            <nav className="chapter-list" aria-label="章节列表">
              {workspace.chapters.length === 0 ? (
                <div className="chapter-empty">
                  <LibraryBig size={24} aria-hidden="true" />
                  <p>还没有章节</p>
                  <button type="button" disabled={readOnly} onClick={openNewChapter}>
                    新建第一章
                  </button>
                </div>
              ) : (
                workspace.chapters.map((chapter) => (
                  <button
                    type="button"
                    className={
                      chapter.id === workspace.activeChapterId
                        ? "is-active"
                        : ""
                    }
                    key={chapter.id}
                    onClick={() => void onSelectChapter(chapter.id)}
                  >
                    <span>{chapter.position.toString().padStart(2, "0")}</span>
                    {chapter.title}
                  </button>
                ))
              )}
            </nav>
          </>
        )}
      </aside>

      <section
        className={`workspace-main${readOnly ? " workspace-main--read-only" : ""}`}
        aria-readonly={readOnly || undefined}
      >
        {error && !showNewChapter && (
          <div className="workspace-alert" role="alert">
            <span>{error}</span>
            <button type="button" onClick={onDismissError}>
              关闭
            </button>
          </div>
        )}
        <fieldset className="workspace-main__content" disabled={readOnly}>
          <WorkbenchRouter
            projection={workbench}
            page={currentPage}
            workspace={workspace}
            client={client}
            error={error}
            onCreateChapter={onCreateChapter}
            onSelectChapter={onSelectChapter}
            onChapterUpdated={onChapterUpdated}
            onChaptersImported={onChaptersImported}
            onWorkspaceRestored={onWorkspaceRestored}
            onBeforeVersionOperation={onBeforeVersionOperation}
            onDismissError={onDismissError}
            onFlushAvailable={onFlushAvailable}
            onCreateRecoveryCopy={onCreateRecoveryCopy}
            onRefreshFilesystemHealth={onRefreshFilesystemHealth}
            onNavigate={onSetCurrentPage}
            onStartWholeBookArchive={onStartWholeBookArchive}
            onSelectionReview={onAIPanelSelection}
          />
        </fieldset>
      </section>

      <AIPanel
        projectId={workspace.project.id}
        mode={effectiveAIMode}
        input={aiPanelInput}
        onInput={onAIPanelInput}
        selection={pendingSelection}
        onSelectionReview={onAIPanelSelection}
        onDismissSelection={onClearAIPanelSelection}
        onSubmitSelectionReview={(payload) => {
          // R2 不实现选区意见提交：清除选区并返回结构化未实现状态。
          // 后续能力阶段会接入 STALE_SELECTION 校验与 agent 提交。
          onClearAIPanelSelection();
        }}
      />

      {showNewChapter && (
        <div className="dialog-backdrop">
          <form
            className="project-dialog"
            role="dialog"
            aria-label="新建章节"
            onSubmit={submitChapter}
          >
            <p className="eyebrow">章节结构</p>
            <h2>新建章节</h2>
            {error && (
              <p role="alert" className="entry-error">
                {error}
              </p>
            )}
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
              <button
                type="submit"
                className="primary-action"
                disabled={creatingChapter}
              >
                {creatingChapter ? "正在创建…" : "确认新建"}
              </button>
            </div>
          </form>
        </div>
      )}
    </main>
  );
}
