import {
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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
    extractionStatus: "none",
    extractionRunId: "",
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
        wordCount: 9876,
        bodyPreview: "潮声越过防波堤。\n守灯人回头。",
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
        wordCount: 8,
        bodyPreview: "雪落在旧界碑上。",
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

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
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
    expect(
      screen.getAllByRole("button", { name: "选择小说文件" }),
    ).toHaveLength(1);
    expect(client.importJobsCalls).toBe(1);
  });

  it("shows a safe Chinese error state when initial loading fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsError = new Error(
      "SELECT * FROM imports /Users/author/private.md",
    );
    const user = userEvent.setup();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "导入操作失败，请重试。",
    );
    expect(screen.queryByText(/SELECT|\/Users\/author/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    client.importJobsError = null;
    client.importJobsResult = [importJob()];
    const failedState = screen.getByText(
      "导入任务暂时无法读取。",
    ).parentElement;
    await user.click(
      within(failedState!).getByRole("button", {
        name: "刷新任务与详情",
      }),
    );

    expect(
      await screen.findByRole("heading", { name: "长夜.md" }),
    ).toBeInTheDocument();
    expect(client.importJobsCalls).toBe(2);
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
    expect(within(first).getByText(/9876 字/)).toBeInTheDocument();
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

  it("requires explicit import confirmation and cancellation has no side effects", async () => {
    const client = FakeStudioClient.withReadyProject();
    const prepared = importJob();
    client.importJobsResult = [prepared];
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    const first = await screen.findByRole("group", {
      name: "第一章 潮声",
    });

    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    const applyTrigger = screen.getByRole("button", {
      name: "导入所选章节",
    });
    await user.click(applyTrigger);

    const dialog = screen.getByRole("dialog", { name: "确认导入章节" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveTextContent("长夜.md");
    expect(dialog).toHaveTextContent("1 个章节");
    expect(dialog).toHaveTextContent("正式章节");
    expect(dialog).toHaveTextContent("任务将锁定");
    expect(client.applyImportJobCalls).toHaveLength(0);

    await user.click(within(dialog).getByRole("button", { name: "取消" }));

    expect(
      screen.queryByRole("dialog", { name: "确认导入章节" }),
    ).not.toBeInTheDocument();
    expect(client.applyImportJobCalls).toHaveLength(0);
    expect(applyTrigger).toHaveFocus();
  });

  it("moves focus into confirmation, traps Tab, and cancels with Escape", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [importJob()];
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    const first = await screen.findByRole("group", {
      name: "第一章 潮声",
    });

    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    await user.click(screen.getByRole("button", { name: "导入所选章节" }));

    const dialog = screen.getByRole("dialog", { name: "确认导入章节" });
    const cancel = within(dialog).getByRole("button", { name: "取消" });
    const confirm = within(dialog).getByRole("button", {
      name: "确认导入",
    });
    await waitFor(() => expect(cancel).toHaveFocus());
    await user.tab();
    expect(confirm).toHaveFocus();
    await user.tab();
    expect(cancel).toHaveFocus();
    await user.tab({ shift: true });
    expect(confirm).toHaveFocus();

    await user.keyboard("{Escape}");

    expect(
      screen.queryByRole("dialog", { name: "确认导入章节" }),
    ).not.toBeInTheDocument();
    expect(client.applyImportJobCalls).toHaveLength(0);
    expect(
      screen.getByRole("button", { name: "导入所选章节" }),
    ).toHaveFocus();
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
      body: "潮声越过防波堤。\n守灯人回头。",
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
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );

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
    expect(
      screen.getByRole("button", { name: /长夜\.md 已导入/ }),
    ).toHaveFocus();
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
    const deleteTrigger = screen.getByRole("button", { name: "删除任务" });
    await user.click(deleteTrigger);
    const dialog = screen.getByRole("dialog", {
      name: "确认删除导入任务",
    });
    expect(dialog).toHaveTextContent("长夜.md");
    expect(client.deleteImportJobCalls).toHaveLength(0);
    await user.click(within(dialog).getByRole("button", { name: "取消" }));
    expect(client.deleteImportJobCalls).toHaveLength(0);
    expect(deleteTrigger).toHaveFocus();
    await user.click(screen.getByRole("button", { name: "删除任务" }));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认删除导入任务" }),
      ).getByRole("button", { name: "确认删除" }),
    );

    await waitFor(() =>
      expect(client.deleteImportJobCalls).toEqual(["import-1"]),
    );
    expect(screen.getByText("暂无导入任务")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "选择小说文件" }),
    ).toHaveFocus();
  });

  it("focuses the next task after a successful delete", async () => {
    const client = FakeStudioClient.withReadyProject();
    const taskA = importJob({
      id: "import-a",
      sourceName: "任务A.md",
      status: "failed",
      chapterCount: 0,
      chapters: [],
      errorCode: "IMPORT_PARSE_EMPTY",
      errorMessage: "No importable chapters were found.",
    });
    const taskB = importJob({
      id: "import-b",
      sourceName: "任务B.md",
    });
    client.importJobsResult = [taskA, taskB];
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    expect(
      await screen.findByRole("heading", { name: "任务A.md" }),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "删除任务" }));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认删除导入任务" }),
      ).getByRole("button", { name: "确认删除" }),
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /任务B\.md 待导入/ }),
      ).toHaveFocus(),
    );
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

  it("hides task A details and actions while task B details are pending", async () => {
    const client = FakeStudioClient.withReadyProject();
    const taskA = importJob({
      id: "import-a",
      sourceName: "任务A.md",
    });
    const taskB = importJob({
      id: "import-b",
      sourceName: "任务B.md",
    });
    const taskBRequest = deferred<ImportJobView>();
    client.importJobsResult = [taskA, taskB];
    client.importJob = vi.fn((id: string) =>
      id === taskA.id ? Promise.resolve(taskA) : taskBRequest.promise,
    );
    const user = userEvent.setup();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(
      await screen.findByRole("heading", { name: "任务A.md" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /任务B\.md/ }));
    await waitFor(() =>
      expect(client.importJob).toHaveBeenCalledWith(taskB.id),
    );

    const loading = screen.getByText("正在读取章节预览…");
    expect(loading).toBeInTheDocument();
    expect(loading.closest(".import-detail")).toHaveAttribute(
      "aria-busy",
      "true",
    );
    expect(loading.parentElement).toHaveAttribute("aria-live", "polite");
    expect(
      screen.queryByRole("heading", { name: "任务A.md" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "删除任务" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "导入所选章节" }),
    ).not.toBeInTheDocument();
  });

  it("does not restore task A details or actions when task B loading fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    const taskA = importJob({
      id: "import-a",
      sourceName: "任务A.md",
    });
    const taskB = importJob({
      id: "import-b",
      sourceName: "任务B.md",
    });
    const taskBRequest = deferred<ImportJobView>();
    client.importJobsResult = [taskA, taskB];
    client.importJob = vi.fn((id: string) =>
      id === taskA.id ? Promise.resolve(taskA) : taskBRequest.promise,
    );
    const user = userEvent.setup();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(
      await screen.findByRole("heading", { name: "任务A.md" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /任务B\.md/ }));
    await waitFor(() =>
      expect(client.importJob).toHaveBeenCalledWith(taskB.id),
    );
    taskBRequest.reject(new Error("读取任务B失败"));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "读取任务B失败",
    );
    expect(
      screen.queryByRole("heading", { name: "任务A.md" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "删除任务" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "导入所选章节" }),
    ).not.toBeInTheDocument();
  });

  it("retries a transient detail failure and restores the selected task", async () => {
    const client = FakeStudioClient.withReadyProject();
    const task = importJob();
    client.importJobsResult = [task];
    let attempts = 0;
    client.importJob = vi.fn(async () => {
      attempts++;
      if (attempts === 1) throw new Error("暂时无法读取导入任务");
      return task;
    });
    const user = userEvent.setup();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "暂时无法读取导入任务",
    );
    await user.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    const failedState = screen.getByText(
      "任务详情暂时无法读取。",
    ).parentElement;
    await user.click(
      within(failedState!).getByRole("button", {
        name: "刷新任务与详情",
      }),
    );

    expect(
      await screen.findByRole("heading", { name: "长夜.md" }),
    ).toBeInTheDocument();
    expect(client.importJob).toHaveBeenCalledTimes(2);
  });

  it("offers the same task-and-detail refresh after an apply failure", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.importJobsResult = [importJob()];
    client.importMutationError = new Error("暂时无法导入章节");
    const user = userEvent.setup();
    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );
    const first = await screen.findByRole("group", {
      name: "第一章 潮声",
    });
    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    await user.click(screen.getByRole("button", { name: "导入所选章节" }));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "暂时无法导入章节",
    );

    client.importMutationError = null;
    await user.click(
      screen.getByRole("button", { name: "刷新任务与详情" }),
    );

    expect(
      await screen.findByRole("heading", { name: "长夜.md" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(client.importJobsCalls).toBe(2);
  });

  it("notifies the workspace after apply succeeds even if the workbench unmounts", async () => {
    const client = FakeStudioClient.withReadyProject();
    const prepared = importJob();
    client.importJobsResult = [prepared];
    const created: ChapterView = {
      id: "chapter-imported",
      projectId: "project-1",
      title: "第一章 潮声",
      position: 1,
      body: "潮声越过防波堤。",
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
    const applying = deferred<ImportApplyView>();
    client.applyImportJob = vi.fn(() => applying.promise);
    const onChaptersImported = vi.fn();
    const user = userEvent.setup();
    const { unmount } = render(
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
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );

    unmount();
    applying.resolve(applied);

    await waitFor(() =>
      expect(onChaptersImported).toHaveBeenCalledWith([created]),
    );
  });

  it("notifies after a pending apply without replacing a newly selected task", async () => {
    const firstClient = FakeStudioClient.withReadyProject();
    const prepared = importJob({
      id: "import-a",
      sourceName: "任务A.md",
    });
    firstClient.importJobsResult = [prepared];
    const created: ChapterView = {
      id: "chapter-imported",
      projectId: "project-1",
      title: "第一章 潮声",
      position: 1,
      body: "潮声越过防波堤。",
      revision: 1,
      updatedAt: "2026-08-03T08:01:00Z",
      hasRecoveryDraft: false,
    };
    const applying = deferred<ImportApplyView>();
    firstClient.applyImportJob = vi.fn(() => applying.promise);
    const onChaptersImported = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(
      <ImportWorkbench
        client={firstClient}
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
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );

    const secondClient = FakeStudioClient.withReadyProject();
    const replacement = importJob({
      id: "import-b",
      sourceName: "任务B.md",
    });
    secondClient.importJobsResult = [replacement];
    rerender(
      <ImportWorkbench
        client={secondClient}
        onChaptersImported={onChaptersImported}
      />,
    );
    expect(
      await screen.findByRole("heading", { name: "任务B.md" }),
    ).toBeInTheDocument();

    applying.resolve({
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
    });

    await waitFor(() =>
      expect(onChaptersImported).toHaveBeenCalledWith([created]),
    );
    expect(
      screen.getByRole("heading", { name: "任务B.md" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "任务A.md" }),
    ).not.toBeInTheDocument();
  });

  it("marks the selected task and disables task switching during apply", async () => {
    const client = FakeStudioClient.withReadyProject();
    const taskA = importJob({
      id: "import-a",
      sourceName: "任务A.md",
    });
    const taskB = importJob({
      id: "import-b",
      sourceName: "任务B.md",
    });
    client.importJobsResult = [taskA, taskB];
    const applying = deferred<ImportApplyView>();
    client.applyImportJob = vi.fn(() => applying.promise);
    const user = userEvent.setup();

    render(
      <ImportWorkbench client={client} onChaptersImported={vi.fn()} />,
    );

    const first = await screen.findByRole("group", {
      name: "第一章 潮声",
    });
    expect(
      screen.getByRole("button", { name: /任务A\.md/ }),
    ).toHaveAttribute("aria-current", "true");
    await user.click(
      within(first).getByRole("checkbox", { name: "选择 第一章 潮声" }),
    );
    await user.click(screen.getByRole("button", { name: "导入所选章节" }));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );

    expect(
      screen.getByRole("button", { name: /任务A\.md/ }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /任务B\.md/ }),
    ).toBeDisabled();
    await user.keyboard("{Escape}");
    expect(
      screen.getByRole("dialog", { name: "确认导入章节" }),
    ).toBeInTheDocument();
  });
});
