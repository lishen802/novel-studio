import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { App } from "../../app/App";
import { FakeStudioClient } from "../../lib/fake-studio-client";
import { StudioClientError } from "../../lib/studio-client";

describe("Workspace", () => {
  it("updates the persistent filesystem risk after a disk-full chapter commit failure", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    client.commitChapterError = new StudioClientError(
      "PROJECT_FILESYSTEM_DISK_FULL",
      "项目磁盘空间不足，请清理空间后重试。",
    );
    const filesystemHealth = vi.fn(async () => ({
      level: "degraded" as const,
      category: "disk_full" as const,
      actions: ["retry", "copy_text", "create_recovery_copy"] as const,
    }));
    Object.assign(client, { filesystemHealth });
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(await screen.findByRole("button", { name: /第一章/ }));
    fireEvent.change(screen.getByRole("textbox", { name: "章节正文" }), {
      target: { value: "不能丢失的正文" },
    });
    await act(
      async () =>
        await new Promise((resolve) => {
          setTimeout(resolve, 800);
        }),
    );

    expect(await screen.findByRole("status")).toHaveTextContent("项目写入遇到风险");
    expect(screen.getByRole("button", { name: "创建恢复副本" })).toBeEnabled();
    expect(filesystemHealth).toHaveBeenCalledOnce();
  });

  it("prevents a second recovery-copy request while the first is pending", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.openProjectResult = {
      ...client.openProjectResult,
      filesystemHealth: {
        level: "degraded",
        category: "sharing_violation",
        actions: ["create_recovery_copy"],
      },
    };
    let chooseParent: ((path: string | null) => void) | undefined;
    client.chooseProjectParent = vi.fn(
      () => new Promise<string | null>((resolve) => {
        chooseParent = resolve;
      }),
    );
    client.recoveryCopyResponse = {
      outcome: "published",
      path: "/copies/长夜织境-恢复副本.zjproject",
    };
    render(<App client={client} initialProject={client.openProjectResult} />);

    const action = await screen.findByRole("button", { name: "创建恢复副本" });
    fireEvent.click(action);
    expect(action).toBeDisabled();
    fireEvent.click(action);
    expect(client.chooseProjectParent).toHaveBeenCalledOnce();

    chooseParent?.("/copies");
    await waitFor(() => {
      expect(client.createProjectRecoveryCopyCalls).toEqual([
        {
          sourcePath: "/books/长夜织境.zjproject",
          parentDirectory: "/copies",
        },
      ]);
    });
  });

  it("keeps an editable workspace enabled for sync risk", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = { ...client.createChapterResult, body: "可继续编辑" };
    client.openProjectResult = {
      ...client.openProjectResult,
      filesystemHealth: { level: "warning", category: "sync_risk", actions: [] },
    };
    client.chaptersResult = [chapter];
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);
    await user.click(await screen.findByRole("button", { name: /第一章/ }));
    expect(screen.getByRole("status")).toHaveTextContent("项目文件系统存在风险");
    expect(screen.getByRole("textbox", { name: "章节正文" })).not.toHaveAttribute("readonly");
  });

  it("renders a read-only workspace without author save controls", async () => {
    const client = FakeStudioClient.withReadyProject();
    const chapter = { ...client.createChapterResult, body: "只读正文" };
    client.openProjectResult = { ...client.openProjectResult, accessMode: "read_only" };
    client.chaptersResult = [chapter];
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(await screen.findByRole("button", { name: /第一章/ }));

    expect(screen.getByRole("status")).toHaveTextContent("只读模式");
    expect(screen.getByRole("button", { name: "新建章节" })).toBeDisabled();
    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveAttribute("readonly");
  });

  it("creates a project once and enters an empty workspace", async () => {
    const client = new FakeStudioClient();
    client.projectParent = "/books";
    client.createProjectResult = {
      id: "project-1",
      title: "长夜织境",
      genre: "东方奇幻",
      summary: "失落城邦与梦境交叠。",
      path: "/books/长夜织境.zjproject",
      accessMode: "editable",
      filesystemHealth: { level: "normal", category: "", actions: [] },
    };
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(await screen.findByRole("button", { name: /从一个想法开始/ }));
    await user.type(screen.getByLabelText("书名"), "长夜织境");
    await user.type(screen.getByLabelText("类型"), "东方奇幻");
    await user.type(screen.getByLabelText("简介"), "失落城邦与梦境交叠。");
    await user.click(screen.getByRole("button", { name: "确认创建" }));

    expect(client.createProjectCalls).toEqual([
      {
        parentDirectory: "/books",
        title: "长夜织境",
        genre: "东方奇幻",
        summary: "失落城邦与梦境交叠。",
      },
    ]);
    expect(
      await screen.findByRole("heading", { name: "长夜织境" }),
    ).toBeInTheDocument();
    expect(screen.getByText("还没有章节")).toBeInTheDocument();
  });

  it("stays on the welcome page when directory selection is cancelled", async () => {
    const client = new FakeStudioClient();
    client.projectParent = null;
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(await screen.findByRole("button", { name: /从一个想法开始/ }));

    expect(
      screen.getByRole("heading", { name: "织境 Story Studio" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(client.createProjectCalls).toHaveLength(0);
  });

  it("creates and focuses a new chapter", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(await screen.findByRole("button", { name: "新建章节" }));
    await user.type(screen.getByLabelText("章节标题"), "第一章");
    await user.click(screen.getByRole("button", { name: "确认新建" }));

    const editor = await screen.findByRole("textbox", { name: "章节正文" });
    await waitFor(() => expect(editor).toHaveFocus());
    expect(client.createChapterCalls).toEqual(["第一章"]);
  });

  it("switches between manuscript and story architecture", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 故事架构 在 作品档案 折叠分组下，需要先展开
    await user.click(await screen.findByRole("button", { name: "作品档案" }));
    await user.click(await screen.findByRole("button", { name: "故事架构" }));
    expect(
      await screen.findByRole("heading", { name: "故事架构" }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "正文与章节" }));
    expect(screen.getByText("选择一个章节开始创作")).toBeInTheDocument();
  });

  it("opens manuscript import analysis from the product navigation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(
      await screen.findByRole("button", { name: "导入与续写" }),
    );

    expect(
      await screen.findByRole("heading", { name: "导入分析" }),
    ).toBeInTheDocument();
  });

  it("refreshes and focuses workspace chapters after applying an import", async () => {
    const client = FakeStudioClient.withReadyProject();
    const original = client.createChapterResult;
    client.chaptersResult = [original];
    const candidate = {
      id: "candidate-1",
      volumeTitle: "",
      title: "导入第一章",
      wordCount: 7,
      bodyPreview: "雾从海面升起。",
      position: 1,
      sourceStartLine: 1,
      sourceEndLine: 2,
      contentHashSummary: "abcdef012345",
      selected: false,
      createdChapterId: "",
    };
    const job = {
      id: "import-1",
      status: "prepared" as const,
      sourceName: "旧稿.txt",
      sourceFormat: "txt" as const,
      sourceEncoding: "UTF-8",
      sourceHashDigest: "0123456789ab",
      sourceBytes: 128,
      chapterCount: 1,
      selectedChapterCount: 0,
      extractionStatus: "none",
      extractionRunId: "",
      errorCode: "",
      errorMessage: "",
      createdAt: "2026-08-03T08:00:00Z",
      updatedAt: "2026-08-03T08:00:00Z",
      appliedAt: "",
      chapters: [candidate],
    };
    const created = {
      ...client.createChapterResult,
      id: "chapter-imported",
      title: candidate.title,
      position: 2,
      body: "雾从海面升起。",
    };
    client.importJobsResult = [job];
    client.importApplyResult = {
      job: {
        ...job,
        status: "applied",
        selectedChapterCount: 1,
        chapters: [
          {
            ...candidate,
            selected: true,
            createdChapterId: created.id,
          },
        ],
      },
      chapters: [original, created],
    };
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(
      await screen.findByRole("button", { name: /第一章/ }),
    );
    expect(
      await screen.findByRole("heading", { name: original.title }),
    ).toBeInTheDocument();
    await user.click(
      await screen.findByRole("button", { name: "导入与续写" }),
    );
    await user.click(
      await screen.findByRole("checkbox", {
        name: `选择 ${candidate.title}`,
      }),
    );
    await user.click(screen.getByRole("button", { name: "导入所选章节" }));
    await user.click(
      within(
        screen.getByRole("dialog", { name: "确认导入章节" }),
      ).getByRole("button", { name: "确认导入" }),
    );
    await user.click(screen.getByRole("button", { name: "正文与章节" }));

    const chapterList = screen.getByRole("navigation", {
      name: "章节列表",
    });
    expect(within(chapterList).getByText(original.title)).toBeInTheDocument();
    expect(within(chapterList).getByText(candidate.title)).toBeInTheDocument();
    expect(
      within(chapterList).getByText(original.title).closest("button"),
    ).toHaveClass("is-active");
    expect(
      await screen.findByRole("heading", { name: original.title }),
    ).toBeInTheDocument();
  });

  it("opens the characters and factions workbench from the product navigation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 人物与势力 在 作品档案 折叠分组下
    await user.click(await screen.findByRole("button", { name: "作品档案" }));
    await user.click(await screen.findByRole("button", { name: "人物与势力" }));

    expect(
      await screen.findByRole("heading", { name: "人物与势力" }),
    ).toBeInTheDocument();
  });

  it("opens world setting assets from the product navigation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 世界设定 在 作品档案 折叠分组下
    await user.click(await screen.findByRole("button", { name: "作品档案" }));
    await user.click(await screen.findByRole("button", { name: "世界设定" }));

    expect(
      await screen.findByRole("heading", { name: "世界设定" }),
    ).toBeInTheDocument();
  });

  it("opens timeline and canon controls from the product navigation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 时间线与事实 在 作品档案 折叠分组下
    await user.click(await screen.findByRole("button", { name: "作品档案" }));
    await user.click(
      await screen.findByRole("button", { name: "时间线与事实" }),
    );

    expect(
      await screen.findByRole("heading", { name: "时间线与事实" }),
    ).toBeInTheDocument();
  });

  it("opens chapter history and project snapshots from the product navigation", async () => {
    const client = FakeStudioClient.withReadyProject();
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 版本历史 在 版本与交付 折叠分组下
    await user.click(await screen.findByRole("button", { name: "版本与交付" }));
    await user.click(await screen.findByRole("button", { name: "版本历史" }));

    expect(
      await screen.findByRole("heading", { name: "版本历史" }),
    ).toBeInTheDocument();
    expect(screen.getByText("项目快照")).toBeInTheDocument();
  });

  it("opens an existing project selected by the native client", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.projectToOpen = "/books/长夜织境.zjproject";
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(await screen.findByRole("button", { name: /打开项目/ }));

    expect(
      await screen.findByRole("heading", { name: "长夜织境" }),
    ).toBeInTheDocument();
    expect(client.openProjectCalls).toEqual(["/books/长夜织境.zjproject"]);
  });

  it("keeps the new chapter dialog open when creation fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.createChapterError = new Error("无法写入章节");
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(await screen.findByRole("button", { name: "新建章节" }));
    const dialog = screen.getByRole("dialog", { name: "新建章节" });
    await user.type(within(dialog).getByLabelText("章节标题"), "第一章");
    await user.click(within(dialog).getByRole("button", { name: "确认新建" }));

    expect(dialog).toBeInTheDocument();
    expect(within(dialog).getByRole("alert")).toHaveTextContent("无法写入章节");
  });

  it("keeps form input when project creation fails", async () => {
    const client = new FakeStudioClient();
    client.projectParent = "/books";
    client.createProjectError = new Error("磁盘空间不足");
    render(<App client={client} />);

    fireEvent.click(await screen.findByRole("button", { name: /从一个想法开始/ }));
    fireEvent.change(await screen.findByLabelText("书名"), {
      target: { value: "未完成的书" },
    });
    fireEvent.click(screen.getByRole("button", { name: "确认创建" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("磁盘空间不足");
    expect(screen.getByLabelText("书名")).toHaveValue("未完成的书");
  });

  it("awaits the active chapter draft before completing native close", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    render(<App client={client} initialProject={client.openProjectResult} />);

    fireEvent.click(await screen.findByRole("button", { name: /第一章/ }));
    const editor = await screen.findByRole("textbox", { name: "章节正文" });
    fireEvent.change(editor, { target: { value: "关闭窗口前的未提交文本" } });

    await act(async () => client.requestNativeClose());

    expect(client.saveDraftCalls).toContainEqual({
      chapterId: "chapter-1",
      body: "关闭窗口前的未提交文本",
      baseRevision: 1,
    });
    expect(client.completeCloseCalls).toBe(1);
  });

  it("keeps the native window open when the final draft flush fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    client.saveDraftError = new Error("磁盘写入失败");
    render(<App client={client} initialProject={client.openProjectResult} />);

    fireEvent.click(await screen.findByRole("button", { name: /第一章/ }));
    const editor = await screen.findByRole("textbox", { name: "章节正文" });
    fireEvent.change(editor, { target: { value: "仍需保留在窗口中的文本" } });

    let closeError: unknown;
    await act(async () => {
      try {
        await client.requestNativeClose();
      } catch (error) {
        closeError = error;
      }
    });

    expect(closeError).toEqual(new Error("磁盘写入失败"));
    expect(client.completeCloseCalls).toBe(0);
    expect(screen.getByText("保存失败 · 本地文本已保留")).toBeInTheDocument();
  });

  it("waits for a previous chapter flush when closing immediately after switching", async () => {
    const client = FakeStudioClient.withReadyProject();
    const secondChapter = {
      ...client.createChapterResult,
      id: "chapter-2",
      title: "第二章",
      position: 2,
    };
    client.chaptersResult = [client.createChapterResult, secondChapter];
    let finishPreviousFlush: (() => void) | undefined;
    client.saveDraft = vi.fn(async (chapterId, body, baseRevision) => {
      client.saveDraftCalls.push({ chapterId, body, baseRevision });
      await new Promise<void>((resolve) => {
        finishPreviousFlush = resolve;
      });
    });
    render(<App client={client} initialProject={client.openProjectResult} />);

    fireEvent.click(await screen.findByRole("button", { name: /第一章/ }));
    fireEvent.change(await screen.findByRole("textbox", { name: "章节正文" }), {
      target: { value: "切章前尚未落盘的文本" },
    });
    fireEvent.click(screen.getByRole("button", { name: /第二章/ }));
    await screen.findByRole("heading", { name: "第二章" });

    const close = client.requestNativeClose();
    await Promise.resolve();
    expect(client.completeCloseCalls).toBe(0);

    finishPreviousFlush?.();
    await close;
    expect(client.completeCloseCalls).toBe(1);
  });
});
