import { useEffect } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { clsx } from 'clsx';
import { TitleBar } from './TitleBar';
import { Sidebar } from './Sidebar';
import { AiPanel } from './AiPanel';
import { useApp } from '../store/useApp';
import { isApplicationWorkspace } from '../store/appFlows';

// Wails 运行时在桌面端通过 window.runtime 注入；dev 浏览器（npm run dev）下不存在。
interface WailsRuntime {
  EventsOn: (name: string, cb: (data: { type: string }) => void) => () => void;
}
type WindowWithRuntime = Window & { runtime?: WailsRuntime };

// 原生菜单发出的事件名，与 Go 侧 menuCommandEvent 常量一致。
const menuCommandEvent = 'app:menu-command';

export function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { state, dispatch } = useApp();
  const applicationWorkspace = isApplicationWorkspace(location.pathname);
  const hasProject = state.currentProject !== null;
  const showProjectShell = !applicationWorkspace && hasProject;
  const showAiPanel = showProjectShell && ['/chapters', '/review'].includes(location.pathname);

  // 原生菜单（macOS 系统顶部 / Windows 窗口菜单栏）点击后由 Go 侧 emit
  // app:menu-command 事件；此处将稳定命令映射到路由跳转与 store 动作。
  // dev 浏览器无 window.runtime，监听自动跳过。
  useEffect(() => {
    const runtime = (window as WindowWithRuntime).runtime;
    if (!runtime) return;
    const off = runtime.EventsOn(menuCommandEvent, (data) => {
      switch (data.type) {
        case 'app_settings':
          navigate('/settings');
          break;
        case 'project_settings':
          navigate('/project-settings');
          break;
        case 'import_novel':
          navigate('/import');
          break;
        case 'new_original':
          navigate('/?new=1');
          break;
        case 'open_project':
          navigate('/?open=1');
          break;
        case 'close_project':
          dispatch({ type: 'SET_PROJECT', payload: null });
          navigate('/');
          break;
        case 'export':
        case 'project_maintenance':
          navigate('/versions');
          break;
        case 'toggle_ai_panel':
        case 'focus_ai':
          dispatch({ type: 'TOGGLE_AI_PANEL' });
          break;
        // about/check_for_updates/quit_application/open_recent:* 暂由原生处理或后续接入。
        default:
          break;
      }
    });
    return () => off();
  }, [navigate, dispatch]);

  useEffect(() => {
    if (!showAiPanel) return;
    const handleShortcut = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest('input, textarea, [contenteditable="true"]')) return;
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'j') {
        event.preventDefault();
        dispatch({ type: 'TOGGLE_AI_PANEL' });
      }
    };
    window.addEventListener('keydown', handleShortcut);
    return () => window.removeEventListener('keydown', handleShortcut);
  }, [dispatch, showAiPanel]);

  if (!applicationWorkspace && !hasProject) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="h-full flex flex-col bg-atmosphere">
      <TitleBar />
      <div className="flex-1 flex overflow-hidden relative">
        {showProjectShell && <Sidebar />}
        <main
          className={clsx(
            'flex-1 overflow-auto bg-atmosphere relative transition-[margin-right] duration-220',
            showProjectShell && 'ml-[220px]',
            showAiPanel && state.aiPanelOpen && 'mr-[360px]',
          )}
          style={{ transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)' }}
        >
          <div className="min-h-full p-6">
            <Outlet />
          </div>
        </main>
        {showAiPanel && <AiPanel mode={location.pathname === '/review' ? 'review' : 'writing'} />}
      </div>
    </div>
  );
}
