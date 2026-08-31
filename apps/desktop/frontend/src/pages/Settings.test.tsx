import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Settings from './Settings';
import { AppProvider } from '../store/AppContext';

describe('应用级创作方案', () => {
  const applicationBootstrap = vi.fn();
  const saveProvider = vi.fn();
  const savePreferences = vi.fn();
  const deleteProvider = vi.fn();

  beforeEach(() => {
    applicationBootstrap.mockResolvedValue({
      preferences: { theme: 'light', defaultCreativeProfileId: 'profile-1' }, recentProjects: [],
      providers: [{ id: 'provider-1', name: '主模型', kind: 'openai', baseUrl: '', hasCredential: true, createdAt: '', updatedAt: '' }],
      profiles: [{ profileId: 'profile-1', version: 1, name: '长篇方案', maxRunTokens: 100000000, maxContextChars: 100000000, createdAt: '', roles: ['planner', 'writer', 'continuity_reviewer'].map((role) => ({ role, connectionId: 'provider-1', model: 'model', reasoningEffort: 'auto', outputLimitMode: 'profile_budget', temperature: 0.5, enabled: true })) }],
    });
    saveProvider.mockResolvedValue({ id: 'provider-1', name: '主模型', kind: 'openai', baseUrl: '', hasCredential: true, createdAt: '', updatedAt: '' });
    savePreferences.mockResolvedValue({ theme: 'light', defaultCreativeProfileId: 'profile-1' });
    deleteProvider.mockResolvedValue(undefined);
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: applicationBootstrap,
      SaveApplicationProvider: saveProvider,
      SaveApplicationPreferences: savePreferences,
      DeleteApplicationProvider: deleteProvider,
    } } };
  });
  afterEach(() => {
    cleanup();
    delete window.go;
    vi.clearAllMocks();
  });

  it('为规划、写作和审阅三个角色分别配置模型，并说明默认只影响新作品', async () => {
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
    expect(screen.getByLabelText('规划推理强度')).toBeTruthy();
    expect(screen.getByLabelText('写作推理强度')).toBeTruthy();
    expect(screen.getByLabelText('审阅推理强度')).toBeTruthy();
    expect(screen.getByLabelText('运行 Token 预算')).toBeTruthy();
    expect(screen.getByText('只影响以后新建的作品')).toBeTruthy();
  });

  it('支持选择推理强度、预置预算、不限制和自定义预算', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AppProvider>
          <Settings />
        </AppProvider>
      </MemoryRouter>,
    );

    await user.click(await screen.findByRole('button', { name: '创作方案' }));
    await user.selectOptions(screen.getByLabelText('写作推理强度'), 'high');
    expect((screen.getByLabelText('写作推理强度') as HTMLSelectElement).value).toBe('high');

    await user.selectOptions(screen.getByLabelText('运行 Token 预算'), 'unlimited');
    expect((screen.getByLabelText('运行 Token 预算') as HTMLSelectElement).value).toBe('unlimited');
    expect(screen.queryByLabelText('运行 Token 预算自定义')).toBeNull();

    await user.selectOptions(screen.getByLabelText('运行 Token 预算'), 'custom');
    const custom = await screen.findByLabelText('运行 Token 预算自定义') as HTMLInputElement;
    expect(custom.value).toBe('10000');
    fireEvent.change(custom, { target: { value: '2' } });
    expect((screen.getByLabelText('运行 Token 预算自定义') as HTMLInputElement).value).toBe('2');
  });

  it('选择内置 DeepSeek 厂商后自动填充接口与模型目录', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AppProvider><Settings /></AppProvider>
      </MemoryRouter>,
    );

    const vendor = await screen.findByLabelText('模型厂商');
    await user.selectOptions(vendor, 'deepseek');

    expect((screen.getByLabelText('接口地址') as HTMLInputElement).value).toBe('https://api.deepseek.com');
    expect((screen.getByLabelText('测试模型') as HTMLSelectElement).value).toBe('deepseek-v4-flash');
    expect(screen.getByRole('option', { name: 'DeepSeek V4 Pro' })).toBeTruthy();
  });

  it('保存应用设置后仍保留方案角色的模型绑定', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AppProvider><Settings /></AppProvider>
      </MemoryRouter>,
    );

    await user.click(await screen.findByRole('button', { name: '创作方案' }));
    expect((screen.getByLabelText('规划模型') as HTMLSelectElement).value).toBe('__custom__');
    expect((screen.getByLabelText('规划自定义模型 ID') as HTMLInputElement).value).toBe('model');
    await user.click(screen.getByRole('button', { name: '保存设置' }));
    expect((await screen.findByLabelText('规划模型') as HTMLSelectElement).value).toBe('__custom__');
    expect((screen.getByLabelText('规划自定义模型 ID') as HTMLInputElement).value).toBe('model');
  });

  it('删除被方案引用的连接时显示应用内原因而不是调用删除', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AppProvider><Settings /></AppProvider>
      </MemoryRouter>,
    );

    await user.click(await screen.findByRole('button', { name: '删除' }));
    expect(screen.getByRole('dialog')).toBeTruthy();
    expect(screen.getByText('当前连接仍被以下创作方案引用：')).toBeTruthy();
    expect(screen.getByText('长篇方案')).toBeTruthy();
    expect(deleteProvider).not.toHaveBeenCalled();
  });

  it('确认后删除未被方案引用的连接', async () => {
    applicationBootstrap.mockResolvedValue({
      preferences: { theme: 'light', defaultCreativeProfileId: '' }, recentProjects: [],
      providers: [{ id: 'provider-1', name: '临时连接', kind: 'openai', baseUrl: 'https://api.openai.com/v1', hasCredential: true, createdAt: '', updatedAt: '' }],
      profiles: [],
    });
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AppProvider><Settings /></AppProvider>
      </MemoryRouter>,
    );

    await user.click(await screen.findByRole('button', { name: '删除' }));
    expect(screen.getByRole('dialog')).toBeTruthy();
    await user.click(screen.getByRole('button', { name: '确认删除' }));
    expect(deleteProvider).toHaveBeenCalledWith('provider-1');
  });
});
