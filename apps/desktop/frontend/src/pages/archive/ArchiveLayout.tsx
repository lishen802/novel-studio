import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  FileText, Layers, Users, Globe, History, GitBranch,
} from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { useApp } from '../../store/useApp';

const tabs = [
  { label: '作品方案', path: 'outline', icon: FileText },
  { label: '故事架构', path: 'structure', icon: Layers },
  { label: '人物与势力', path: 'characters', icon: Users },
  { label: '世界设定', path: 'world', icon: Globe },
  { label: '时间线与事实', path: 'timeline', icon: History },
  { label: '伏笔与关系', path: 'foreshadowing', icon: GitBranch },
];

export default function ArchiveLayout() {
  const navigate = useNavigate();
  const { state } = useApp();
  const confirmed = state.backendProject?.workbench.archiveStatus === 'confirmed';

  if (!confirmed) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-center space-y-4 animate-fade-up" data-testid="archive-unconfirmed">
        <h1 className="text-2xl font-display text-text">作品档案尚未确认</h1>
        <p className="text-sm text-text-sub max-w-md">确认作品档案后，人物、世界规则和大纲才会出现在这里。未确认候选不会进入正式资产。</p>
        <Button variant="primary" onClick={() => navigate('/cockpit')}>返回驾驶舱</Button>
      </div>
    );
  }

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
