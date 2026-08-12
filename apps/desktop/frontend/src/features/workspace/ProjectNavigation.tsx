import {
  BookOpenText,
  Clock3,
  FileDown,
  FileSearch2,
  GitBranch,
  GitCompareArrows,
  Globe2,
  History,
  LayoutDashboard,
  LibraryBig,
  Lightbulb,
  Network,
  Package,
  Sparkles,
  UsersRound,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useState } from "react";
import type { ProjectPage } from "./workspace-store";

// 项目一级导航分组。一级固定，二级页面在可折叠分组下展开；
// stage/canvas/counts 始终从权威 WorkbenchProjection 读取，不在此推断。
export type ProjectSection =
  | "dashboard"
  | "manuscript"
  | "review"
  | "import"
  | "archive"
  | "delivery";

type NavLeaf = {
  page: ProjectPage;
  label: string;
  icon: LucideIcon;
};

type NavSection = {
  id: ProjectSection;
  label: string;
  icon: LucideIcon;
  // 直接跳转的页面（无二级菜单时）；与 leaves 互斥
  directPage?: ProjectPage;
  // 二级页面列表；存在时该分组可折叠
  leaves?: NavLeaf[];
};

const SECTIONS: NavSection[] = [
  {
    id: "dashboard",
    label: "创作驾驶舱",
    icon: LayoutDashboard,
    directPage: "dashboard",
  },
  {
    id: "manuscript",
    label: "正文与章节",
    icon: BookOpenText,
    directPage: "manuscript",
  },
  {
    id: "review",
    label: "审阅与候选",
    icon: GitCompareArrows,
    directPage: "changes",
  },
  {
    id: "import",
    label: "导入与续写",
    icon: FileSearch2,
    directPage: "imports",
  },
  {
    id: "archive",
    label: "作品档案",
    icon: LibraryBig,
    leaves: [
      { page: "proposal", label: "故事提案", icon: Lightbulb },
      { page: "story", label: "故事架构", icon: Network },
      { page: "characters", label: "人物与势力", icon: UsersRound },
      { page: "world", label: "世界设定", icon: Globe2 },
      { page: "continuity", label: "时间线与事实", icon: Clock3 },
      { page: "foreshadowing", label: "伏笔与回收", icon: Sparkles },
    ],
  },
  {
    id: "delivery",
    label: "版本与交付",
    icon: History,
    leaves: [
      { page: "versions", label: "版本历史", icon: History },
      { page: "branches", label: "分支管理", icon: GitBranch },
      { page: "export", label: "导出与发布", icon: Package },
      { page: "maintenance", label: "项目维护", icon: Wrench },
    ],
  },
];

// 将 ProjectPage 映射到所属分组，便于自动展开当前页所在的折叠组
function sectionOfPage(page: ProjectPage): ProjectSection {
  for (const section of SECTIONS) {
    if (section.directPage === page) return section.id;
    if (section.leaves?.some((leaf) => leaf.page === page)) return section.id;
  }
  return "manuscript";
}

export type ProjectNavigationProps = {
  currentPage: ProjectPage;
  onSelectPage(page: ProjectPage): void;
  // 来自权威 WorkbenchProjection 的计数，仅用于徽章展示
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
  const [expanded, setExpanded] = useState<Record<ProjectSection, boolean>>({
    dashboard: false,
    manuscript: false,
    review: false,
    import: false,
    archive: activeSection === "archive",
    delivery: activeSection === "delivery",
  });

  // 切换到属于折叠分组的页面时，自动展开该分组
  useEffect(() => {
    if (activeSection === "archive" || activeSection === "delivery") {
      setExpanded((current) =>
        current[activeSection]
          ? current
          : { ...current, [activeSection]: true },
      );
    }
  }, [activeSection]);

  const toggleSection = (section: ProjectSection) => {
    setExpanded((current) => ({ ...current, [section]: !current[section] }));
  };

  return (
    <nav className="workspace-sections" aria-label="作品功能">
      {SECTIONS.map((section) => {
        const Icon = section.icon;
        const isCollapsible = Boolean(section.leaves);
        const isExpanded = expanded[section.id];
        const isActiveSection = activeSection === section.id;
        const showReviewBadge =
          section.id === "review" && pendingReviewCount > 0;
        const showRecoverableBadge =
          section.id === "dashboard" && recoverableJobCount > 0;

        return (
          <div key={section.id} className="workspace-section-group">
            <button
              type="button"
              className={
                isActiveSection && !isCollapsible ? "is-active" : ""
              }
              aria-expanded={isCollapsible ? isExpanded : undefined}
              onClick={() => {
                if (isCollapsible) {
                  toggleSection(section.id);
                  return;
                }
                if (section.directPage) {
                  onSelectPage(section.directPage);
                }
              }}
            >
              <Icon size={17} />
              <span>{section.label}</span>
              {showReviewBadge && (
                <span className="workspace-section-badge" aria-label="待审阅数量">
                  {pendingReviewCount}
                </span>
              )}
              {showRecoverableBadge && (
                <span
                  className="workspace-section-badge workspace-section-badge--warning"
                  aria-label="可恢复任务数量"
                >
                  {recoverableJobCount}
                </span>
              )}
            </button>
            {isCollapsible && isExpanded && section.leaves && (
              <div className="workspace-section-leaves" role="group" aria-label={section.label}>
                {section.leaves.map((leaf) => {
                  const LeafIcon = leaf.icon;
                  const isActive = currentPage === leaf.page;
                  return (
                    <button
                      type="button"
                      key={leaf.page}
                      className={
                        isActive
                          ? "is-active workspace-section-leaf"
                          : "workspace-section-leaf"
                      }
                      onClick={() => onSelectPage(leaf.page)}
                    >
                      <LeafIcon size={15} />
                      <span>{leaf.label}</span>
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </nav>
  );
}
