import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { FakeStudioClient } from "../lib/fake-studio-client";
import type {
  CreateImportedProjectResult,
  RecentProjectView,
} from "../lib/studio-client";

describe("App", () => {
  it("shows the product identity and project entry actions once bootstrap loads", async () => {
    const client = new FakeStudioClient();
    render(<App client={client} />);

    await waitFor(
      () => {
        const heading = screen.queryByRole("heading", { name: "织境 Story Studio" });
        expect(heading).not.toBeNull();
      },
      { timeout: 2000 },
    );

    expect(
      await screen.findByRole("button", { name: /从一个想法开始/ }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /打开项目/ }),
    ).toBeEnabled();
  });

  it("shows welcome actions without a redundant brand dropdown when recent list is empty", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapResult.recentProjects = [];
    render(<App client={client} />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "织境 Story Studio" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getByRole("button", { name: /从一个想法开始/ }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /导入小说并续写/ }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /打开项目/ }),
    ).toBeEnabled();
    // 产品标题是标题，不是按钮——没有品牌下拉
    expect(
      screen.queryByRole("button", { name: "织境" }),
    ).not.toBeInTheDocument();
  });

  it("offers a retry when the initial application bootstrap fails", async () => {
    const client = new FakeStudioClient();
    client.applicationBootstrapError = new Error("无法读取配置");
    const user = userEvent.setup();
    render(<App client={client} />);

    expect(await screen.findByRole("alertdialog", { name: "应用错误" })).toBeVisible();
    client.applicationBootstrapError = null;
    await user.click(screen.getByRole("button", { name: "重试" }));

    expect(
      await screen.findByRole("button", { name: /从一个想法开始/ }),
    ).toBeEnabled();
  });

  it("opens a recent project from its card", async () => {
    const client = new FakeStudioClient();
    const recent: RecentProjectView = {
      projectId: "recent-1",
      title: "长夜织境",
      path: "/books/长夜织境.zjproject",
      lastWorkspace: "manuscript",
      lastOpenedAt: "2026-08-10T08:00:00Z",
      missing: false,
    };
    client.applicationBootstrapResult.recentProjects = [recent];
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(
      await screen.findByRole("button", { name: "打开 长夜织境" }),
    );

    expect(client.openRecentProjectCalls).toEqual(["recent-1"]);
  });

  it("keeps project mounted while application settings is open", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    // 选中章节并输入正文
    await user.click(await screen.findByRole("button", { name: /第一章/ }));
    const editor = await screen.findByRole("textbox", { name: "章节正文" });
    await user.type(editor, "未保存内容");

    // 通过菜单命令打开设置
    act(() => {
      client.emitMenuCommand({ type: "app_settings" });
    });

    // 关闭设置
    await user.click(
      await screen.findByRole("button", { name: "关闭设置" }),
    );

    // 验证正文内容仍然保留
    expect(screen.getByRole("textbox", { name: "章节正文" })).toHaveValue(
      "未保存内容",
    );
  });

  it("clears API key field after save and never renders the secret", async () => {
    const client = new FakeStudioClient();
    const user = userEvent.setup();
    render(<App client={client} />);

    // 等待首页加载
    await screen.findByRole("button", { name: /从一个想法开始/ });

    // 通过菜单命令打开设置
    act(() => {
      client.emitMenuCommand({ type: "app_settings" });
    });

    // 填写模型连接表单
    await user.type(screen.getByLabelText("名称"), "OpenAI 主线");
    await user.type(screen.getByLabelText("API Key"), "sk-test-123456");
    await user.click(screen.getByRole("button", { name: "保存模型连接" }));

    // 验证 API Key 字段已清空且密钥不会被回显
    await waitFor(() =>
      expect(screen.getByLabelText("API Key")).toHaveValue(""),
    );
    expect(screen.queryByText("sk-test-123456")).not.toBeInTheDocument();
  });

  it("opens imported manuscript immediately and shows whole-book archive action", async () => {
    const client = new FakeStudioClient();
    client.projectParent = "/books";
    client.manuscriptToImport = "/Users/author/长夜.md";
    const imported: CreateImportedProjectResult = {
      project: {
        ...client.createProjectResult,
        title: "长夜织境",
        genre: "东方奇幻",
      },
      chapters: [
        {
          ...client.createChapterResult,
          title: "第一章 潮声",
          body: "潮声越过防波堤。",
        },
      ],
      workbench: {
        stage: "writing",
        canvas: "manuscript",
        aiPanel: "hidden",
        pendingReviewCount: 0,
        recoverableJobCount: 0,
        archiveStatus: "not_generated",
      },
    };
    client.createImportedProjectResult = imported;
    const user = userEvent.setup();
    render(<App client={client} />);

    await screen.findByRole("button", { name: /导入小说并续写/ });
    await user.click(screen.getByRole("button", { name: /导入小说并续写/ }));

    // 向导：选择源文件 → 填写标题与版权确认 → 确认导入
    await screen.findByRole("heading", { name: "导入小说并续写" });
    await user.click(screen.getByRole("button", { name: "选择小说文件" }));
    await user.type(
      screen.getByLabelText("书名"),
      "长夜织境",
    );
    await user.click(screen.getByLabelText("我拥有这篇小说的版权"));
    await user.click(screen.getByRole("button", { name: "确认导入" }));

    expect(await screen.findByRole("heading", { name: "正文与章节" })).toBeVisible();
    // 正文画布顶部展示整书档案生成入口；R2 不允许伪造完成。
    expect(
      screen.getByRole("button", { name: "AI 生成整书档案" }),
    ).toBeVisible();
    expect(client.createImportedProjectCalls).toHaveLength(1);
    expect(client.createImportedProjectCalls[0]).toMatchObject({
      sourcePath: "/Users/author/长夜.md",
      parentDirectory: "/books",
      title: "长夜织境",
      copyrightConfirmed: true,
    });
  });

  it("surfaces a structured error when import manuscript fails without faking completion", async () => {
    const client = new FakeStudioClient();
    client.projectParent = "/books";
    client.manuscriptToImport = "/Users/author/长夜.md";
    client.createImportedProjectError = new (class extends Error {
      code = "IMPORT_SOURCE_UNSAFE";
      constructor() {
        super("所选来源不是安全的普通文件。");
        this.name = "StudioClientError";
      }
    })();
    const user = userEvent.setup();
    render(<App client={client} />);

    await screen.findByRole("button", { name: /导入小说并续写/ });
    await user.click(screen.getByRole("button", { name: /导入小说并续写/ }));
    await screen.findByRole("heading", { name: "导入小说并续写" });
    await user.click(screen.getByRole("button", { name: "选择小说文件" }));
    await user.type(screen.getByLabelText("书名"), "长夜织境");
    await user.click(screen.getByLabelText("我拥有这篇小说的版权"));
    await user.click(screen.getByRole("button", { name: "确认导入" }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "正文与章节" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "AI 生成整书档案" }),
    ).not.toBeInTheDocument();
  });

  it("keeps the AI panel hidden during normal writing until Cmd+J", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    client.workbenchProjectionResult = {
      stage: "writing",
      canvas: "manuscript",
      aiPanel: "hidden",
      pendingReviewCount: 0,
      recoverableJobCount: 0,
      archiveStatus: "not_generated",
    };
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await screen.findByRole("button", { name: /第一章/ });
    expect(
      screen.queryByRole("complementary", { name: "AI 创作" }),
    ).not.toBeInTheDocument();

    await user.keyboard("{Meta>}j{/Meta}");

    expect(
      await screen.findByRole("complementary", { name: "AI 创作" }),
    ).toBeVisible();
  });

  it("preserves unsaved chapter and AI input while toggling panel and settings", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.chaptersResult = [client.createChapterResult];
    client.workbenchProjectionResult = {
      stage: "writing",
      canvas: "manuscript",
      aiPanel: "hidden",
      pendingReviewCount: 0,
      recoverableJobCount: 0,
      archiveStatus: "not_generated",
    };
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(await screen.findByRole("button", { name: /第一章/ }));
    const editor = screen.getByRole("textbox", { name: "章节正文" });
    await user.type(editor, "正文草稿");

    await user.keyboard("{Meta>}j{/Meta}");
    const aiInput = await screen.findByRole("textbox", {
      name: "给 AI 的说明",
    });
    await user.type(aiInput, "保持第三人称");

    // 通过菜单命令打开应用设置，再关闭设置抽屉。
    act(() => {
      client.emitMenuCommand({ type: "app_settings" });
    });
    await user.click(
      await screen.findByRole("button", { name: "关闭设置" }),
    );

    // 验证正文与 AI 输入均不因设置抽屉的开合而丢失。
    // 注：jest-dom v6 的 toHaveValue 不支持 expect.stringContaining，使用 toContain 替代。
    const chapterEditor = screen.getByRole("textbox", {
      name: "章节正文",
    }) as HTMLTextAreaElement;
    expect(chapterEditor.value).toContain("正文草稿");
    expect(
      screen.getByRole("textbox", { name: "给 AI 的说明" }),
    ).toHaveValue("保持第三人称");
  });
});
