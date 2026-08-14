import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ApplicationBootstrapView, RecentProjectView } from "../../lib/studio-client";
import { HomeScreen } from "./HomeScreen";

const emptyBootstrap: ApplicationBootstrapView = {
  preferences: {
    theme: "system",
    defaultCreativeProfileId: "",
    updatedAt: "2026-08-12T10:00:00Z",
  },
  recentProjects: [],
  providers: [],
  profiles: [],
};

const recentProject: RecentProjectView = {
  projectId: "recent-1",
  title: "长夜织境",
  path: "/books/长夜织境.zjproject",
  lastWorkspace: "manuscript",
  lastOpenedAt: "2026-08-10T08:00:00Z",
  missing: false,
};

const bootstrapWithRecent: ApplicationBootstrapView = {
  ...emptyBootstrap,
  recentProjects: [recentProject],
};

describe("HomeScreen", () => {
  it("shows the approved two-column welcome shell without a redundant brand block", () => {
    const onBeginCreate = vi.fn();
    render(
      <HomeScreen
        bootstrap={emptyBootstrap}
        busy={false}
        onBeginCreate={onBeginCreate}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
        onReviewRecent={vi.fn()}
      />,
    );

    const navigation = screen.getByRole("complementary", { name: "首页导航" });
    const content = screen.getByRole("region", { name: "首页内容" });
    expect(within(content).getByRole("heading", { name: "开始一本新作品" })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "织境 Story Studio" }),
    ).not.toBeInTheDocument();
    expect(
      within(navigation).getByRole("button", { name: /从一个想法开始/ }),
    ).toBeEnabled();
    expect(
      within(navigation).getByRole("button", { name: /导入小说并续写/ }),
    ).toBeEnabled();
    expect(
      within(navigation).getByRole("button", { name: /打开项目/ }),
    ).toBeEnabled();
    expect(within(content).getByText(/还没有最近作品/)).toBeInTheDocument();
  });

  it("renders the recent-project cover, workspace and time in the prototype card layout", () => {
    render(
      <HomeScreen
        bootstrap={bootstrapWithRecent}
        busy={false}
        onBeginCreate={vi.fn()}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
        onReviewRecent={vi.fn()}
      />,
    );

    const navigation = screen.getByRole("complementary", { name: "首页导航" });
    const content = screen.getByRole("region", { name: "首页内容" });
    expect(within(navigation).getByRole("button", { name: "打开 长夜织境" })).toBeVisible();
    const cover = navigation.querySelector(".recent-project-card__cover");
    expect(cover).toHaveAttribute("aria-hidden", "true");
    expect(within(navigation).queryByRole("img")).not.toBeInTheDocument();
    expect(within(navigation).getByText("最近工作区：正文编辑")).toBeVisible();
    expect(within(content).getByRole("heading", { name: "长夜织境" })).toBeVisible();
    expect(within(content).getByText("最近继续")).toBeVisible();
    expect(within(content).getByText("上次位置：正文编辑")).toBeVisible();
    expect(within(content).getByText("上次打开：2026-08-10T08:00:00Z")).toBeVisible();
    expect(within(content).getByRole("button", { name: "继续《长夜织境》" })).toBeVisible();
    expect(
      screen.getByText(/上次打开于 2026-08-10T08:00:00Z/),
    ).toBeInTheDocument();
  });

  it("opens a recent project from its card", async () => {
    const onOpenRecent = vi.fn();
    render(
      <HomeScreen
        bootstrap={bootstrapWithRecent}
        busy={false}
        onBeginCreate={vi.fn()}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={onOpenRecent}
        onReviewRecent={vi.fn()}
      />,
    );

    await userEvent.click(
      screen.getByRole("button", { name: "打开 长夜织境" }),
    );

    expect(onOpenRecent).toHaveBeenCalledWith(recentProject);
  });

  it("disables entry actions while busy", () => {
    render(
      <HomeScreen
        bootstrap={bootstrapWithRecent}
        busy={true}
        onBeginCreate={vi.fn()}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
        onReviewRecent={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: /从一个想法开始/ }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /导入小说并续写/ }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /打开项目/ }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "打开 长夜织境" }),
    ).toBeDisabled();
  });

  it("shows path unavailable and disables continuing a missing recent project", () => {
    const missingBootstrap: ApplicationBootstrapView = {
      ...emptyBootstrap,
      recentProjects: [{ ...recentProject, missing: true }],
    };
    render(
      <HomeScreen
        bootstrap={missingBootstrap}
        busy={false}
        onBeginCreate={vi.fn()}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
        onReviewRecent={vi.fn()}
      />,
    );

    expect(screen.getByText("路径不可用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "继续《长夜织境》" })).toBeDisabled();
  });
});
