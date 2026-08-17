import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Settings from './Settings';
import { AppProvider } from '../store/AppContext';

describe('应用级创作方案', () => {
  beforeEach(() => {
    window.go = { main: { DesktopAPI: { ApplicationBootstrap: vi.fn().mockResolvedValue({
      preferences: { theme: 'light', defaultCreativeProfileId: 'profile-1' }, recentProjects: [],
      providers: [{ id: 'provider-1', name: '主模型', kind: 'openai', baseUrl: '', hasCredential: true, createdAt: '', updatedAt: '' }],
      profiles: [{ profileId: 'profile-1', version: 1, name: '长篇方案', maxRunTokens: 1000, maxContextChars: 1000, createdAt: '', roles: ['planning', 'writing', 'review', 'memory', 'impact'].map((role) => ({ role, connectionId: 'provider-1', model: 'model', maxOutputTokens: 1000, temperature: 0.5, enabled: true })) }],
    }) } } };
  });
  afterEach(() => { delete window.go; });

  it('为规划、写作、审阅、记忆和影响分析分别配置模型，并说明默认只影响新作品', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AppProvider>
          <Settings />
        </AppProvider>
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', { name: '返回应用' })).toBeTruthy();
    expect(screen.getByRole('textbox', { name: '搜索设置' })).toBeTruthy();
    await user.click(screen.getByRole('button', { name: '创作方案' }));

    expect(await screen.findByText('角色分配')).toBeTruthy();
    expect(screen.getByLabelText('规划')).toBeTruthy();
    expect(screen.getByLabelText('写作')).toBeTruthy();
    expect(screen.getByLabelText('审阅')).toBeTruthy();
    expect(screen.getByLabelText('记忆')).toBeTruthy();
    expect(screen.getByLabelText('影响分析')).toBeTruthy();
    expect(screen.getByText('只影响以后新建的作品')).toBeTruthy();
  });
});
