import { Feather, Lightbulb, Package, Sparkles, Wrench } from "lucide-react";
import type { ReactNode } from "react";
import type {
  ChapterView,
  SnapshotRestoreView,
  StudioClient,
  WorkbenchProjection,
} from "../../lib/studio-client";
import { ChapterEditor } from "../editor/ChapterEditor";
import { AgentWorkbench } from "../agents/AgentWorkbench";
import { WorldAssetsWorkbench } from "../assets/WorldAssetsWorkbench";
import { ContinuityWorkbench } from "../continuity/ContinuityWorkbench";
import { ImportWorkbench } from "../imports/ImportWorkbench";
import { ChangeReviewWorkbench } from "../revisions/ChangeReviewWorkbench";
import { VersionHistoryWorkbench } from "../revisions/VersionHistoryWorkbench";
import { BranchWorkbench } from "../revisions/BranchWorkbench";
import { StoryArchitecture } from "../story/StoryArchitecture";
import { StoryWorldWorkbench } from "../world/StoryWorldWorkbench";
import type { ProjectPage, WorkspaceStatus } from "./workspace-store";

type ReadyWorkspace = Extract<WorkspaceStatus, { kind: "ready" }>;
type WorkspaceReplacement = Pick<SnapshotRestoreView, "project" | "chapters">;

export type WorkbenchRouterProps = {
  // 权威工作台投影；stage/canvas/counts 全部来自 client.projectWorkbench()，
  // 不允许在此处用本地模式猜测。
  projection: WorkbenchProjection | null;
  page: ProjectPage;
  workspace: ReadyWorkspace;
  error: string | null;
  client: StudioClient;
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
  onNavigate(page: ProjectPage): void;
};

// NotReadyCanvas 展示诚实的"未就绪"状态，不伪造已完成内容
function NotReadyCanvas({
  title,
  reason,
  hint,
}: {
  title: string;
  reason: string;
  hint: string;
}) {
  return (
    <section className="workbench-canvas workbench-canvas--not-ready" aria-live="polite">
      <header className="workbench-canvas__header">
        <Feather size={28} aria-hidden="true" />
        <h2>{title}</h2>
      </header>
      <p className="workbench-canvas__reason">{reason}</p>
      <p className="workbench-canvas__hint">{hint}</p>
    </section>
  );
}

// UnavailablePage 展示尚未在当前阶段提供的页面占位
function UnavailablePage({
  icon: Icon,
  title,
  description,
}: {
  icon: typeof Feather;
  title: string;
  description: string;
}) {
  return (
    <section className="workbench-canvas workbench-canvas--unavailable" aria-live="polite">
      <header className="workbench-canvas__header">
        <Icon size={28} aria-hidden="true" />
        <h2>{title}</h2>
      </header>
      <p className="workbench-canvas__reason">尚未在当前阶段提供</p>
      <p className="workbench-canvas__hint">{description}</p>
    </section>
  );
}

export function WorkbenchRouter({
  projection,
  page,
  workspace,
  client,
  onSelectChapter,
  onChapterUpdated,
  onChaptersImported,
  onWorkspaceRestored,
  onBeforeVersionOperation,
  onFlushAvailable,
  onRefreshFilesystemHealth,
  onNavigate,
}: WorkbenchRouterProps): ReactNode {
  const activeChapter = workspace.chapters.find(
    (chapter) => chapter.id === workspace.activeChapterId,
  );
  const readOnly = workspace.project.accessMode === "read_only";

  // 当 page 切换到 manuscript 时，遵循权威 canvas；不属于 manuscript 画布时
  // 显示诚实的"未就绪"状态，不退回本地模式猜测。
  if (page === "manuscript") {
    const canvas = projection?.canvas ?? "";
    if (canvas !== "manuscript") {
      return (
        <NotReadyCanvas
          title="正文与章节"
          reason="当前作品阶段尚未进入正文写作。"
          hint="完成导入或前置阶段后，工作台会自动切换到正文画布。"
        />
      );
    }
    return activeChapter ? (
      <ChapterEditor
        key={activeChapter.id}
        chapter={activeChapter}
        client={client}
        onChapterUpdated={onChapterUpdated}
        onFlushAvailable={onFlushAvailable}
        onFilesystemHealthChanged={onRefreshFilesystemHealth}
        readOnly={readOnly}
      />
    ) : (
      <div className="editor-empty">
        <Feather size={36} aria-hidden="true" />
        <h2>选择一个章节开始创作</h2>
        <p>章节的短周期草稿会自动保存，正式正文仍由版本提交保护。</p>
      </div>
    );
  }

  switch (page) {
    case "dashboard":
      return (
        <AgentWorkbench
          client={client}
          chapters={workspace.chapters}
          onOpenChanges={() => onNavigate("changes")}
        />
      );

    case "changes":
      return (
        <ChangeReviewWorkbench
          client={client}
          chapters={workspace.chapters}
          onChapterUpdated={onChapterUpdated}
        />
      );

    case "imports":
      return (
        <ImportWorkbench
          client={client}
          onChaptersImported={onChaptersImported}
        />
      );

    case "story":
      return (
        <StoryArchitecture
          client={client}
          chapters={workspace.chapters}
          onOpenChapter={(id) => {
            onNavigate("manuscript");
            void onSelectChapter(id);
          }}
        />
      );

    case "characters":
      return <StoryWorldWorkbench client={client} chapters={workspace.chapters} />;

    case "world":
      return <WorldAssetsWorkbench client={client} chapters={workspace.chapters} />;

    case "continuity":
      return <ContinuityWorkbench client={client} chapters={workspace.chapters} />;

    case "versions":
      return (
        <VersionHistoryWorkbench
          client={client}
          project={workspace.project}
          chapters={workspace.chapters}
          onChapterUpdated={onChapterUpdated}
          onWorkspaceRestored={onWorkspaceRestored}
          onBeforeVersionOperation={onBeforeVersionOperation}
        />
      );

    case "branches":
      return (
        <BranchWorkbench
          client={client}
          onBeforeVersionOperation={onBeforeVersionOperation}
          onWorkspaceChanged={(result) => onWorkspaceRestored(result)}
        />
      );

    case "proposal":
      return (
        <UnavailablePage
          icon={Lightbulb}
          title="故事提案"
          description="三套原创方向与提案生成将在后续能力阶段开放。"
        />
      );

    case "foreshadowing":
      return (
        <UnavailablePage
          icon={Sparkles}
          title="伏笔与回收"
          description="独立的伏笔追踪与回收看板将随档案能力一起提供。"
        />
      );

    case "export":
      return (
        <UnavailablePage
          icon={Package}
          title="导出与发布"
          description="导出到常见文稿格式的能力将在后续阶段交付。"
        />
      );

    case "maintenance":
      return (
        <UnavailablePage
          icon={Wrench}
          title="项目维护"
          description="项目维护工具将在后续阶段交付。"
        />
      );

    default:
      return (
        <NotReadyCanvas
          title="未就绪"
          reason="尚未在当前阶段提供。"
          hint="该页面将随后续能力阶段开放。"
        />
      );
  }
}
