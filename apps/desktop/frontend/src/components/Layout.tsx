import { useEffect } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { clsx } from 'clsx';
import { TitleBar } from './TitleBar';
import { Sidebar } from './Sidebar';
import { AiPanel } from './AiPanel';
import { useApp } from '../store/useApp';
import { isApplicationWorkspace } from '../store/appFlows';

export function Layout() {
  const location = useLocation();
  const { state, dispatch } = useApp();
  const applicationWorkspace = isApplicationWorkspace(location.pathname);
  const hasProject = state.currentProject !== null;
  const showProjectShell = !applicationWorkspace && hasProject;
  const showAiPanel = showProjectShell && ['/chapters', '/review'].includes(location.pathname);

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
