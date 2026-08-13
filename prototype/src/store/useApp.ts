import { useContext } from 'react';
import { AppContext } from './appContextValue';

export function useApp() {
  const context = useContext(AppContext);
  if (!context) throw new Error('useApp 必须在 AppProvider 内使用');
  return context;
}
