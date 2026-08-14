import {
  BookOpenText,
  FileSearch2,
  GitCompareArrows,
  History,
  LayoutDashboard,
  LibraryBig,
  type LucideIcon,
} from "lucide-react";
import type { ProjectPage } from "./workspace-store";

// 项目侧栏仅保留原型中的六个一级入口；档案和交付的细分页面由内容区标签承载。
export type ProjectSection =
  | "dashboard"
  | "manuscript"
  | "review"
  | "import"
  | "archive"
  | "delivery";

type NavSection = {
  id: ProjectSection;
  label: string;
  icon: LucideIcon;
  page: ProjectPage;
};

const SECTIONS: NavSection[] = [
  { id: "dashboard", label: "创作驾驶舱", icon: LayoutDashboard, page: "dashboard" },
  { id: "manuscript", label: "正文与章节", icon: BookOpenText, page: "manuscript" },
  { id: "review", label: "审阅与候选", icon: GitCompareArrows, page: "changes" },
  { id: "import", label: "导入与续写", icon: FileSearch2, page: "imports" },
  { id: "archive", label: "作品档案", icon: LibraryBig, page: "story" },
  { id: "delivery", label: "版本与交付", icon: History, page: "versions" },
];

// 将内容页映射为侧栏一级入口，确保在档案/交付内部切换标签时保持高亮。
function sectionOfPage(page: ProjectPage): ProjectSection {
  if (["proposal", "story", "characters", "world", "continuity", "foreshadowing"].includes(page)) {
    return "archive";
  }
  if (["versions", "branches", "export", "maintenance"].includes(page)) {
    return "delivery";
  }
  if (page === "changes") return "review";
  if (page === "imports") return "import";
  if (page === "dashboard") return "dashboard";
  return "manuscript";
}

export type ProjectNavigationProps = {
  currentPage: ProjectPage;
  onSelectPage(page: ProjectPage): void;
  // 来自权威 WorkbenchProjection 的计数，仅用于徽章展示。
  pendingReviewCount?: number;
  recoverableJobCount?: number;
};

export function ProjectNavigation({
  currentPage,
  onSelectPage,
  pendingReviewCount = 0,
  recoverableJobCount = 0,
}: ProjectNavigationProps) {
  const activeSection = sectionOfPage(currentPage);

  return (
    <nav className="workspace-sections" aria-label="作品功能">
      {SECTIONS.map((section) => {
        const Icon = section.icon;
        const showReviewBadge = section.id === "review" && pendingReviewCount > 0;
        const showRecoverableBadge = section.id === "dashboard" && recoverableJobCount > 0;

        return (
          <button
            type="button"
            key={section.id}
            className={activeSection === section.id ? "is-active" : ""}
            onClick={() => onSelectPage(section.page)}
          >
            <Icon size={17} />
            <span>{section.label}</span>
            {showReviewBadge && (
              <span className="workspace-section-badge" aria-label="待审阅数量">
                {pendingReviewCount}
              </span>
            )}
            {showRecoverableBadge && (
              <span className="workspace-section-badge workspace-section-badge--warning" aria-label="可恢复任务数量">
                {recoverableJobCount}
              </span>
            )}
          </button>
        );
      })}
    </nav>
  );
}
