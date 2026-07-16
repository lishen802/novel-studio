import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { App } from "../../app/App";
import { FakeStudioClient } from "../../lib/fake-studio-client";

describe("Workspace", () => {
  it("creates a project once and enters an empty workspace", async () => {
    const client = new FakeStudioClient();
    client.projectParent = "/books";
    client.createProjectResult = {
      id: "project-1",
      title: "长夜织境",
      genre: "东方奇幻",
      summary: "失落城邦与梦境交叠。",
      path: "/books/长夜织境.zjproject",
    };
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(screen.getByRole("button", { name: "创建项目" }));
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

    await user.click(screen.getByRole("button", { name: "创建项目" }));

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

    await user.click(
      await screen.findByRole("button", { name: "新建章节" }),
    );
    await user.type(screen.getByLabelText("章节标题"), "第一章");
    await user.click(screen.getByRole("button", { name: "确认新建" }));

    const editor = await screen.findByRole("textbox", { name: "章节正文" });
    await waitFor(() => expect(editor).toHaveFocus());
    expect(client.createChapterCalls).toEqual(["第一章"]);
  });

  it("opens an existing project selected by the native client", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.projectToOpen = "/books/长夜织境.zjproject";
    const user = userEvent.setup();
    render(<App client={client} />);

    await user.click(screen.getByRole("button", { name: "打开项目" }));

    expect(
      await screen.findByRole("heading", { name: "长夜织境" }),
    ).toBeInTheDocument();
    expect(client.openProjectCalls).toEqual([
      "/books/长夜织境.zjproject",
    ]);
  });

  it("keeps the new chapter dialog open when creation fails", async () => {
    const client = FakeStudioClient.withReadyProject();
    client.createChapterError = new Error("无法写入章节");
    const user = userEvent.setup();
    render(<App client={client} initialProject={client.openProjectResult} />);

    await user.click(
      await screen.findByRole("button", { name: "新建章节" }),
    );
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

    fireEvent.click(screen.getByRole("button", { name: "创建项目" }));
    fireEvent.change(await screen.findByLabelText("书名"), {
      target: { value: "未完成的书" },
    });
    fireEvent.click(screen.getByRole("button", { name: "确认创建" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("磁盘空间不足");
    expect(screen.getByLabelText("书名")).toHaveValue("未完成的书");
  });
});
