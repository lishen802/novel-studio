import { useNavigate } from 'react-router-dom';
import { Settings } from 'lucide-react';
import { useApp } from '../store/useApp';
import { getActiveModel } from '../types';

// 顶部状态栏：仅承载应用名与模型配置提示。
// 应用菜单由 Wails 原生菜单统一承载——
// macOS 渲染到系统顶部菜单栏，Windows 渲染到窗口菜单栏，
// 此处不再用 JSX 重复绘制下拉菜单，避免与原生菜单冲突。
export function TitleBar() {
  const navigate = useNavigate();
  const { state } = useApp();

  return (
    <div className="h-10 bg-surface border-b border-border flex items-center px-4 select-none shrink-0 z-40">
      <span className="text-sm font-medium text-text">织境 Story Studio</span>

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
