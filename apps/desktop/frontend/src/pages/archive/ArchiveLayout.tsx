import { NavLink, Outlet } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  FileText, Layers, Users, Globe, History, GitBranch,
} from 'lucide-react';

const tabs = [
  { label: '作品方案', path: 'outline', icon: FileText },
  { label: '故事架构', path: 'structure', icon: Layers },
  { label: '人物与势力', path: 'characters', icon: Users },
  { label: '世界设定', path: 'world', icon: Globe },
  { label: '时间线与事实', path: 'timeline', icon: History },
  { label: '伏笔与关系', path: 'foreshadowing', icon: GitBranch },
];

export default function ArchiveLayout() {
  return (
    <div className="h-full flex flex-col animate-fade-up">
      <div className="mb-6">
        <h1 className="text-2xl font-display text-text mb-1">作品档案</h1>
        <p className="text-sm text-text-sub">管理作品的设定、人物、故事结构与伏笔线索。</p>
      </div>

      <nav className="border-b border-border mb-6">
        <div className="flex gap-1 overflow-x-auto">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <NavLink
                key={tab.path}
                to={tab.path}
                end
                className={({ isActive }) =>
                  clsx(
                    'group flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap',
                    isActive
                      ? 'text-primary border-primary'
                      : 'text-text-sub border-transparent hover:text-text hover:border-border-strong',
                  )
                }
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </NavLink>
            );
          })}
        </div>
      </nav>

      <div className="flex-1 min-h-0">
        <Outlet />
      </div>
    </div>
  );
}
