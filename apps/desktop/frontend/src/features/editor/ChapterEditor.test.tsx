import { act, fireEvent, render, screen } from "@testing-library/react";
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
    fireEvent.click(screen.getByRole("button", { name: "恢复草稿" }));

    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveValue(
      "崩溃前草稿",
    );
  });
});
