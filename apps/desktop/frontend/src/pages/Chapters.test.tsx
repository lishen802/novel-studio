import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Chapters from './Chapters';
import { AppProvider } from '../store/AppContext';
import { AppContext, type AppCtxValue } from '../store/appContextValue';
import { defaultSettings } from '../store/defaultSettings';

describe('正文编辑器模型提示', () => {
  beforeEach(() => {
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: vi.fn().mockResolvedValue({ preferences: {}, recentProjects: [], providers: [], profiles: [] }),
      ListChapters: vi.fn().mockResolvedValue([{ id: 'chapter-1', projectId: 'project-1', title: '第一章', position: 1, body: '', revision: 1, updatedAt: '', hasRecoveryDraft: false }]),
      OpenChapter: vi.fn().mockResolvedValue({ id: 'chapter-1', projectId: 'project-1', title: '第一章', position: 1, body: '', revision: 1, updatedAt: '', hasRecoveryDraft: false }),
    } } };
  });
  afterEach(() => {
    cleanup();
    delete window.go;
  });

  it('保持暖灰提示底色，不随正文画布切换', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AppProvider>
          <Chapters />
        </AppProvider>
      </MemoryRouter>,
    );

    const notice = await screen.findByTestId('model-configuration-notice');
    expect(notice.className).toContain('bg-model-notice');
    expect(notice.className).toContain('border-model-notice-border');
    expect(notice.className).not.toContain('bg-warning/5');

    await user.click(screen.getByRole('button', { name: '排版' }));
    await user.click(screen.getByTitle('深夜'));

    expect(notice.className).toContain('bg-model-notice');
    expect(notice.className).toContain('border-model-notice-border');
    expect(notice.className).not.toContain('bg-warning/5');
  });
});

