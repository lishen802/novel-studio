import { useState } from 'react';
import { clsx } from 'clsx';
import { useNavigate } from 'react-router-dom';
import { Settings } from 'lucide-react';
import { useApp } from '../store/useApp';
import { getActiveModel } from '../types';

const appMenu = [
  { label: '关于 织境 Story Studio', action: 'about' },
  { label: '应用设置…', action: 'settings', shortcut: '⌘,' },
  { label: '检查更新', action: 'update' },
  { label: '服务', action: 'services' },
  null,
  { label: '隐藏 织境 Story Studio', action: 'hide', shortcut: '⌘H' },
  { label: '退出', action: 'quit', shortcut: '⌘Q' },
];

const fileMenu = [
  { label: '新建项目…', action: 'new', shortcut: '⌘N' },
  { label: '打开…', action: 'open', shortcut: '⌘O' },
  { label: '打开最近项目', action: 'recent' },
  null,
  { label: '关闭项目', action: 'close' },
  { label: '导入小说并续写…', action: 'import' },
  { label: '项目设置…', action: 'project-settings' },
  null,
  { label: '导出', action: 'export' },
  { label: '项目维护…', action: 'maintenance' },
];

const editMenu = [
  { label: '撤销', action: 'undo', shortcut: '⌘Z' },
  { label: '重做', action: 'redo', shortcut: '⇧⌘Z' },
  null,
  { label: '剪切', action: 'cut', shortcut: '⌘X' },
  { label: '复制', action: 'copy', shortcut: '⌘C' },
  { label: '粘贴', action: 'paste', shortcut: '⌘V' },
];

const viewMenu = [
  { label: '显示/隐藏 AI 面板', action: 'ai-panel', shortcut: '⌘J' },
  { label: '进入专注模式', action: 'focus', shortcut: '⌘\\' },
];

interface MenuDef {
  label: string;
  items: (null | { label: string; action: string; shortcut?: string })[];
}

const menus: MenuDef[] = [
  { label: '织境 Story Studio', items: appMenu },
  { label: '文件', items: fileMenu },
  { label: '编辑', items: editMenu },
  { label: '视图', items: viewMenu },
  { label: '窗口', items: [] },
  { label: '帮助', items: [] },
];

export function TitleBar() {
  const [activeMenu, setActiveMenu] = useState<string | null>(null);
  const navigate = useNavigate();
  const { state, dispatch } = useApp();

  const handleAction = (action: string) => {
    setActiveMenu(null);
    if (action === 'settings') navigate('/settings');
    if (action === 'project-settings') navigate('/project-settings');
    if (action === 'import') navigate('/import');
    if (action === 'new') navigate('/?new=1');
    if (action === 'open') navigate('/?open=1');
    if (action === 'close') {
      dispatch({ type: 'SET_PROJECT', payload: null });
      navigate('/');
    }
    if (action === 'maintenance') navigate('/versions');
    if (action === 'ai-panel') dispatch({ type: 'TOGGLE_AI_PANEL' });
  };

  return (
    <div className="h-10 bg-surface border-b border-border flex items-center px-4 select-none shrink-0 z-40">
      {/* macOS traffic lights */}
      <div className="flex items-center gap-2 mr-6">
        <span className="w-3 h-3 rounded-full bg-[#ff5f57] border border-black/10" />
        <span className="w-3 h-3 rounded-full bg-[#febc2e] border border-black/10" />
        <span className="w-3 h-3 rounded-full bg-[#28c840] border border-black/10" />
      </div>

      {/* Menus */}
      <div className="flex items-center gap-1 relative">
        {menus.map((menu) => (
          <div key={menu.label} className="relative">
            <button
              className={clsx(
                'px-3 py-1 text-sm rounded transition-colors',
                activeMenu === menu.label
                  ? 'bg-primary text-white'
                  : 'text-text-sub hover:text-text hover:bg-surface-hover',
              )}
              onClick={() => setActiveMenu(activeMenu === menu.label ? null : menu.label)}
              onMouseEnter={() => activeMenu && setActiveMenu(menu.label)}
            >
              {menu.label}
            </button>
            {activeMenu === menu.label && menu.items.length > 0 && (
              <div
                className="absolute top-full left-0 mt-1 w-56 bg-surface-popup border border-border rounded-lg shadow-lg py-1 z-50"
                onMouseLeave={() => setActiveMenu(null)}
              >
                {menu.items.map((item, idx) =>
                  item === null ? (
                    <div key={idx} className="h-px bg-border my-1" />
                  ) : (
                    <button
                      key={item.action}
                      className="w-full flex items-center justify-between px-4 py-1.5 text-sm text-text hover:bg-surface-hover transition-colors"
                      onClick={() => handleAction(item.action)}
                    >
                      <span>{item.label}</span>
                      {item.shortcut && (
                        <span className="text-xs text-text-muted">{item.shortcut}</span>
                      )}
                    </button>
                  ),
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="ml-auto flex items-center gap-2">
        {!getActiveModel(state.settings).apiKeyConfigured && (
          <button
            onClick={() => navigate('/settings')}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-warning bg-warning/10 border border-warning/20 rounded-md hover:bg-warning/15 transition-colors"
          >
            <Settings className="w-3.5 h-3.5" />
            未配置模型
          </button>
        )}
      </div>
    </div>
  );
}
