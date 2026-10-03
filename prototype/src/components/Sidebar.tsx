import { NavLink, useLocation } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Compass, BookOpen, GitPullRequest, Upload, Library, GitBranch,
} from 'lucide-react';
import { useApp } from '../store/useApp';

const navItems = [
  { label: '创作驾驶舱', icon: Compass, path: '/cockpit' },
  { label: '正文与章节', icon: BookOpen, path: '/chapters' },
  { label: '审阅与候选', icon: GitPullRequest, path: '/review' },
  { label: '导入与续写', icon: Upload, path: '/import' },
  { label: '作品档案', icon: Library, path: '/archive/outline' },
  { label: '版本与交付', icon: GitBranch, path: '/versions' },
];

export function Sidebar() {
  const location = useLocation();
  const { state } = useApp();
  const project = state.currentProject;

  if (!project) return null;

  return (
    <aside className="project-navigation fixed left-0 top-10 bottom-0 w-[220px] bg-surface border-r border-border flex flex-col z-30">
      <nav className="flex-1 py-4 px-3 space-y-1">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive =
            item.path === '/archive/outline'
              ? location.pathname.startsWith('/archive')
              : location.pathname === item.path ||
                (item.path !== '/' && location.pathname.startsWith(item.path));
          return (
            <NavLink
              key={item.path}
              to={item.path}
              aria-label={item.label}
              title={item.label}
              className={clsx(
                'group flex items-center gap-3 h-10 px-3 rounded-md text-sm transition-colors relative',
                isActive
                  ? 'bg-primary-subtle text-primary'
                  : 'text-text-sub hover:bg-surface-hover hover:text-text',
              )}
            >
              {isActive && (
                <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-primary rounded-r-full" />
              )}
              <Icon className={clsx('w-5 h-5', isActive ? 'text-primary' : 'text-text-muted group-hover:text-text')} />
              <span className="navigation-label">{item.label}</span>
            </NavLink>
          );
        })}
      </nav>

      <div className="project-navigation-summary p-4 border-t border-border">
        <div className="bg-surface-elevated border border-border rounded-lg p-3">
          <p className="text-xs text-text-muted mb-1">当前作品</p>
          <p className="text-sm font-medium text-text truncate">{project.title}</p>
          <p className="text-xs text-text-sub mt-1">{project.wordCount.toLocaleString()} / {project.targetWordCount.toLocaleString()} 字</p>
          <div className="mt-2 h-1 bg-surface-hover rounded-full overflow-hidden">
            <div
              className="h-full bg-primary rounded-full"
              style={{ width: `${Math.min(100, (project.wordCount / project.targetWordCount) * 100)}%` }}
            />
          </div>
        </div>
      </div>
    </aside>
  );
}
