import { useEffect } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { TitleBar } from './TitleBar';
import { Sidebar } from './Sidebar';
import { useApp } from '../store/useApp';
import { isApplicationWorkspace } from '../store/appFlows';

// Wails 运行时在桌面端通过 window.runtime 注入；dev 浏览器（npm run dev）下不存在。
interface WailsRuntime {
  EventsOn: (name: string, cb: (data: { command?: string; type?: string }) => void) => () => void;
  ClipboardGetText: () => Promise<string>;
  WindowMinimise: () => void;
  WindowToggleMaximise: () => void;
  WindowIsFullscreen: () => Promise<boolean>;
  WindowFullscreen: () => void;
  WindowUnfullscreen: () => void;
}
type WindowWithRuntime = Window & { runtime?: WailsRuntime };

// 原生菜单发出的事件名，与 Go 侧 menuCommandEvent 常量一致。
const menuCommandEvent = 'app:menu-command';

// 获取输入控件或页面当前选区中的文本，供原生“语音”菜单朗读。
function selectedText(): string {
  const active = document.activeElement;
  if (active instanceof HTMLInputElement || active instanceof HTMLTextAreaElement) {
    return active.value.slice(active.selectionStart ?? 0, active.selectionEnd ?? 0);
  }
  return window.getSelection()?.toString() ?? '';
}

// 将剪贴板纯文本写回当前编辑位置，并触发 React 能识别的输入事件。
function insertPlainText(text: string) {
  const active = document.activeElement;
  if (active instanceof HTMLInputElement || active instanceof HTMLTextAreaElement) {
    const start = active.selectionStart ?? active.value.length;
    const end = active.selectionEnd ?? start;
    active.setRangeText(text, start, end, 'end');
    active.dispatchEvent(new InputEvent('input', {
      bubbles: true,
      inputType: 'insertFromPaste',
      data: text,
    }));
    return;
  }
  document.execCommand('insertText', false, text);
}

// 执行中文自定义菜单对应的编辑和窗口动作。
function runNativeMenuAction(command: string, runtime: WailsRuntime): boolean {
  const editCommands: Record<string, string> = {
    edit_undo: 'undo',
    edit_redo: 'redo',
    edit_cut: 'cut',
    edit_copy: 'copy',
    edit_delete: 'delete',
    edit_select_all: 'selectAll',
  };
  const editCommand = editCommands[command];
  if (editCommand) {
    document.execCommand(editCommand);
    return true;
  }
  if (command === 'edit_paste' || command === 'edit_paste_plain') {
    void runtime.ClipboardGetText().then(insertPlainText).catch(() => undefined);
    return true;
  }
  if (command === 'speech_start') {
    const text = selectedText().trim();
    if (text && 'speechSynthesis' in window) {
      window.speechSynthesis.cancel();
      window.speechSynthesis.speak(new SpeechSynthesisUtterance(text));
    }
    return true;
  }
  if (command === 'speech_stop') {
    window.speechSynthesis?.cancel();
    return true;
  }
  if (command === 'window_minimise') {
    runtime.WindowMinimise();
    return true;
  }
  if (command === 'window_zoom') {
    runtime.WindowToggleMaximise();
    return true;
  }
  if (command === 'window_fullscreen') {
    void runtime.WindowIsFullscreen().then((fullscreen) => {
      if (fullscreen) runtime.WindowUnfullscreen();
      else runtime.WindowFullscreen();
    }).catch(() => undefined);
    return true;
  }
  return false;
}

export function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { state, dispatch, closeProject, openRecentProject, requestApplicationClose } = useApp();
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
      if (runNativeMenuAction(command, runtime)) return;
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
        case 'quit_application':
          void requestApplicationClose();
          break;
        // about/check_for_updates 暂由原生处理或后续接入。
        default:
          break;
      }
    });
    return () => off();
  }, [navigate, dispatch, closeProject, openRecentProject, requestApplicationClose]);

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
          <div data-testid="route-viewport" className="h-full min-h-full p-6">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
}
