import { createContext } from 'react';
import type React from 'react';
import type { AppState, AppSettings, Project } from '../types';
import type { ApplicationBootstrapView, ProjectView } from '../api/studio';

export type AppAction =
  | { type: 'SET_PROJECT'; payload: Project | null }
  | { type: 'SET_BACKEND_PROJECT'; payload: ProjectView | null }
  | { type: 'SET_BOOTSTRAP'; payload: ApplicationBootstrapView }
  | { type: 'SET_BUSY'; payload: boolean }
  | { type: 'SET_ERROR'; payload: string | null }
  | { type: 'TOGGLE_AI_PANEL' }
  | { type: 'SET_IMPORT_STATUS'; payload: AppState['importStatus'] }
  | { type: 'SET_IMPORT_PROGRESS'; payload: number }
  | { type: 'SET_SETTINGS'; payload: AppSettings }
  | { type: 'SET_EDITOR_SETTINGS'; payload: Partial<AppSettings> };

export interface AppCtxValue {
  state: AppState;
  dispatch: React.Dispatch<AppAction>;
  loadSettings: () => Promise<void>;
  saveSettings: (settings: AppSettings) => Promise<void>;
  refreshBootstrap: () => Promise<void>;
  createProject: (input: { title: string; genre: string; author: string }) => Promise<boolean>;
  openProject: () => Promise<boolean>;
  openRecentProject: (projectId: string) => Promise<boolean>;
  closeProject: () => Promise<void>;
}

export const AppContext = createContext<AppCtxValue | undefined>(undefined);
