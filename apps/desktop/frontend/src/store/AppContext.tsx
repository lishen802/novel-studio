import React, { useCallback, useEffect, useReducer, useRef } from 'react';
import type { AppState, AppSettings, Project, ProjectStatus } from '../types';
import type { ApplicationBootstrapView, ProjectView } from '../api/studio';
import { defaultSettings } from './defaultSettings';
import { studio } from '../api/studio';
import { AppContext, type AppAction } from './appContextValue';

interface WailsRuntime {
  EventsOn: (name: string, callback: () => void) => () => void;
}

type WindowWithRuntime = Window & { runtime?: WailsRuntime };

/** 将后端权威项目投影转换为当前原型页面所需的展示模型。 */
function projectModel(project: ProjectView): Project {
  const stage = project.workbench?.stage || 'writing';
  const status: ProjectStatus = stage === 'review'
    ? 'review'
    : stage === 'idea' || stage === 'proposal' || stage === 'roadmap'
      ? 'creative'
      : 'writing';
  return {
    id: project.id,
    title: project.title,
    subtitle: project.genre || '未分类长篇',
    author: '',
    wordCount: 0,
    targetWordCount: 0,
    status,
    phase: stage,
    updatedAt: '',
    coverColor: '#3a4a5c',
    synopsis: project.summary,
  };
}

/** 把应用级连接和方案投影进现有设置界面，不暴露任何密钥。 */
function settingsFromBootstrap(bootstrap: ApplicationBootstrapView, previous: AppSettings): AppSettings {
  const configuredModels = bootstrap.providers.map((provider) => ({
    id: provider.id,
    name: provider.name,
    provider: provider.kind,
    baseUrl: provider.baseUrl,
    modelName: '',
    apiKeyConfigured: provider.hasCredential,
  }));
  const models = configuredModels.length > 0 ? configuredModels : [{
    id: 'm-new',
    name: '新模型连接',
    provider: 'openai',
    baseUrl: 'https://api.openai.com/v1',
    modelName: '',
    apiKeyConfigured: false,
  }];
  const profiles = bootstrap.profiles.map((profile) => ({
    id: profile.profileId,
    name: profile.name,
    version: profile.version,
    published: true,
    roleModels: Object.fromEntries(profile.roles.map((role) => [role.role, role.connectionId])),
  })) as AppSettings['creativeProfiles'];
  return {
    ...previous,
    theme: (bootstrap.preferences.theme as AppSettings['theme']) || previous.theme,
    models,
    activeModelId: models[0]?.id || '',
    creativeProfiles: profiles,
    defaultCreativeProfileId: bootstrap.preferences.defaultCreativeProfileId || profiles[0]?.id || '',
    writingPlan: profiles[0]?.name || '尚未配置创作方案',
  };
}

function createInitialState(): AppState {
  return {
    currentProject: null,
    backendProject: null,
    bootstrap: null,
    desktopAvailable: studio.available(),
    busy: false,
    error: null,
    settings: defaultSettings,
    aiPanelOpen: false,
    importStatus: 'idle',
    importProgress: 0,
  };
}

