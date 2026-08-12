import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ApplicationBootstrapView, RecentProjectView } from "../../lib/studio-client";
import { HomeScreen } from "./HomeScreen";

const emptyBootstrap: ApplicationBootstrapView = {
  preferences: { theme: "system", defaultCreativeProfileId: "" },
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
  it("shows welcome actions without a redundant brand dropdown when recent list is empty", () => {
    const onBeginCreate = vi.fn();
    render(
      <HomeScreen
        bootstrap={emptyBootstrap}
        busy={false}
        onBeginCreate={onBeginCreate}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "织境 Story Studio" }),
    ).toBeInTheDocument();
    // The product title is a heading, not a button — there is no brand dropdown.
    expect(
      screen.queryByRole("button", { name: "织境 Story Studio" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /从一个想法开始/ }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /导入小说并续写/ }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: /打开项目/ }),
    ).toBeEnabled();
    // Welcome guide replaces the recent project list.
    expect(screen.getByText(/暂无最近项目/)).toBeInTheDocument();
  });

  it("lists recent projects with their last opened time", () => {
    render(
      <HomeScreen
        bootstrap={bootstrapWithRecent}
        busy={false}
        onBeginCreate={vi.fn()}
        onOpenProject={vi.fn()}
        onImportManuscript={vi.fn()}
        onOpenRecent={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "打开 长夜织境" }),
    ).toBeInTheDocument();
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

  it("shows path unavailable for missing recent projects", () => {
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
      />,
    );

    expect(screen.getByText("路径不可用")).toBeInTheDocument();
  });
});
