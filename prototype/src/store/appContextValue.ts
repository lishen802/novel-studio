import { createContext } from 'react';
import type React from 'react';
import type { AppState, AppSettings, Project } from '../types';

export type AppAction =
  | { type: 'SET_PROJECT'; payload: Project | null }
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
}

export const AppContext = createContext<AppCtxValue | undefined>(undefined);
