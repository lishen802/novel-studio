import { BookOpenText, FolderOpen, Lightbulb, Upload } from "lucide-react";
import type {
  ApplicationBootstrapView,
  RecentProjectView,
} from "../../lib/studio-client";

type HomeScreenProps = {
  bootstrap: ApplicationBootstrapView;
  busy: boolean;
  onBeginCreate(): void;
  onOpenProject(): void;
  onImportManuscript(): void;
  onOpenRecent(project: RecentProjectView): void;
  onReviewRecent(project: RecentProjectView): void;
};

export function HomeScreen({
  bootstrap,
  busy,
  onBeginCreate,
  onOpenProject,
  onImportManuscript,
  onOpenRecent,
  onReviewRecent,
}: HomeScreenProps) {
  const recent = bootstrap.recentProjects;
  const hasRecent = recent.length > 0;
  const featured = recent[0];

  // 将后端工作区标识转换为首页可读的最近位置。
  const workspaceLabel = (workspace: string) => {
    if (workspace === "manuscript") return "正文编辑";
    if (workspace === "review") return "作者审阅";
    if (workspace === "proposal") return "作品方案";
    return "作品工作台";
  };

  return (
    <div className="home-shell">
      <aside className="home-sidebar" aria-label="首页导航">
        <nav className="home-entry-actions" aria-label="创作入口">
          <button type="button" disabled={busy} onClick={onBeginCreate}>
            <Lightbulb size={19} aria-hidden="true" />
            <span>从一个想法开始</span>
          </button>
          <button type="button" disabled={busy} onClick={onImportManuscript}>
            <Upload size={19} aria-hidden="true" />
            <span>导入小说并续写</span>
          </button>
          <button type="button" disabled={busy} onClick={onOpenProject}>
            <FolderOpen size={19} aria-hidden="true" />
            <span>打开项目</span>
          </button>
        </nav>

        <div className="home-recent">
          <p className="home-sidebar__label">最近项目</p>
          {hasRecent ? (
            <ul className="recent-project-list" aria-label="最近打开的项目">
              {recent.map((project) => (
                <li key={project.projectId}>
                  <button
                    type="button"
                    className="recent-project-card"
                    disabled={busy}
                    aria-label={`打开 ${project.title}`}
                    onClick={() => onOpenRecent(project)}
                  >
                    <span
                      className="recent-project-card__cover"
                      aria-hidden="true"
                    />
                    <span className="recent-project-card__details">
                      <span className="recent-project-card__title">{project.title}</span>
                      <span className="recent-project-card__workspace">
                        最近工作区：{workspaceLabel(project.lastWorkspace)}
                      </span>
                      <span className="recent-project-card__meta">
                        {project.missing
                          ? "路径不可用"
                          : project.lastOpenedAt
                            ? `上次打开于 ${project.lastOpenedAt}`
                            : "尚未打开"}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <p className="home-sidebar__empty">暂无最近项目</p>
          )}
        </div>
      </aside>

      <section className="home-content" aria-label="首页内容">
        {featured ? (
          <div className="home-featured-project">
            <div className="home-featured-project__card">
              <p className="eyebrow">最近继续</p>
              <div className="home-featured-project__summary">
                <span className="home-featured-project__cover" aria-hidden="true" />
                <div>
                  <h1>{featured.title}</h1>
                  <p className="home-featured-project__workspace">
                    上次位置：{workspaceLabel(featured.lastWorkspace)}
                  </p>
                  <p className="home-featured-project__time">
                    {featured.lastOpenedAt
                      ? `上次打开：${featured.lastOpenedAt}`
                      : "尚未打开"}
                  </p>
                  {featured.missing ? (
                    <p className="home-featured-project__unavailable">路径不可用，暂时无法继续</p>
                  ) : null}
                </div>
              </div>
              <div className="home-featured-project__actions">
                <button
                  type="button"
                  className="primary-action home-continue-action"
                  disabled={busy || featured.missing}
                  onClick={() => onOpenRecent(featured)}
                >
                  <BookOpenText size={19} aria-hidden="true" />
                  继续《{featured.title}》
                </button>
                <button
                  type="button"
                  className="secondary-action home-review-action"
                  disabled={busy || featured.missing}
                  onClick={() => onReviewRecent(featured)}
                >
                  去审阅
                </button>
              </div>
            </div>
          </div>
        ) : (
          <div className="home-empty-state">
            <div className="product-mark" aria-hidden="true">
              <BookOpenText size={30} strokeWidth={1.65} />
            </div>
            <p className="eyebrow">欢迎使用织境</p>
            <h1>开始一本新作品</h1>
            <p className="home-content__description">
              从一句想法开始构建原创故事，或导入已有手稿继续写作。
            </p>
            <p className="welcome-guide">还没有最近作品，请从左侧选择一个入口开始。</p>
          </div>
        )}

        <p className="local-first-note">
          {busy ? "正在准备本地创作空间…" : "作品默认保存在本地，未经确认不会覆盖正文。"}
        </p>
      </section>
    </div>
  );
}
