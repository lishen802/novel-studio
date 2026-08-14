import type { CSSProperties } from "react";
import type { ProjectPage } from "./workspace-store";

// 深蓝焦点环与白色按钮/浅色主画布均保持足够的可见对比。
export const SECTION_NAVIGATION_FOCUS_COLOR = "#204c7a";

type WorkspaceTab = {
  page: ProjectPage;
  label: string;
};

const TABS: Record<"archive" | "delivery", WorkspaceTab[]> = {
  archive: [
    { page: "proposal", label: "故事提案" },
    { page: "story", label: "故事架构" },
    { page: "characters", label: "人物与势力" },
    { page: "world", label: "世界设定" },
    { page: "continuity", label: "时间线与事实" },
    { page: "foreshadowing", label: "伏笔与回收" },
  ],
  delivery: [
    { page: "versions", label: "版本历史" },
    { page: "branches", label: "分支管理" },
    { page: "export", label: "导出与发布" },
    { page: "maintenance", label: "项目维护" },
  ],
};

export type WorkspaceSectionTabsProps = {
  section: "archive" | "delivery";
  currentPage: ProjectPage;
  onSelectPage(page: ProjectPage): void;
};

// 内容区标签保留所有已实现和诚实未就绪页面，使一级侧栏保持紧凑而能力仍可达。
export function WorkspaceSectionTabs({
  section,
  currentPage,
  onSelectPage,
}: WorkspaceSectionTabsProps) {
  const label = section === "archive" ? "作品档案" : "版本与交付";

  return (
    <nav
      className="workspace-section-tabs"
      aria-label={label}
      style={{ "--workspace-section-focus": SECTION_NAVIGATION_FOCUS_COLOR } as CSSProperties}
    >
      {TABS[section].map((tab) => (
        <button
          type="button"
          key={tab.page}
          aria-current={currentPage === tab.page ? "page" : undefined}
          className={currentPage === tab.page ? "is-active" : ""}
          onClick={() => onSelectPage(tab.page)}
        >
          {tab.label}
        </button>
      ))}
    </nav>
  );
}
