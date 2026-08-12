import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ProjectNavigation } from "./ProjectNavigation";

describe("ProjectNavigation", () => {
  it("renders only the six fixed top-level groups by default", () => {
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: /创作驾驶舱/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /正文与章节/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /审阅与候选/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /导入与续写/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /作品档案/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /版本与交付/ })).toBeInTheDocument();

    // 二级页面默认折叠
    expect(screen.queryByRole("button", { name: /故事架构/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /人物与势力/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /版本历史/ })).not.toBeInTheDocument();
  });

  it("groups archive pages below a secondary section that expands on click", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={onSelect}
      />,
    );

    const archive = screen.getByRole("button", { name: /作品档案/ });
    expect(archive).toHaveAttribute("aria-expanded", "false");

    await user.click(archive);
    expect(archive).toHaveAttribute("aria-expanded", "true");
    expect(await screen.findByRole("button", { name: /故事架构/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /人物与势力/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /世界设定/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /时间线与事实/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /故事提案/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /伏笔与回收/ })).toBeVisible();

    // 折叠后二级页面再次隐藏
    await user.click(archive);
    expect(archive).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("button", { name: /故事架构/ })).not.toBeInTheDocument();
  });

  it("groups delivery pages below a secondary section that expands on click", async () => {
    const user = userEvent.setup();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={() => undefined}
      />,
    );

    const delivery = screen.getByRole("button", { name: /版本与交付/ });
    await user.click(delivery);
    expect(await screen.findByRole("button", { name: /版本历史/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /分支管理/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /导出与发布/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /项目维护/ })).toBeVisible();
  });

  it("invokes onSelectPage with the page id when a top-level direct page is clicked", () => {
    const onSelect = vi.fn();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /正文与章节/ }));
    expect(onSelect).toHaveBeenCalledWith("manuscript");

    fireEvent.click(screen.getByRole("button", { name: /审阅与候选/ }));
    expect(onSelect).toHaveBeenCalledWith("changes");

    fireEvent.click(screen.getByRole("button", { name: /导入与续写/ }));
    expect(onSelect).toHaveBeenCalledWith("imports");

    fireEvent.click(screen.getByRole("button", { name: /创作驾驶舱/ }));
    expect(onSelect).toHaveBeenCalledWith("dashboard");
  });

  it("invokes onSelectPage with the page id when a secondary leaf is clicked", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={onSelect}
      />,
    );

    await user.click(screen.getByRole("button", { name: /作品档案/ }));
    await user.click(screen.getByRole("button", { name: /故事架构/ }));
    expect(onSelect).toHaveBeenCalledWith("story");

    await user.click(screen.getByRole("button", { name: /时间线与事实/ }));
    expect(onSelect).toHaveBeenCalledWith("continuity");

    await user.click(screen.getByRole("button", { name: /版本与交付/ }));
    await user.click(screen.getByRole("button", { name: /版本历史/ }));
    expect(onSelect).toHaveBeenCalledWith("versions");
  });

  it("auto-expands the archive group when the current page belongs to it", () => {
    render(
      <ProjectNavigation
        currentPage="characters"
        onSelectPage={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: /人物与势力/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /作品档案/ })).toHaveAttribute("aria-expanded", "true");
  });

  it("auto-expands the delivery group when the current page belongs to it", () => {
    render(
      <ProjectNavigation
        currentPage="branches"
        onSelectPage={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: /分支管理/ })).toBeVisible();
    expect(screen.getByRole("button", { name: /版本与交付/ })).toHaveAttribute("aria-expanded", "true");
  });

  it("renders the pending review count badge from the authoritative projection", () => {
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={() => undefined}
        pendingReviewCount={4}
      />,
    );

    expect(screen.getByRole("button", { name: /审阅与候选/ })).toContainHTML("4");
  });

  it("renders the recoverable job count badge on the dashboard", () => {
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={() => undefined}
        recoverableJobCount={2}
      />,
    );

    expect(screen.getByRole("button", { name: /创作驾驶舱/ })).toContainHTML("2");
  });
});
