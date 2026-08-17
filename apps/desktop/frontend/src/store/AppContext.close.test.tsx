import { useEffect } from 'react';
import { act, render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppProvider } from './AppContext';
import { useApp } from './useApp';

interface TestRuntime {
  EventsOn: (name: string, callback: () => void) => () => void;
}

function CloseProbe({ flush }: { flush: () => Promise<void> }) {
  const { registerBeforeClose } = useApp();

  useEffect(() => registerBeforeClose(flush), [flush, registerBeforeClose]);
  return null;
}

describe('应用关闭协调', () => {
  afterEach(() => {
    delete window.go;
    delete (window as Window & { runtime?: TestRuntime }).runtime;
  });

  it('收到原生关闭事件后先刷新页面草稿，再允许后端退出', async () => {
    let beforeClose: (() => void) | undefined;
    const eventsOn = vi.fn((name: string, callback: () => void) => {
      if (name === 'app:before-close') beforeClose = callback;
      return () => undefined;
    });
    (window as Window & { runtime?: TestRuntime }).runtime = { EventsOn: eventsOn };

    const calls: string[] = [];
    const completeClose = vi.fn(async () => { calls.push('close'); });
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: vi.fn().mockResolvedValue({ preferences: {}, recentProjects: [], providers: [], profiles: [] }),
      CompleteClose: completeClose,
    } } };

    render(
      <AppProvider>
        <CloseProbe flush={async () => { calls.push('flush'); }} />
      </AppProvider>,
    );

    expect(eventsOn).toHaveBeenCalledWith('app:before-close', expect.any(Function));
    await act(async () => { beforeClose?.(); });

    expect(calls).toEqual(['flush', 'close']);
    expect(completeClose).toHaveBeenCalledTimes(1);
  });
});