function reducer(state: AppState, action: AppAction): AppState {
  switch (action.type) {
    case 'SET_PROJECT':
      return { ...state, currentProject: action.payload, aiPanelOpen: false };
    case 'SET_BACKEND_PROJECT':
      return {
        ...state,
        backendProject: action.payload,
        currentProject: action.payload ? projectModel(action.payload) : null,
        aiPanelOpen: false,
      };
    case 'SET_BOOTSTRAP':
      return { ...state, bootstrap: action.payload };
    case 'SET_BUSY':
      return { ...state, busy: action.payload };
    case 'SET_ERROR':
      return { ...state, error: action.payload };
    case 'TOGGLE_AI_PANEL':
      return { ...state, aiPanelOpen: !state.aiPanelOpen };
    case 'SET_IMPORT_STATUS':
      return { ...state, importStatus: action.payload };
    case 'SET_IMPORT_PROGRESS':
      return { ...state, importProgress: action.payload };
    case 'SET_SETTINGS':
      return { ...state, settings: action.payload };
    case 'SET_EDITOR_SETTINGS':
      return { ...state, settings: { ...state.settings, ...action.payload } };
    default:
      return state;
  }
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败，请重试';
}

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(reducer, undefined, createInitialState);
  const beforeCloseHandler = useRef<(() => Promise<void>) | null>(null);
  const closing = useRef(false);

  // 页面可注册退出前刷新动作；正文编辑器用它落盘尚未触发防抖保存的内容。
  const registerBeforeClose = useCallback((handler: () => Promise<void>) => {
    beforeCloseHandler.current = handler;
    return () => {
      if (beforeCloseHandler.current === handler) beforeCloseHandler.current = null;
    };
  }, []);

  // 原生关闭按钮和“退出”菜单共用同一条链路，保证先保存再通知后端放行。
  const requestApplicationClose = useCallback(async () => {
    if (closing.current || !studio.available()) return;
    closing.current = true;
    try {
      await beforeCloseHandler.current?.();
    } catch (error) {
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
    }
    try {
      await studio.completeClose();
    } catch (error) {
      closing.current = false;
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
    }
  }, []);

  const refreshBootstrap = useCallback(async () => {
    if (!studio.available()) return;
    try {
      const bootstrap = await studio.applicationBootstrap();
      dispatch({ type: 'SET_BOOTSTRAP', payload: bootstrap });
      dispatch({ type: 'SET_SETTINGS', payload: settingsFromBootstrap(bootstrap, state.settings) });
      dispatch({ type: 'SET_ERROR', payload: null });
    } catch (error) {
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
    }
  }, [state.settings]);

  const loadSettings = useCallback(async () => {
    await refreshBootstrap();
  }, [refreshBootstrap]);

  const saveSettings = useCallback(async (settings: AppSettings) => {
    if (studio.available()) {
      await studio.saveApplicationPreferences({
        theme: settings.theme,
        defaultCreativeProfileId: settings.defaultCreativeProfileId,
      });
    }
    dispatch({ type: 'SET_SETTINGS', payload: settings });
  }, []);

  const activateProject = useCallback((project: ProjectView) => {
    dispatch({ type: 'SET_BACKEND_PROJECT', payload: project });
    dispatch({ type: 'SET_ERROR', payload: null });
  }, []);

  const createProject = useCallback(async (input: { title: string; genre: string; author: string }) => {
    dispatch({ type: 'SET_BUSY', payload: true });
    try {
      const parentDirectory = await studio.chooseProjectParent();
      if (!parentDirectory) return false;
      const project = await studio.createProject({
        parentDirectory,
        title: input.title,
        genre: input.genre,
        summary: '',
      });
      activateProject(project);
      await refreshBootstrap();
      return true;
    } catch (error) {
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
      return false;
    } finally {
      dispatch({ type: 'SET_BUSY', payload: false });
    }
  }, [activateProject, refreshBootstrap]);

  const openProject = useCallback(async () => {
    dispatch({ type: 'SET_BUSY', payload: true });
    try {
      const path = await studio.chooseProjectToOpen();
      if (!path) return false;
      const result = await studio.openProject(path);
      if (result.outcome !== 'opened' || !result.project) {
        throw new Error('项目正在被其他实例使用，请稍后重试或以只读方式打开。');
      }
      activateProject(result.project);
      await refreshBootstrap();
      return true;
    } catch (error) {
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
      return false;
    } finally {
      dispatch({ type: 'SET_BUSY', payload: false });
    }
  }, [activateProject, refreshBootstrap]);

  const openRecentProject = useCallback(async (projectId: string) => {
    dispatch({ type: 'SET_BUSY', payload: true });
    try {
      const result = await studio.openRecentProject(projectId);
      if (result.outcome !== 'opened' || !result.project) throw new Error('无法打开最近项目');
      activateProject(result.project);
      return true;
    } catch (error) {
      dispatch({ type: 'SET_ERROR', payload: errorMessage(error) });
      return false;
    } finally {
      dispatch({ type: 'SET_BUSY', payload: false });
    }
  }, [activateProject]);

  const closeProject = useCallback(async () => {
    if (studio.available()) await studio.closeProject();
    dispatch({ type: 'SET_BACKEND_PROJECT', payload: null });
  }, []);

  useEffect(() => {
    void refreshBootstrap();
    // Wails bridge 只在应用初次挂载时读取。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const runtime = (window as WindowWithRuntime).runtime;
    if (!runtime) return;
    const off = runtime.EventsOn('app:before-close', () => {
      void requestApplicationClose();
    });
    return () => off();
  }, [requestApplicationClose]);

  return (
    <AppContext.Provider value={{
      state,
      dispatch,
      loadSettings,
      saveSettings,
      refreshBootstrap,
      createProject,
      openProject,
      openRecentProject,
      closeProject,
      registerBeforeClose,
      requestApplicationClose,
    }}>
      {children}
    </AppContext.Provider>
  );
}
