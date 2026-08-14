import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AIPanel } from "./AIPanel";
import type { AIPanelSelection } from "../../lib/studio-client";

describe("AIPanel", () => {
  it("renders nothing when mode is hidden", () => {
    const onInput = vi.fn();
    const onSelectionReview = vi.fn();
    const { container } = render(
      <AIPanel
        projectId="project-1"
        mode="hidden"
        input=""
        onInput={onInput}
        selection={null}
        onSelectionReview={onSelectionReview}
        onDismissSelection={vi.fn()}
        onSubmitSelectionReview={vi.fn()}
      />,
    );
    expect(container.firstChild).toBeNull();
    expect(
      screen.queryByRole("complementary", { name: "AI 创作" }),
    ).not.toBeInTheDocument();
  });

  it("shows a textarea with the current draft in expanded mode", async () => {
    const onInput = vi.fn();
    const user = userEvent.setup();
    render(
      <AIPanel
        projectId="project-1"
        mode="expanded"
        input="保持第三人称"
        onInput={onInput}
        selection={null}
        onSelectionReview={vi.fn()}
        onDismissSelection={vi.fn()}
        onSubmitSelectionReview={vi.fn()}
      />,
    );
    const textarea = screen.getByRole("textbox", { name: "给 AI 的说明" });
    expect(textarea).toHaveValue("保持第三人称");
    expect(screen.getByRole("heading", { name: "AI 创作" })).toBeInTheDocument();
    expect(
      screen.getByText("AI 创作能力尚未就绪，不会生成候选或改写正文。"),
    ).toBeInTheDocument();
    await user.type(textarea, "。");
    expect(onInput).toHaveBeenLastCalledWith("保持第三人称。");
  });

  it("renders collapsed mode as a complementary region that can be expanded", async () => {
    const onInput = vi.fn();
    const user = userEvent.setup();
    render(
      <AIPanel
        projectId="project-1"
        mode="collapsed"
        input=""
        onInput={onInput}
        selection={null}
        onSelectionReview={vi.fn()}
        onDismissSelection={vi.fn()}
        onSubmitSelectionReview={vi.fn()}
      />,
    );
    const region = screen.getByRole("complementary", { name: "AI 创作" });
    expect(region).toBeInTheDocument();
    // 折叠模式下不直接显示输入区，点击展开按钮后才出现。
    expect(
      screen.queryByRole("textbox", { name: "给 AI 的说明" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "展开 AI 面板" }));
    expect(
      screen.getByRole("textbox", { name: "给 AI 的说明" }),
    ).toBeInTheDocument();
  });

  it("shows selection metadata in selection_review mode", () => {
    const selection: AIPanelSelection = {
      chapterId: "chapter-1",
      baseRevision: 7,
      start: 0,
      end: 2,
      text: "潮声",
    };
    render(
      <AIPanel
        projectId="project-1"
        mode="selection_review"
        input=""
        onInput={vi.fn()}
        selection={selection}
        onSelectionReview={vi.fn()}
        onDismissSelection={vi.fn()}
        onSubmitSelectionReview={vi.fn()}
      />,
    );
    expect(screen.getByText("Revision 7 · 字符 0–2")).toBeInTheDocument();
    expect(screen.getByText("潮声")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "取消选区意见" }),
    ).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "关闭 AI 面板" }),
    ).not.toBeInTheDocument();
  });

  it("keeps selection review unsubmitted until the capability is ready", async () => {
    const onSubmitSelectionReview = vi.fn();
    const onDismissSelection = vi.fn();
    const user = userEvent.setup();
    const selection: AIPanelSelection = {
      chapterId: "chapter-1",
      baseRevision: 7,
      start: 0,
      end: 2,
      text: "潮声",
    };
    render(
      <AIPanel
        projectId="project-1"
        mode="selection_review"
        input="重写这一段"
        onInput={vi.fn()}
        selection={selection}
        onSelectionReview={vi.fn()}
        onDismissSelection={onDismissSelection}
        onSubmitSelectionReview={onSubmitSelectionReview}
      />,
    );
    expect(
      screen.getByText("选区意见提交将在后续阶段提供；当前不会生成或提交 AI 请求。"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "提交选区意见" }),
    ).not.toBeInTheDocument();
    expect(onSubmitSelectionReview).not.toHaveBeenCalled();
  });

  it("allows the author to dismiss the selection review", async () => {
    const onDismissSelection = vi.fn();
    const user = userEvent.setup();
    const selection: AIPanelSelection = {
      chapterId: "chapter-1",
      baseRevision: 7,
      start: 0,
      end: 2,
      text: "潮声",
    };
    render(
      <AIPanel
        projectId="project-1"
        mode="selection_review"
        input=""
        onInput={vi.fn()}
        selection={selection}
        onSelectionReview={vi.fn()}
        onDismissSelection={onDismissSelection}
        onSubmitSelectionReview={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: "取消选区意见" }));
    expect(onDismissSelection).toHaveBeenCalledOnce();
  });
});
