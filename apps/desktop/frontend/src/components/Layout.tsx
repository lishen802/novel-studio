import { useEffect } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { TitleBar } from './TitleBar';
import { Sidebar } from './Sidebar';
import { useApp } from '../store/useApp';
import { isApplicationWorkspace } from '../store/appFlows';

// Wails 运行时在桌面端通过 window.runtime 注入；dev 浏览器（npm run dev）下不存在。
interface WailsRuntime {
  EventsOn: (name: string, cb: (data: { command?: string; type?: string }) => void) => () => void;
}
type WindowWithRuntime = Window & { runtime?: WailsRuntime };

// 原生菜单发出的事件名，与 Go 侧 menuCommandEvent 常量一致。
const menuCommandEvent = 'app:menu-command';

export function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { state, dispatch, closeProject, openRecentProject } = useApp();
  const applicationWorkspace = isApplicationWorkspace(location.pathname);
  const hasProject = state.currentProject !== null;
  const showProjectShell = !applicationWorkspace && hasProject;

  // 原生菜单（macOS 系统顶部 / Windows 窗口菜单栏）点击后由 Go 侧 emit
  // app:menu-command 事件；此处将稳定命令映射到路由跳转与 store 动作。
  // dev 浏览器无 window.runtime，监听自动跳过。
  useEffect(() => {
    const runtime = (window as WindowWithRuntime).runtime;
    if (!runtime) return;
    const off = runtime.EventsOn(menuCommandEvent, (data) => {
      const command = data.command || data.type || '';
      if (command.startsWith('open_recent:')) {
        void openRecentProject(command.slice('open_recent:'.length)).then((opened) => {
          if (opened) navigate('/chapters');
        });
        return;
      }
      switch (command) {
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
          void closeProject().then(() => navigate('/'));
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
  }, [navigate, dispatch, closeProject, openRecentProject]);

  if (!applicationWorkspace && !hasProject) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="h-full flex flex-col bg-atmosphere">
      <TitleBar />
      <div className="flex-1 flex overflow-hidden relative">
        {showProjectShell && <Sidebar />}
        <main
          className={`flex-1 overflow-auto bg-atmosphere relative ${showProjectShell ? 'ml-[220px]' : ''}`}
          style={{ transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)' }}
        >
          <div className="min-h-full p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
