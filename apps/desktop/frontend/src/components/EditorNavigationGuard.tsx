import { useContext, useEffect, useRef } from 'react';
import { UNSAFE_DataRouterContext, useBlocker } from 'react-router-dom';
import { useApp } from '../store/useApp';

// 真实桌面路由在卸载编辑页前刷新草稿；失败时保持原页面供作者重试。
function DataNavigationGuard() {
  const { flushEdits, hasEditGuard, dispatch } = useApp();
  const blocker = useBlocker(({ currentLocation, nextLocation }) => currentLocation.pathname !== nextLocation.pathname && Boolean(hasEditGuard?.()));
  const active = useRef(false);
  useEffect(() => {
    if (blocker.state !== 'blocked' || active.current) return;
    active.current = true;
    void (async () => {
      try { await flushEdits?.(); blocker.proceed(); }
      catch (error) {
        dispatch({ type: 'SET_ERROR', payload: error instanceof Error ? error.message : '保存失败，已保留当前页面和输入。' });
        blocker.reset();
      } finally { active.current = false; }
    })();
  }, [blocker, flushEdits, dispatch]);
  return null;
}
export function EditorNavigationGuard() {
  const router = useContext(UNSAFE_DataRouterContext);
  return router ? <DataNavigationGuard /> : null;
}
