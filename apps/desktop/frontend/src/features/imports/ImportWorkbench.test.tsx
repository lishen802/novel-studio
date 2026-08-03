import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import type {
  ChapterView,
  ImportApplyView,
  ImportJobView,
} from "../../lib/studio-client";
import { ImportWorkbench } from "./ImportWorkbench";

function importJob(
  overrides: Partial<ImportJobView> = {},
): ImportJobView {
  return {
    id: "import-1",
    status: "prepared",
    sourceName: "长夜.md",
    sourceFormat: "markdown",
    sourceEncoding: "UTF-8",
    sourceHashDigest: "0123456789ab",
    sourceBytes: 2048,
    chapterCount: 2,
    selectedChapterCount: 0,
    errorCode: "",
    errorMessage: "",
    createdAt: "2026-08-03T08:00:00Z",
    updatedAt: "2026-08-03T08:00:00Z",
    appliedAt: "",
    chapters: [
      {
        id: "candidate-1",
        volumeTitle: "第一卷 雾港",
        title: "第一章 潮声",
        body: "潮声越过防波堤。\n守灯人回头。",
        position: 1,
        sourceStartLine: 3,
        sourceEndLine: 5,
        contentHashSummary: "abcdef012345",
        selected: false,
        createdChapterId: "",
      },
      {
        id: "candidate-2",
        volumeTitle: "第二卷 北境",
        title: "第二章 雪线",
        body: "雪落在旧界碑上。",
        position: 2,
        sourceStartLine: 9,
        sourceEndLine: 10,
        contentHashSummary: "123456abcdef",
        selected: false,
        createdChapterId: "",
      },
    ],
    ...overrides,
  };
}

describe("ImportWorkbench", () => {
  it("loads the initial task list and explains the empty state", async () => {
    const client = FakeStudioClient.withReadyProject();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(
      screen.getByRole("heading", { name: "导入分析" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("暂无导入任务")).toBeInTheDocument();
    expect(client.importJobsCalls).toBe(1);
  });

  it("shows a safe Chinese error state when initial loading fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsError = new Error(
      "SELECT * FROM imports /Users/author/private.md",
    );

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "导入操作失败，请重试。",
    );
    expect(screen.queryByText(/SELECT|\/Users\/author/)).not.toBeInTheDocument();
  });

  it("treats file picker cancellation as a quiet no-op", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.manuscriptToImport = null;
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    await screen.findByText("暂无导入任务");

    await user.click(screen.getByRole("button", { name: "选择小说文件" }));

    expect(client.prepareManuscriptImportCalls).toHaveLength(0);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("selects a newly prepared task and shows only safe source metadata", async () => {
    const client = FakeStudioClient.withReadyProject();
    const prepared = importJob();
    client.manuscriptToImport = "/Users/author/Documents/private/长夜.md";
    client.prepareManuscriptImportResult = { job: prepared };
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    await screen.findByText("暂无导入任务");

    await user.click(screen.getByRole("button", { name: "选择小说文件" }));

    expect(await screen.findAllByText("长夜.md")).toHaveLength(2);
    expect(screen.getByText("Markdown")).toBeInTheDocument();
    expect(screen.getByText("UTF-8")).toBeInTheDocument();
    expect(screen.getByText("2 KiB")).toBeInTheDocument();
    expect(screen.getByText("0123456789ab")).toBeInTheDocument();
    expect(screen.getAllByText("待导入")).toHaveLength(2);
    expect(screen.queryByText(/\/Users\/author/)).not.toBeInTheDocument();
    expect(client.prepareManuscriptImportCalls).toEqual([
      "/Users/author/Documents/private/长夜.md",
    ]);
  });

  it("groups chapter previews by volume and supports chapter and bulk selection", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [importJob()];
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(await screen.findByText("第一卷 雾港")).toBeInTheDocument();
    expect(screen.getByText("第二卷 北境")).toBeInTheDocument();
    const first = screen.getByRole("group", { name: "第一章 潮声" });
    expect(within(first).getByText(/原文第\s*3–5\s*行/)).toBeInTheDocument();
    expect(within(first).getByText("潮声越过防波堤。")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "导入所选章节" }),
    ).toBeDisabled();

    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    expect(
      screen.getByRole("button", { name: "导入所选章节" }),
    ).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "全选章节" }));
    expect(screen.getAllByRole("checkbox")).toHaveLength(2);
    for (const checkbox of screen.getAllByRole("checkbox")) {
      expect(checkbox).toBeChecked();
    }
    await user.click(screen.getByRole("button", { name: "取消全选" }));
    expect(
      screen.getByRole("button", { name: "导入所选章节" }),
    ).toBeDisabled();
  });

  it("applies selected chapters once and refreshes the workspace with the complete chapter list", async () => {
    const client = FakeStudioClient.withReadyProject();
    const prepared = importJob();
    client.importJobsResult = [prepared];
    const created: ChapterView = {
      id: "chapter-imported",
      projectId: "project-1",
      title: "第一章 潮声",
      position: 1,
      body: prepared.chapters[0].body,
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const applied: ImportApplyView = {
      job: {
        ...prepared,
        status: "applied",
        selectedChapterCount: 1,
        appliedAt: "2026-08-03T08:01:00Z",
        chapters: prepared.chapters.map((chapter, index) =>
          index === 0
            ? {
                ...chapter,
                selected: true,
                createdChapterId: created.id,
              }
            : chapter,
        ),
      },
      chapters: [created],
    };
    client.importApplyResult = applied;
    const onChaptersImported = vi.fn();
    const user = userEvent.setup();
    render(
      <ImportWorkbench
        client={client}
        onChaptersImported={onChaptersImported}
      />,
    );
    const first = await screen.findByRole("group", {
      name: "第一章 潮声",
    });

    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    await user.click(screen.getByRole("button", { name: "导入所选章节" }));

    await waitFor(() =>
      expect(onChaptersImported).toHaveBeenCalledWith([created]),
    );
    expect(client.applyImportJobCalls).toEqual([
      { id: prepared.id, selectedChapterIds: ["candidate-1"] },
    ]);
    expect(screen.getAllByText("已导入")).toHaveLength(2);
    expect(
      screen.getByRole("button", { name: "导入所选章节" }),
    ).toBeDisabled();
    expect(screen.queryByRole("button", { name: "删除任务" })).not.toBeInTheDocument();
  });

  it("shows safe failed-task details and lets the author delete non-applied jobs", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [
      importJob({
        status: "failed",
        chapterCount: 0,
        chapters: [],
        errorCode: "IMPORT_PARSE_EMPTY",
        errorMessage: "No importable chapters were found.",
      }),
    ];
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(
      await screen.findByText("没有识别到可导入章节。"),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "删除任务" }));

    await waitFor(() =>
      expect(client.deleteImportJobCalls).toEqual(["import-1"]),
    );
    expect(screen.getByText("暂无导入任务")).toBeInTheDocument();
  });

  it("does not render unsafe details stored on a failed import task", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [
      importJob({
        status: "failed",
        chapterCount: 0,
        chapters: [],
        errorCode: "UNKNOWN_IMPORT_FAILURE",
        errorMessage: "SELECT source FROM /Users/author/private.md",
      }),
    ];

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(
      await screen.findByText("导入操作失败，请重试。"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/SELECT|\/Users\/author/)).not.toBeInTheDocument();
  });
});
