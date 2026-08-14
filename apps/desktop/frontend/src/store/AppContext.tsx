import React, { useReducer, useEffect, useCallback } from 'react';
import type { AppState, AppSettings, Project } from '../types';
import { defaultSettings } from '../mock/data';
import * as api from '../api/stubs';
import { AppContext, type AppAction } from './appContextValue';

const activeProjectStorageKey = 'story-studio.active-project';

/** 从浏览器保存的工作区恢复当前作品，损坏数据会被忽略。 */
function restoreActiveProject(): Project | null {
  try {
    const raw = window.localStorage.getItem(activeProjectStorageKey);
    if (!raw) return null;
    const project: unknown = JSON.parse(raw);
    if (
      typeof project === 'object' &&
      project !== null &&
      typeof (project as Project).id === 'string' &&
      typeof (project as Project).title === 'string' &&
      typeof (project as Project).status === 'string'
    ) {
      return project as Project;
    }
  } catch {
    // 本地原型的缓存异常不应阻断应用启动。
  }
  return null;
}

function createInitialState(): AppState {
  return {
    currentProject: restoreActiveProject(),
    settings: defaultSettings,
    aiPanelOpen: false,
    importStatus: 'idle',
    importProgress: 0,
  };
}

const initialState: AppState = {
  currentProject: null,
  settings: defaultSettings,
  aiPanelOpen: false,
  importStatus: 'idle',
  importProgress: 0,
};

function reducer(state: AppState, action: AppAction): AppState {
  switch (action.type) {
    case 'SET_PROJECT':
      return { ...state, currentProject: action.payload, aiPanelOpen: false };
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

export function AppProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(reducer, initialState, createInitialState);

  const loadSettings = useCallback(async () => {
    const settings = await api.fetchSettings();
    dispatch({ type: 'SET_SETTINGS', payload: settings });
  }, []);

  const saveSettings = useCallback(async (settings: AppSettings) => {
    const saved = await api.saveSettings(settings);
    dispatch({ type: 'SET_SETTINGS', payload: saved });
  }, []);

  useEffect(() => {
    loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    if (state.currentProject) {
      window.localStorage.setItem(activeProjectStorageKey, JSON.stringify(state.currentProject));
      return;
    }
    window.localStorage.removeItem(activeProjectStorageKey);
  }, [state.currentProject]);

  return (
    <AppContext.Provider value={{ state, dispatch, loadSettings, saveSettings }}>
      {children}
    </AppContext.Provider>
  );
}
