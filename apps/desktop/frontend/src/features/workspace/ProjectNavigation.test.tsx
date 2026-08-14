import { fireEvent, render, screen } from "@testing-library/react";
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

    // 二级页面改由内容区标签导航承载，不应出现在侧栏。
    expect(screen.queryByRole("button", { name: /故事架构/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /人物与势力/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /版本历史/ })).not.toBeInTheDocument();
  });

  it("opens the story architecture directly when the archive entry is clicked", () => {
    const onSelect = vi.fn();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={onSelect}
      />,
    );

    const archive = screen.getByRole("button", { name: /作品档案/ });
    expect(archive).not.toHaveAttribute("aria-expanded");
    fireEvent.click(archive);
    expect(onSelect).toHaveBeenCalledWith("story");
  });

  it("opens version history directly when the delivery entry is clicked", () => {
    const onSelect = vi.fn();
    render(
      <ProjectNavigation
        currentPage="manuscript"
        onSelectPage={onSelect}
      />,
    );

    const delivery = screen.getByRole("button", { name: /版本与交付/ });
    expect(delivery).not.toHaveAttribute("aria-expanded");
    fireEvent.click(delivery);
    expect(onSelect).toHaveBeenCalledWith("versions");
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

    fireEvent.click(screen.getByRole("button", { name: /作品档案/ }));
    expect(onSelect).toHaveBeenCalledWith("story");

    fireEvent.click(screen.getByRole("button", { name: /版本与交付/ }));
    expect(onSelect).toHaveBeenCalledWith("versions");
  });

  it("marks the archive entry active for each archive content page", () => {
    render(
      <ProjectNavigation
        currentPage="characters"
        onSelectPage={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: /作品档案/ })).toHaveClass("is-active");
  });

  it("marks the delivery entry active for each delivery content page", () => {
    render(
      <ProjectNavigation
        currentPage="branches"
        onSelectPage={() => undefined}
      />,
    );

    expect(screen.getByRole("button", { name: /版本与交付/ })).toHaveClass("is-active");
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
