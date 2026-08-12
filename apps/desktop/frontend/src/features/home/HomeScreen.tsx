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
};

export function HomeScreen({
  bootstrap,
  busy,
  onBeginCreate,
  onOpenProject,
  onImportManuscript,
  onOpenRecent,
}: HomeScreenProps) {
  const recent = bootstrap.recentProjects;
  const hasRecent = recent.length > 0;

  return (
    <section className="welcome-card" aria-labelledby="product-title">
      <div className="product-mark" aria-hidden="true">
        <BookOpenText size={30} strokeWidth={1.65} />
      </div>
      <p className="eyebrow">长篇小说智能创作工作台</p>
      <h1 id="product-title">织境 Story Studio</h1>
      <p className="welcome-card__description">
        从故事工程开始，让智能体参与规划、写作与审阅，同时由作者掌握最终决定。
      </p>
      <div className="welcome-actions">
        <button
          type="button"
          className="primary-action"
          disabled={busy}
          onClick={onBeginCreate}
        >
          <Lightbulb size={18} aria-hidden="true" />
          从一个想法开始
        </button>
        <button
          type="button"
          className="secondary-action"
          disabled={busy}
          onClick={onImportManuscript}
        >
          <Upload size={18} aria-hidden="true" />
          导入小说并续写
        </button>
        <button
          type="button"
          className="secondary-action"
          disabled={busy}
          onClick={onOpenProject}
        >
          <FolderOpen size={18} aria-hidden="true" />
          打开项目
        </button>
      </div>
      <p className="local-first-note">
        {busy ? "正在准备本地创作空间…" : "作品默认保存在本地，未经确认不会覆盖正文。"}
      </p>

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
                <span className="recent-project-card__title">
                  {project.title}
                </span>
                <span className="recent-project-card__meta">
                  {project.missing
                    ? "路径不可用"
                    : project.lastOpenedAt
                      ? `上次打开于 ${project.lastOpenedAt}`
                      : "尚未打开"}
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="welcome-guide">
          暂无最近项目。从「一个想法」开始或导入已有的小说手稿，让织境帮你建立故事工程。
        </p>
      )}
    </section>
  );
}
