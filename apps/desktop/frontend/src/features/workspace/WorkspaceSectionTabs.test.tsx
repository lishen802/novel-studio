import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import {
  SECTION_NAVIGATION_FOCUS_COLOR,
  WorkspaceSectionTabs,
} from "./WorkspaceSectionTabs";

describe("WorkspaceSectionTabs", () => {
  it("uses a dark focus ring with sufficient contrast on the light canvas", () => {
    render(
      <WorkspaceSectionTabs
        section="archive"
        currentPage="story"
        onSelectPage={() => undefined}
      />,
    );

    expect(SECTION_NAVIGATION_FOCUS_COLOR).toBe("#204c7a");
    expect(screen.getByRole("navigation", { name: "作品档案" })).toHaveStyle(
      "--workspace-section-focus: #204c7a",
    );
  });

  it("renders archive pages as content-area navigation and reports the selected page", () => {
    const onSelectPage = vi.fn();
    render(
      <WorkspaceSectionTabs
        section="archive"
        currentPage="story"
        onSelectPage={onSelectPage}
      />,
    );

    expect(screen.getByRole("navigation", { name: "作品档案" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "故事架构" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("button", { name: "故事提案" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "伏笔与回收" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "人物与势力" }));
    expect(onSelectPage).toHaveBeenCalledWith("characters");
  });

  it("renders delivery pages as content-area navigation and reports the selected page", () => {
    const onSelectPage = vi.fn();
    render(
      <WorkspaceSectionTabs
        section="delivery"
        currentPage="versions"
        onSelectPage={onSelectPage}
      />,
    );

    expect(screen.getByRole("navigation", { name: "版本与交付" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "版本历史" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("button", { name: "项目维护" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "分支管理" }));
    expect(onSelectPage).toHaveBeenCalledWith("branches");
  });
});
