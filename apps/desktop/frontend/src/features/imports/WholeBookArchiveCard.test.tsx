import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { WholeBookArchiveCard } from "./WholeBookArchiveCard";

describe("WholeBookArchiveCard", () => {
  it("renders the not_generated status with a disabled generate button", () => {
    render(<WholeBookArchiveCard status="not_generated" onStart={vi.fn()} />);
    expect(
      screen.getByRole("heading", { name: "整书档案" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("尚未生成整本小说档案。"),
    ).toBeInTheDocument();
    // 按钮可见，但因 R2 未实现整书档案生成能力而禁用。
    expect(
      screen.getByRole("button", { name: "AI 生成整书档案" }),
    ).toBeDisabled();
  });

  it("does not show chapter-level archive conclusions", () => {
    render(<WholeBookArchiveCard status="not_generated" onStart={vi.fn()} />);
    expect(
      screen.getByRole("button", { name: "AI 生成整书档案" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /确认第.*章档案/ }),
    ).not.toBeInTheDocument();
  });

  it("calls onStart when the button is enabled for an active archive status", async () => {
    const onStart = vi.fn();
    const user = userEvent.setup();
    render(<WholeBookArchiveCard status="running" onStart={onStart} />);
    // running 状态下按钮可见且可点击，以表达"档案生成进行中"的可见性。
    // R2 后端不会返回 running，但前端组件应当能正确渲染该状态。
    const button = screen.getByRole("button", { name: "AI 生成整书档案" });
    expect(button).toBeEnabled();
    await user.click(button);
    expect(onStart).toHaveBeenCalledOnce();
  });

  it("renders an unavailable notice rather than fake completion when status is not_generated", () => {
    render(<WholeBookArchiveCard status="not_generated" onStart={vi.fn()} />);
    expect(
      screen.getByText(/将在后续能力阶段开放|尚未生成/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/档案已生成|档案已完成/),
    ).not.toBeInTheDocument();
  });
});