describe('正文编辑器真实模型运行确认', () => {
  const chapter = { id: 'chapter-1', projectId: 'project-1', title: '第一章', position: 1, body: '', revision: 1, updatedAt: '', hasRecoveryDraft: false };

  function contextValue(): AppCtxValue {
    return {
      state: {
        currentProject: null,
        backendProject: {
          id: 'project-1',
          title: '测试作品',
          genre: '悬疑',
          summary: '',
          path: '/books/test.zjproject',
          accessMode: 'editable',
          profileSelection: { profileId: 'profile-1', version: 1, roleOverrides: {} },
          workbench: { stage: 'writing', canvas: '', aiPanel: '', pendingReviewCount: 0, recoverableJobCount: 0, archiveStatus: '' },
        },
        bootstrap: {
          preferences: { theme: 'light', defaultCreativeProfileId: 'profile-1', updatedAt: '' },
          recentProjects: [],
          providers: [{ id: 'provider-1', name: '主模型', kind: 'openai', baseUrl: 'https://example.com/v1', hasCredential: true, createdAt: '', updatedAt: '' }],
          profiles: [],
        },
        desktopAvailable: true,
        busy: false,
        error: null,
        settings: defaultSettings,
        aiPanelOpen: false,
        importStatus: 'idle',
        importProgress: 0,
      },
      dispatch: vi.fn(),
      loadSettings: vi.fn(),
      saveSettings: vi.fn(),
      refreshBootstrap: vi.fn(),
      createProject: vi.fn(),
      openProject: vi.fn(),
      openRecentProject: vi.fn(),
      closeProject: vi.fn(),
      registerBeforeClose: vi.fn(() => vi.fn()),
      requestApplicationClose: vi.fn(),
    };
  }

  function renderConfiguredEditor(getProfileVersion: (...args: unknown[]) => Promise<unknown>) {
    const createChapterAgentRun = vi.fn((..._args: unknown[]) => Promise.resolve(undefined));
    window.go = { main: { DesktopAPI: {
      ListChapters: vi.fn().mockResolvedValue([chapter]),
      OpenChapter: vi.fn().mockResolvedValue(chapter),
      AgentRuns: vi.fn().mockResolvedValue([]),
      GetProfileVersion: getProfileVersion,
      CreateChapterAgentRun: createChapterAgentRun,
    } } };
    render(
      <MemoryRouter>
        <AppContext.Provider value={contextValue()}>
          <Chapters />
        </AppContext.Provider>
      </MemoryRouter>,
    );
    return createChapterAgentRun;
  }

  async function openRunConfirmation(user: ReturnType<typeof userEvent.setup>) {
    await user.click(await screen.findByRole('button', { name: /唤起创作助手/ }));
    await user.type(screen.getByPlaceholderText('例如：让沈知秋在栈桥上发现一个隐藏记号…'), '续写这一章');
    await user.click(screen.getByRole('button', { name: '发送给 AI' }));
    await screen.findByText('确认运行创作任务');
    return screen.getByRole('dialog');
  }

  afterEach(() => {
    cleanup();
    delete window.go;
  });

  it('精确方案预览加载期间禁止确认运行', async () => {
    const user = userEvent.setup();
    const createChapterAgentRun = renderConfiguredEditor(
      vi.fn((..._args: unknown[]) => new Promise<unknown>(() => undefined)),
    );

    await openRunConfirmation(user);

    expect((screen.getByRole('button', { name: '确认运行' }) as HTMLButtonElement).disabled).toBe(true);
    expect(createChapterAgentRun).not.toHaveBeenCalled();
  });

  it('精确方案预览失败后禁止运行并允许重试', async () => {
    const user = userEvent.setup();
    const getProfileVersion = vi.fn((..._args: unknown[]) => Promise.reject(new Error('load failed')));
    const createChapterAgentRun = renderConfiguredEditor(getProfileVersion);

    await openRunConfirmation(user);

    expect((await screen.findByText(/已阻止真实模型调用/)).textContent).toContain('已阻止真实模型调用');
    expect((screen.getByRole('button', { name: '确认运行' }) as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByRole('button', { name: '重试加载' }));
    expect(getProfileVersion).toHaveBeenCalledTimes(2);
    expect(createChapterAgentRun).not.toHaveBeenCalled();
  });

  it('精确方案预览成功后才允许确认运行', async () => {
    const user = userEvent.setup();
    renderConfiguredEditor(vi.fn((..._args: unknown[]) => Promise.resolve({
      profileId: 'profile-1',
      version: 1,
      name: '正式创作方案',
      roles: [
        { role: 'planner', connectionId: 'provider-1', model: 'model-a', reasoningEffort: 'high', outputLimitMode: 'profile_budget', temperature: 0.7, enabled: true },
        { role: 'writer', connectionId: 'provider-1', model: 'model-b', reasoningEffort: 'medium', outputLimitMode: 'profile_budget', temperature: 0.8, enabled: true },
        { role: 'continuity_reviewer', connectionId: 'provider-1', model: 'model-c', reasoningEffort: 'low', outputLimitMode: 'profile_budget', temperature: 0.3, enabled: true },
      ],
      maxRunTokens: 10000000,
      maxContextChars: 20000000,
      createdAt: '',
    })));

    await openRunConfirmation(user);

    expect(await screen.findByText('正式创作方案')).toBeTruthy();
    expect(screen.getByText('高')).toBeTruthy();
    expect(screen.getByText('中')).toBeTruthy();
    expect(screen.getByText('低')).toBeTruthy();
    expect((screen.getByRole('button', { name: '确认运行' }) as HTMLButtonElement).disabled).toBe(false);
  });
  it('重新生成先展示当前预算，取消不会调用重试接口', async () => {
    const user=userEvent.setup();
    renderConfiguredEditor(vi.fn().mockResolvedValue({profileId:'profile-1',version:1,name:'当前方案',roles:[],maxRunTokens:12000,maxContextChars:40000,createdAt:''}));
    const retry=vi.fn();
    const failed={id:'failed-run',chapterId:'chapter-1',prompt:'原写作目标',status:'recoverable',steps:[],errorCode:'interrupted',errorMessage:'中断',inputTokens:0,outputTokens:0};
    Object.assign(window.go!.main!.DesktopAPI!,{AgentRuns:vi.fn().mockResolvedValue([failed]),AgentRun:vi.fn().mockResolvedValue(failed),RetryAgentRun:retry,JobSnapshot:vi.fn().mockResolvedValue({status:'recoverable',maxSequence:0}),JobEvents:vi.fn().mockResolvedValue([])});
    await user.click(await screen.findByRole('button',{name:/唤起创作助手/}));
    await user.click(await screen.findByRole('button',{name:'重新生成'}));
    expect(await screen.findByText('当前方案')).toBeTruthy();
    expect(screen.getByText(/重新执行规划、写作和审阅步骤/)).toBeTruthy();expect(retry).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button',{name:/^取消$/}));expect(retry).not.toHaveBeenCalled();
  });

});
