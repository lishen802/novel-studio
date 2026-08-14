import { useNavigate } from 'react-router-dom';
import { Settings } from 'lucide-react';
import { useApp } from '../store/useApp';

// 顶部状态栏：仅承载应用名与模型配置提示。
// 应用菜单由 Wails 原生菜单统一承载——
// macOS 渲染到系统顶部菜单栏，Windows 渲染到窗口菜单栏，
// 此处不再用 JSX 重复绘制下拉菜单，避免与原生菜单冲突。
export function TitleBar() {
  const navigate = useNavigate();
  const { state } = useApp();
  const isMac = /Macintosh|Mac OS X/.test(window.navigator.userAgent);
  const configured = state.bootstrap?.providers.some((provider) => provider.hasCredential) || false;

  const modelButton = (
    <button
      onClick={() => navigate('/settings')}
      className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-warning bg-warning/10 border border-warning/20 rounded-md hover:bg-warning/15 transition-colors"
      style={{ '--wails-draggable': 'no-drag' } as React.CSSProperties}
    >
      <Settings className="w-3.5 h-3.5" />
      {configured ? '模型设置' : '未配置模型'}
    </button>
  );

  if (!isMac) {
    return <div className="fixed right-4 top-2 z-50">{modelButton}</div>;
  }

  return (
    <div
      className="h-10 bg-surface border-b border-border flex items-center pl-[86px] pr-4 select-none shrink-0 z-40"
      style={{ '--wails-draggable': 'drag' } as React.CSSProperties}
    >
      <span className="text-sm font-medium text-text">织境 Story Studio</span>

      <div className="ml-auto flex items-center gap-2">
        {modelButton}
      </div>
    </div>
  );
}
