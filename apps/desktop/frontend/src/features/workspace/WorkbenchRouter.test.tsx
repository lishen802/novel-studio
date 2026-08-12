import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChapterView,
  WorkbenchProjection,
} from "../../lib/studio-client";
import { WorkbenchRouter } from "./WorkbenchRouter";
import type { ProjectPage } from "./workspace-store";

function chapter(overrides: Partial<ChapterView> = {}): ChapterView {
  return {
    id: "chapter-1",
    projectId: "project-1",
    title: "第一章",
    position: 1,
    body: "雾从海面升起。",
    revision: 1,
    updatedAt: "2026-08-12T08:00:00Z",
    hasRecoveryDraft: false,
    ...overrides,
  };
}

function manuscriptProjection(): WorkbenchProjection {
  return {
    stage: "writing",
    canvas: "manuscript",
    aiPanel: "hidden",
    pendingReviewCount: 0,
    recoverableJobCount: 0,
    archiveStatus: "not_generated",
  };
}

function ideaNotAvailableProjection(): WorkbenchProjection {
  return {
    stage: "idea",
    canvas: "idea_not_available",
    aiPanel: "hidden",
    pendingReviewCount: 0,
    recoverableJobCount: 0,
    archiveStatus: "not_generated",
  };
}

function readyWorkspace(chapters: ChapterView[] = []) {
  const client = FakeStudioClient.withReadyProject();
  return {
    workspace: {
      kind: "ready" as const,
      project: client.openProjectResult,
      chapters,
      activeChapterId: chapters[0]?.id ?? null,
    },
    client,
  };
}

function renderRouter({
  page,
  projection,
  chapters = [chapter()],
  activeChapterId,
}: {
  page: ProjectPage;
  projection: WorkbenchProjection | null;
  chapters?: ChapterView[];
  activeChapterId?: string | null;
}) {
  const { workspace, client } = readyWorkspace(chapters);
  const ws = activeChapterId === undefined ? workspace : { ...workspace, activeChapterId };
  return render(
    <WorkbenchRouter
      projection={projection}
      page={page}
      workspace={ws}
      client={client}
      error={null}
      onCreateChapter={vi.fn()}
      onSelectChapter={vi.fn()}
      onChapterUpdated={vi.fn()}
      onChaptersImported={vi.fn()}
      onWorkspaceRestored={vi.fn()}
      onBeforeVersionOperation={vi.fn()}
      onDismissError={vi.fn()}
      onFlushAvailable={vi.fn()}
      onCreateRecoveryCopy={vi.fn()}
      onRefreshFilesystemHealth={vi.fn()}
      onNavigate={vi.fn()}
      onStartWholeBookArchive={vi.fn()}
    />,
  );
}

describe("WorkbenchRouter", () => {
  it("routes to the chapter editor when projection canvas is manuscript", () => {
    renderRouter({
      page: "manuscript",
      projection: manuscriptProjection(),
      chapters: [chapter()],
      activeChapterId: "chapter-1",
    });

    // ChapterEditor 渲染章节正文文本框
    expect(screen.getByRole("textbox", { name: "章节正文" })).toBeInTheDocument();
  });

  it("shows honest not-ready state when manuscript canvas is not available", () => {
    renderRouter({
      page: "manuscript",
      projection: ideaNotAvailableProjection(),
    });

    expect(screen.getByRole("heading", { name: "正文与章节" })).toBeVisible();
    expect(screen.getByText("当前作品阶段尚未进入正文写作。")).toBeInTheDocument();
    // 不应该出现章节编辑器
    expect(screen.queryByRole("textbox", { name: "章节正文" })).not.toBeInTheDocument();
  });

  it("shows not-ready state when projection is null instead of guessing canvas", () => {
    renderRouter({
      page: "manuscript",
      projection: null,
    });

    expect(screen.getByRole("heading", { name: "正文与章节" })).toBeVisible();
    expect(screen.getByText("当前作品阶段尚未进入正文写作。")).toBeInTheDocument();
    expect(screen.queryByRole("textbox", { name: "章节正文" })).not.toBeInTheDocument();
  });

  it("does not render a local-mode build tab when canvas is manuscript", () => {
    renderRouter({
      page: "manuscript",
      projection: manuscriptProjection(),
      chapters: [chapter()],
      activeChapterId: "chapter-1",
    });

    expect(screen.queryByRole("tab", { name: "构建模式" })).not.toBeInTheDocument();
  });

  it("shows honest unavailable page for proposal stage", () => {
    renderRouter({
      page: "proposal",
      projection: manuscriptProjection(),
    });

    expect(screen.getByRole("heading", { name: "故事提案" })).toBeVisible();
    expect(screen.getByText("尚未在当前阶段提供")).toBeInTheDocument();
  });

  it("shows honest unavailable page for foreshadowing, export, and maintenance", () => {
    const { rerender } = renderRouter({
      page: "foreshadowing",
      projection: manuscriptProjection(),
    });
    expect(screen.getByRole("heading", { name: "伏笔与回收" })).toBeVisible();
    expect(screen.getByText("尚未在当前阶段提供")).toBeInTheDocument();

    rerender(
      <WorkbenchRouter
        projection={manuscriptProjection()}
        page="export"
        workspace={readyWorkspace([chapter()]).workspace}
        client={readyWorkspace([chapter()]).client}
        error={null}
        onCreateChapter={vi.fn()}
        onSelectChapter={vi.fn()}
        onChapterUpdated={vi.fn()}
        onChaptersImported={vi.fn()}
        onWorkspaceRestored={vi.fn()}
        onBeforeVersionOperation={vi.fn()}
        onDismissError={vi.fn()}
        onFlushAvailable={vi.fn()}
        onCreateRecoveryCopy={vi.fn()}
        onRefreshFilesystemHealth={vi.fn()}
        onNavigate={vi.fn()}
        onStartWholeBookArchive={vi.fn()}
      />,
    );
    expect(screen.getByRole("heading", { name: "导出与发布" })).toBeVisible();
  });

  it("routes the dashboard page to the agent workbench", () => {
    renderRouter({
      page: "dashboard",
      projection: manuscriptProjection(),
    });

    expect(screen.getByRole("heading", { name: "章节智能创作" })).toBeVisible();
  });
});
