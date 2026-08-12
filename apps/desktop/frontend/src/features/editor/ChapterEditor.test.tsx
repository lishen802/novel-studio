import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { RevisionConflictError } from "../../lib/studio-client";
import { ChapterEditor } from "./ChapterEditor";

describe("ChapterEditor", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("saves a recovery draft after 200ms without changing revision", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = client.createChapterResult;
    render(<ChapterEditor chapter={chapter} client={client} />);

    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), {
      target: { value: "风从城门外吹来。" },
    });
    expect(screen.getByText("草稿保存中")).toBeInTheDocument();

    await act(async () => vi.advanceTimersByTimeAsync(200));

    expect(client.saveDraftCalls).toEqual([
      {
        chapterId: chapter.id,
        body: "风从城门外吹来。",
        baseRevision: 1,
      },
    ]);
    expect(screen.getByText("草稿已保存 · 版本 1")).toBeInTheDocument();
  });

  it("replaces the local revision after the 750ms commit", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterResult = {
      ...client.createChapterResult,
      body: "新的正文",
      revision: 2,
    };
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);

    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), {
      target: { value: "新的正文" },
    });
    await act(async () => vi.advanceTimersByTimeAsync(750));

    expect(client.commitChapterCalls).toEqual([
      {
        chapterId: "chapter-1",
        body: "新的正文",
        baseRevision: 1,
      },
    ]);
    expect(screen.getByText("已提交 · 版本 2")).toBeInTheDocument();
  });

  it("queues newer input while one chapter commit is in flight", async () => {
    const client = FakeStudioClient.withReadyProject();
    let finishFirst: ((chapter: typeof client.createChapterResult) => void) | undefined;
    client.commitChapter = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishFirst = resolve;
          }),
      )
      .mockImplementationOnce(async (_id, body) => ({
        ...client.createChapterResult,
        body,
        revision: 3,
      }));
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });

    fireEvent.change(editor, { target: { value: "第一版输入" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));
    fireEvent.change(editor, { target: { value: "请求期间的新输入" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));

    expect(client.commitChapter).toHaveBeenCalledTimes(1);
    await act(async () => {
      finishFirst?.({
        ...client.createChapterResult,
        body: "第一版输入",
        revision: 2,
      });
      await Promise.resolve();
    });

    expect(client.commitChapter).toHaveBeenCalledTimes(2);
    expect(client.commitChapter).toHaveBeenLastCalledWith(
      "chapter-1",
      "请求期间的新输入",
      2,
    );
    expect(client.saveDraftCalls).toContainEqual({
      chapterId: "chapter-1",
      body: "请求期间的新输入",
      baseRevision: 2,
    });
  });

  it("flushes the current recovery draft when leaving a chapter", () => {
    const client = FakeStudioClient.withReadyProject();
    const view = render(
      <ChapterEditor chapter={client.createChapterResult} client={client} />,
    );

    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), {
      target: { value: "切换前文本" },
    });
    view.unmount();

    expect(client.saveDraftCalls).toEqual([
      {
        chapterId: "chapter-1",
        body: "切换前文本",
        baseRevision: 1,
      },
    ]);
  });

  it("flushes the old chapter identity when the component is reused", async () => {
    const client = FakeStudioClient.withReadyProject();
    let registeredFlush: (() => Promise<void>) | null = null;
    const onFlushAvailable = (flush: (() => Promise<void>) | null) => {
      if (flush) {
        registeredFlush = flush;
      } else {
        void registeredFlush?.();
      }
    };
    const view = render(
      <ChapterEditor
        chapter={client.createChapterResult}
        client={client}
        onFlushAvailable={onFlushAvailable}
      />,
    );
    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), {
      target: { value: "第一章切换前文本" },
    });

    view.rerender(
      <ChapterEditor
        chapter={{
          ...client.createChapterResult,
          id: "chapter-2",
          title: "第二章",
        }}
        client={client}
        onFlushAvailable={onFlushAvailable}
      />,
    );
    await act(async () => Promise.resolve());

    expect(client.saveDraftCalls).toContainEqual({
      chapterId: "chapter-1",
      body: "第一章切换前文本",
      baseRevision: 1,
    });
    expect(client.saveDraftCalls).not.toContainEqual(
      expect.objectContaining({ chapterId: "chapter-2", body: "第一章切换前文本" }),
    );
  });

  it("preserves local text and offers reload after a revision conflict", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new RevisionConflictError();
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });

    fireEvent.change(editor, { target: { value: "不能丢失的本地文本" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));

    expect(editor).toHaveValue("不能丢失的本地文本");
    expect(screen.getByRole("alert")).toHaveTextContent("版本冲突");
    expect(
      screen.getByRole("button", { name: "重新加载服务器版本" }),
    ).toBeEnabled();
  });

  it("safely saves one recovery draft when a commit fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new Error("PROJECT_SHARING_VIOLATION|写入遇到风险");
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });
    fireEvent.change(editor, { target: { value: "必须保留的文本" } });
    await act(async () => vi.advanceTimersByTimeAsync(200));
    await act(async () => vi.advanceTimersByTimeAsync(550));
    expect(editor).toHaveValue("必须保留的文本");
    expect(client.saveDraftCalls).toHaveLength(2);
    expect(client.saveDraftCalls).toContainEqual({ chapterId: "chapter-1", body: "必须保留的文本", baseRevision: 1 });
    expect(screen.getByRole("alert")).toHaveTextContent("恢复草稿已安全保存");
  });

  it("tells the author to copy text when draft recovery also fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new Error("PROJECT_SHARING_VIOLATION|写入遇到风险");
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });
    fireEvent.change(editor, { target: { value: "无法写入的文本" } });
    await act(async () => vi.advanceTimersByTimeAsync(200));
    client.saveDraftError = new Error("PROJECT_SHARING_VIOLATION|写入遇到风险");
    await act(async () => vi.advanceTimersByTimeAsync(550));
    expect(editor).toHaveValue("无法写入的文本");
    expect(screen.getByRole("alert")).toHaveTextContent("无法保存恢复草稿；请复制文本");
  });

  it("retries the failed commit and copies the retained text", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new Error("PROJECT_SHARING_VIOLATION|写入遇到风险");
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), { target: { value: "可恢复正文" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));
    fireEvent.click(screen.getByRole("button", { name: "复制文本" }));
    expect(writeText).toHaveBeenCalledWith("可恢复正文");
    client.commitChapterError = null;
    fireEvent.click(screen.getByRole("button", { name: "重试保存" }));
    await act(async () => Promise.resolve());
    expect(client.commitChapterCalls).toEqual([
      { chapterId: "chapter-1", body: "可恢复正文", baseRevision: 1 },
      { chapterId: "chapter-1", body: "可恢复正文", baseRevision: 1 },
    ]);
  });

  it("does not overwrite newer local input while conflict reload is pending", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new RevisionConflictError();
    let finishReload: ((chapter: typeof client.openChapterResult) => void) | undefined;
    client.openChapter = vi.fn(
      () =>
        new Promise<typeof client.openChapterResult>((resolve) => {
          finishReload = resolve;
        }),
    );
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });

    fireEvent.change(editor, { target: { value: "冲突时的本地文本" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));
    fireEvent.click(screen.getByRole("button", { name: "重新加载服务器版本" }));
    fireEvent.change(editor, { target: { value: "等待重载时继续输入的文本" } });
    await act(async () => {
      finishReload?.({
        ...client.openChapterResult,
        body: "服务器正文",
        revision: 2,
      });
      await Promise.resolve();
    });

    expect(editor).toHaveValue("等待重载时继续输入的文本");
    expect(screen.getByText("版本冲突 · 本地文本已保留")).toBeInTheDocument();
  });

  it("detects edits that return to the same text while reload is pending", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.commitChapterError = new RevisionConflictError();
    let finishReload: ((chapter: typeof client.openChapterResult) => void) | undefined;
    client.openChapter = vi.fn(
      () =>
        new Promise<typeof client.openChapterResult>((resolve) => {
          finishReload = resolve;
        }),
    );
    render(<ChapterEditor chapter={client.createChapterResult} client={client} />);
    const editor = screen.getByRole("textbox", { name: "章节正文" });

    fireEvent.change(editor, { target: { value: "冲突正文" } });
    await act(async () => vi.advanceTimersByTimeAsync(750));
    fireEvent.click(screen.getByRole("button", { name: "重新加载服务器版本" }));
    fireEvent.change(editor, { target: { value: "临时输入" } });
    fireEvent.change(editor, { target: { value: "冲突正文" } });
    await act(async () => {
      finishReload?.({
        ...client.openChapterResult,
        body: "服务器正文",
        revision: 2,
      });
      await Promise.resolve();
    });

    expect(editor).toHaveValue("冲突正文");
    expect(screen.getByText("版本冲突 · 本地文本已保留")).toBeInTheDocument();
  });

  it("asks whether to restore a recovery draft", () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = {
      ...client.createChapterResult,
      body: "已提交正文",
      recoveryBody: "崩溃前草稿",
      hasRecoveryDraft: true,
    };
    render(<ChapterEditor chapter={chapter} client={client} />);

    expect(screen.getByRole("dialog", { name: "发现恢复草稿" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveAttribute(
      "readonly",
    );
    fireEvent.click(screen.getByRole("button", { name: "恢复草稿" }));

    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveValue(
      "崩溃前草稿",
    );
    expect(screen.getByRole("textbox", { name: "章节正文" })).not.toHaveAttribute(
      "readonly",
    );
  });

  it("keeps deferred recovery read-only until the author decides", () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = {
      ...client.createChapterResult,
      body: "已提交正文",
      recoveryBody: "崩溃前草稿",
      recoveryUpdatedAt: "2026-07-15T08:05:00Z",
      hasRecoveryDraft: true,
    };
    render(<ChapterEditor chapter={chapter} client={client} />);

    fireEvent.click(screen.getByRole("button", { name: "稍后决定" }));

    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveAttribute(
      "readonly",
    );
    fireEvent.click(screen.getByRole("button", { name: "处理恢复草稿" }));
    expect(screen.getByRole("dialog", { name: "发现恢复草稿" })).toBeInTheDocument();
  });

  it("deletes only the recovery draft after explicit confirmation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = {
      ...client.createChapterResult,
      body: "已提交正文",
      recoveryBody: "将被丢弃的草稿",
      recoveryUpdatedAt: "2026-07-15T08:05:00Z",
      hasRecoveryDraft: true,
    };
    render(<ChapterEditor chapter={chapter} client={client} />);

    fireEvent.click(screen.getByRole("button", { name: "保留已提交版" }));
    fireEvent.click(screen.getByRole("button", { name: "确认丢弃恢复草稿" }));
    await act(async () => Promise.resolve());

    expect(client.discardDraftCalls).toEqual([
      { chapterId: chapter.id, baseRevision: chapter.revision },
    ]);
    expect(screen.queryByRole("dialog", { name: "发现恢复草稿" })).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "章节正文" })).not.toHaveAttribute(
      "readonly",
    );
  });

  it("forwards selection range and revision when the author asks AI to review a passage", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = {
      ...client.createChapterResult,
      body: "潮声盖过脚步。",
      revision: 7,
    };
    const onSelectionReview = vi.fn();
    const user = userEvent.setup();
    render(
      <ChapterEditor
        chapter={chapter}
        client={client}
        onSelectionReview={onSelectionReview}
      />,
    );

    const editor = screen.getByRole("textbox", { name: "章节正文" }) as HTMLTextAreaElement;
    editor.setSelectionRange(0, 2);
    fireEvent.click(screen.getByRole("button", { name: "告诉 AI 这里有问题" }));

    expect(onSelectionReview).toHaveBeenCalledWith({
      chapterId: chapter.id,
      baseRevision: 7,
      start: 0,
      end: 2,
      text: "潮声",
    });
  });

  it("hides the selection review trigger when no text is selected", () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = {
      ...client.createChapterResult,
      body: "潮声盖过脚步。",
      revision: 7,
    };
    render(<ChapterEditor chapter={chapter} client={client} />);
    expect(
      screen.queryByRole("button", { name: "告诉 AI 这里有问题" }),
    ).not.toBeInTheDocument();
  });
});
