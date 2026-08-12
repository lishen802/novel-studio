import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { App } from "./App";
import { FakeStudioClient } from "../lib/fake-studio-client";
import type { RecentProjectView } from "../lib/studio-client";

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
});
