import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Cockpit from './Cockpit';
import ArchiveLayout from './archive/ArchiveLayout';
import { AppProvider } from '../store/AppContext';
import { AppContext, type AppCtxValue } from '../store/appContextValue';
import { defaultSettings } from '../store/defaultSettings';
import type { CreationWorkbenchView, StructureCollectionView, StructureEditCommand } from '../api/studio';
import type { RevisionCard } from '../components/creation/RevisionPanel';

// 结构增删命令的测试替身类型：断言点集中在命令形状上。
type SaveStructure = (command: StructureEditCommand) => Promise<CreationWorkbenchView>;
// 部分采用命令的测试替身类型：断言点集中在命令携带的差异路径上。
type ApplyRevision = (command: { actionId: string; argumentsHash: string; idempotencyKey: string; paths?: string[] }) => Promise<CreationWorkbenchView>;

function workbench(overrides: Partial<CreationWorkbenchView> = {}): CreationWorkbenchView {
  return {
    sessionId: 'session-1',
    projectId: 'project-1',
    branchId: 'main',
    stage: 'intent',
    advanceMode: 'step_review',
    blockingReason: '',
    recoverable: false,
    archiveConfirmed: false,
    intentText: '',
    stageRail: [
      { id: 'intent', label: '灵感', status: 'current' },
      { id: 'directions', label: '故事方向', status: 'upcoming' },
    ],
    directionSet: { candidateId: '', version: 0, selectedId: '', status: '', cards: [] },
    proposal: { candidateId: '', version: 0, status: '', title: '', sections: [] },
    peopleWorld: { candidateId: '', version: 0, status: '', entities: [] },
    roadmap: {
      candidateId: '', version: 0, status: '',
      book: { id: '', title: '', summary: '', position: 0, accepted: false, rejected: false },
      volumes: [], arcs: [], chapters: [],
    },
    currentRun: { runId: '', status: '', cancellable: false, publicStage: '' },
    assistant: { pendingActionCount: 0 },
    impact: {
      items: [], touchesLocked: false, touchesConfirmed: false, canAdvance: true,
      regenerateStages: [], estimatedCalls: 0, evidence: '',
    },
    capabilities: [],
    structure: [],
    review: { candidateId: '', stage: '', reviewVersion: 0, accepted: [], rejected: [], unreviewed: [], complete: false, updatedAt: '' },
    reviewCursor: { stage: '', candidateId: '', entityId: '', sectionId: '', comparisonRef: '', viewVersion: 0, updatedAt: '' },
    candidateVersions: [],
    ...overrides,
  };
}

function contextValue(options: { archiveStatus?: string; hasCredential?: boolean; projectId?: string } = {}): AppCtxValue {
  return {
    state: {
      currentProject: null,
      backendProject: {
        id: options.projectId ?? 'project-1',
        title: '潮汐关',
        genre: '悬疑',
        summary: '',
        path: '/books/test.zjproject',
        accessMode: 'editable',
        profileSelection: { profileId: 'profile-1', version: 1, roleOverrides: {} },
        workbench: {
          stage: 'idea', canvas: '', aiPanel: '', pendingReviewCount: 0, recoverableJobCount: 0,
          archiveStatus: options.archiveStatus ?? '',
        },
      },
      bootstrap: {
        preferences: { theme: 'light', defaultCreativeProfileId: '', updatedAt: '' },
        recentProjects: [],
        providers: options.hasCredential
          ? [{ id: 'provider-1', name: '主模型', kind: 'openai', baseUrl: '', hasCredential: true, createdAt: '', updatedAt: '' }]
          : [],
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

describe('创作驾驶舱', () => {
  const recoverCreation = vi.fn();
  const saveCreationIntent = vi.fn();
  const previewCreationGenerate = vi.fn();
  const confirmCreationGenerate = vi.fn();
  const cancelCreationRun = vi.fn();
  const creationWorkbench = vi.fn();

  beforeEach(() => {
    recoverCreation.mockResolvedValue(workbench());
    saveCreationIntent.mockImplementation(async (text: string) => workbench({ intentText: text }));
    previewCreationGenerate.mockResolvedValue({
      stage: 'directions', workflowId: 'story.directions', estimatedCalls: 2,
      maxRunTokens: 20000, runBudgetUnlimited: false, remainingStages: [], confirmationDigest: 'digest-1',
    });
    confirmCreationGenerate.mockResolvedValue({ runId: 'run-1' });
    cancelCreationRun.mockResolvedValue(undefined);
    creationWorkbench.mockResolvedValue(workbench());
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: vi.fn().mockResolvedValue({ preferences: {}, recentProjects: [], providers: [], profiles: [] }),
      RecoverCreation: recoverCreation,
      CreationWorkbench: creationWorkbench,
      SaveCreationIntent: saveCreationIntent,
      PreviewCreationGenerate: previewCreationGenerate,
      ConfirmCreationGenerate: confirmCreationGenerate,
      CancelCreationRun: cancelCreationRun,
    } } };
  });

  afterEach(() => {
    cleanup();
    delete window.go;
    vi.clearAllMocks();
  });

  it('无模型时不展示示例方向，并给出真实未配置说明', async () => {
    render(
      <MemoryRouter>
        <AppProvider>
          <Cockpit />
        </AppProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId('model-missing-notice')).toBeTruthy();
    expect(await screen.findByText(/还没有方向候选/)).toBeTruthy();
    expect(screen.queryByText('一座雾中灯塔')).toBeNull();
    expect(screen.queryByText('灯塔守望者')).toBeNull();
    expect(screen.getByRole('button', { name: '前往配置' })).toBeTruthy();
  });

  it('下游过期时局部重算先读取权威范围并预览费用，不直接启动模型', async () => {
    const user = userEvent.setup();
    const impact = {
      items: [{ candidateId: 'proposal-old', stage: 'proposal', severity: 'maybe_affected', reason: '方向已变化', locked: false, confirmed: false, excepted: false }],
      touchesLocked: false, touchesConfirmed: false, canAdvance: false,
      regenerateStages: ['proposal'], estimatedCalls: 2, evidence: '方向已变化',
    };
    const view = workbench({ stage: 'proposal', impact });
    recoverCreation.mockResolvedValue(view);
    const resolve = vi.fn().mockResolvedValue({ projection: impact, regenerateStages: ['proposal'] });
    Object.assign(window.go!.main!.DesktopAPI!, { ResolveCreationImpact: resolve });
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);

    await user.click(await screen.findByRole('button', { name: '局部重算' }));
    expect(screen.getByLabelText('下游影响').textContent).toContain('预计 2 次模型调用');
    await waitFor(() => expect(resolve).toHaveBeenCalledWith('regenerate_local'));
    expect(previewCreationGenerate).toHaveBeenCalledWith('proposal');
    expect(await screen.findByText('确认生成费用')).toBeTruthy();
    expect(confirmCreationGenerate).not.toHaveBeenCalled();
  });

  it('局部重算范围在点击后消失时不请求模型费用预览', async () => {
    const user = userEvent.setup();
    const impact = {
      items: [{ candidateId: 'proposal-old', stage: 'proposal', severity: 'maybe_affected', reason: '方向已变化', locked: false, confirmed: false, excepted: false }],
      touchesLocked: false, touchesConfirmed: false, canAdvance: false,
      regenerateStages: ['proposal'], estimatedCalls: 2, evidence: '方向已变化',
    };
    recoverCreation.mockResolvedValue(workbench({ stage: 'proposal', impact }));
    const resolve = vi.fn().mockResolvedValue({ projection: { ...impact, regenerateStages: [] }, regenerateStages: [] });
    Object.assign(window.go!.main!.DesktopAPI!, { ResolveCreationImpact: resolve });
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);

    await user.click(await screen.findByRole('button', { name: '局部重算' }));
    expect(await screen.findByText('受影响候选已变化，请刷新后重新核对。')).toBeTruthy();
    expect(previewCreationGenerate).not.toHaveBeenCalled();
    expect(confirmCreationGenerate).not.toHaveBeenCalled();
  });

  it('处理下游影响后不覆盖尚未保存的创作输入', async () => {
    const user = userEvent.setup();
    const impact = {
      items: [{ candidateId: 'old', stage: 'proposal', severity: 'maybe_affected', reason: '方向已变化', locked: false, confirmed: false, excepted: false }],
      touchesLocked: false, touchesConfirmed: false, canAdvance: false,
      regenerateStages: ['proposal'], estimatedCalls: 1, evidence: '方向已变化',
    };
    recoverCreation.mockResolvedValue(workbench({ intentText: '原意图', impact }));
    creationWorkbench.mockResolvedValue(workbench({ intentText: '原意图', impact: { ...impact, items: [] } }));
    const resolve = vi.fn().mockResolvedValue({ projection: { ...impact, items: [] }, regenerateStages: [] });
    Object.assign(window.go!.main!.DesktopAPI!, { ResolveCreationImpact: resolve });
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);

    const input = await screen.findByLabelText('创作输入') as HTMLTextAreaElement;
    await waitFor(() => expect(input.value).toBe('原意图'));
    await user.type(input, '与新想法');
    await user.click(screen.getByRole('button', { name: '保留为例外' }));
    await waitFor(() => expect(resolve).toHaveBeenCalledWith('keep_exception'));
    await waitFor(() => expect(screen.queryByRole('button', { name: '保留为例外' })).toBeNull());
    expect(input.value).toBe('原意图与新想法');
  });

  it('切换作品后忽略前一作品迟到的操作投影', async () => {
    const user = userEvent.setup();
    const impact = {
      items: [{ candidateId: 'old', stage: 'proposal', severity: 'maybe_affected', reason: '方向已变化', locked: false, confirmed: false, excepted: false }],
      touchesLocked: false, touchesConfirmed: false, canAdvance: false,
      regenerateStages: ['proposal'], estimatedCalls: 1, evidence: '方向已变化',
    };
    const first = workbench({ intentText: '旧作品构思', impact });
    const second = workbench({ projectId: 'project-2', sessionId: 'session-2', intentText: '新作品构思' });
    recoverCreation.mockResolvedValueOnce(first).mockResolvedValueOnce(second);
    let releaseOld: (value: CreationWorkbenchView) => void = () => undefined;
    creationWorkbench.mockReturnValueOnce(new Promise<CreationWorkbenchView>(resolve => { releaseOld = resolve; }));
    const resolve = vi.fn().mockResolvedValue({ projection: { ...impact, items: [] }, regenerateStages: [] });
    Object.assign(window.go!.main!.DesktopAPI!, { ResolveCreationImpact: resolve });
    const renderPage = (projectId: string) => <MemoryRouter><AppContext.Provider value={contextValue({ projectId })}><Cockpit /></AppContext.Provider></MemoryRouter>;
    const page = render(renderPage('project-1'));
    await user.click(await screen.findByRole('button', { name: '保留为例外' }));
    await waitFor(() => expect(creationWorkbench).toHaveBeenCalledTimes(1));

    page.rerender(renderPage('project-2'));
    await waitFor(() => expect((screen.getByLabelText('创作输入') as HTMLTextAreaElement).value).toBe('新作品构思'));
    await act(async () => { releaseOld(workbench({ ...first, intentText: '旧作品更新' })); });
    expect((screen.getByLabelText('创作输入') as HTMLTextAreaElement).value).toBe('新作品构思');
  });

  it('自动推进显示授权范围与预算，模式切换不直接启动付费运行', async () => {
    const user = userEvent.setup();
    recoverCreation.mockResolvedValue(workbench({ stage: 'proposal', advanceMode: 'step_review' }));
    const setMode = vi.fn().mockResolvedValue(workbench({ stage: 'proposal', advanceMode: 'auto_advance' }));
    previewCreationGenerate.mockResolvedValue({
      stage: 'proposal', workflowId: 'story.foundation', estimatedCalls: 7,
      maxRunTokens: 30000, runBudgetUnlimited: false,
      remainingStages: ['people_world', 'roadmap'], confirmationDigest: 'auto-digest',
    });
    Object.assign(window.go!.main!.DesktopAPI!, { SetCreationAdvanceMode: setMode });
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);

    await user.click(await screen.findByRole('button', { name: '自动推进' }));
    expect(setMode).toHaveBeenCalledWith('auto_advance');
    expect(confirmCreationGenerate).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: '重新生成' }));
    expect(await screen.findByText(/作品方案 → 人物与世界 → 全书路线图/)).toBeTruthy();
    expect(screen.getByText(/预计调用 7 次/)).toBeTruthy();
    expect(screen.getByText(/30000/)).toBeTruthy();
    expect(confirmCreationGenerate).not.toHaveBeenCalled();
  });

  it('已确认档案的作品不再显示创意构建中', async () => {
    recoverCreation.mockResolvedValue(workbench({ archiveConfirmed: true, stage: 'archive_confirm' }));
    render(<MemoryRouter><AppContext.Provider value={contextValue({ archiveStatus: 'confirmed' })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    expect(await screen.findByText('作品档案已确认')).toBeTruthy();
    expect(screen.queryByText('创意构建中')).toBeNull();
  });

  it('选择第二张卡时把候选 ID 和卡 ID 一起传给后端', async () => {
    const user = userEvent.setup();
    const view = workbench({ stage: 'directions', directionSet: {
      candidateId: 'set-1', version: 1, selectedId: '', status: 'draft',
      cards: ['d1', 'd2', 'd3'].map((id) => ({ id, hook: id, genre: '悬疑', protagonist: '主角',
        coreConflict: '冲突', growthCost: '代价', pacing: '节奏', ending: '结局', selected: false })),
    } });
    recoverCreation.mockResolvedValue(view);
    const select = vi.fn().mockResolvedValue(view);
    const advance = vi.fn().mockResolvedValue(workbench({ stage: 'proposal' }));
    Object.assign(window.go!.main!.DesktopAPI!, { SelectCreationDirection: select, AdvanceCreationStage: advance });
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    const buttons = await screen.findAllByRole('button', { name: '选择此方向' });
    await user.click(buttons[1]);
    expect(select).toHaveBeenCalledWith('set-1', 'd2');
    expect(advance).toHaveBeenCalledTimes(1);
  });

  it('方案阶段先接受精确候选，再进入下一阶段', async () => {
	const user = userEvent.setup();
	const view = workbench({
	  stage: 'proposal',
	  proposal: { candidateId: 'proposal-2', version: 2, status: 'draft', title: '潮汐关', sections: [] },
	});
	recoverCreation.mockResolvedValue(view);
	const accept = vi.fn().mockResolvedValue(workbench({
	  ...view,
	  proposal: { ...view.proposal, status: 'confirmed' },
	}));
	const advance = vi.fn().mockResolvedValue(workbench({ stage: 'people_world' }));
	Object.assign(window.go!.main!.DesktopAPI!, { AcceptCreationCandidate: accept, AdvanceCreationStage: advance });

	render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
	await user.click(await screen.findByRole('button', { name: /接受并进入下一步/ }));
	expect(accept).toHaveBeenCalledWith('proposal-2');
	expect(advance).toHaveBeenCalledTimes(1);
  });

  it('异步运行启动后继续刷新直到后端结束，不把请求返回当完成', async () => {
    const user = userEvent.setup();
    const running = workbench({ currentRun: { runId: 'run-1', status: 'queued', cancellable: true, publicStage: 'context_preparing' } });
    creationWorkbench.mockResolvedValue(running);
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await screen.findByLabelText('创作输入');
    await user.type(screen.getByLabelText('创作输入'), '海上灯塔');
    await user.click(screen.getByRole('button', { name: /发送给 AI 团队/ }));
    await user.click(await screen.findByRole('button', { name: '确认生成' }));
    await screen.findByText('准备开始创作');
    expect(screen.queryByText(/context_preparing/)).toBeNull();
    expect((screen.getByRole('button', { name: /发送给 AI 团队/ }) as HTMLButtonElement).disabled).toBe(true);
    creationWorkbench.mockResolvedValue(workbench({ currentRun: { runId: 'run-1', status: 'completed', cancellable: false, publicStage: 'candidate_ready' } }));
    await waitFor(() => expect(screen.queryByRole('button', { name: '取消运行' })).toBeNull(), { timeout: 3000 });
    expect(confirmCreationGenerate).toHaveBeenCalledTimes(1);
  });

  it('生成中禁用重复提交，并允许取消运行', async () => {
    const user = userEvent.setup();
    let release: () => void = () => undefined;
    confirmCreationGenerate.mockImplementation(() => new Promise((resolve) => {
      release = () => resolve({ runId: 'run-1' });
    }));

    render(
      <MemoryRouter>
        <AppContext.Provider value={contextValue({ hasCredential: true })}>
          <Cockpit />
        </AppContext.Provider>
      </MemoryRouter>,
    );

    await screen.findByLabelText('创作输入');
    await waitFor(() => expect(recoverCreation).toHaveBeenCalled());
    expect(screen.queryByTestId('model-missing-notice')).toBeNull();
    await user.type(screen.getByLabelText('创作输入'), '守关人必须放走仇敌');
    await user.click(screen.getByRole('button', { name: /发送给 AI 团队/ }));
    expect(await screen.findByText(/预计调用 2 次/)).toBeTruthy();
    await user.click(screen.getByRole('button', { name: '确认生成' }));
    expect(await screen.findByTestId('run-panel')).toBeTruthy();
    expect((screen.getByRole('button', { name: /发送给 AI 团队/ }) as HTMLButtonElement).disabled).toBe(true);
    await user.click(screen.getByRole('button', { name: '取消运行' }));
    expect(cancelCreationRun).toHaveBeenCalled();
    expect(confirmCreationGenerate).toHaveBeenCalledTimes(1);
    release();
  });

  it('生成失败时展示后端错误，而不是示例成功', async () => {
    const user = userEvent.setup();
    confirmCreationGenerate.mockRejectedValue(new Error('PROVIDER_CREDENTIAL_MISSING|API Key 未保存或系统凭据库不可用，请重新输入。'));

    render(
      <MemoryRouter>
        <AppContext.Provider value={contextValue({ hasCredential: true })}>
          <Cockpit />
        </AppContext.Provider>
      </MemoryRouter>,
    );

    await screen.findByLabelText('创作输入');
    await user.type(screen.getByLabelText('创作输入'), '守关人必须放走仇敌');
    await user.click(screen.getByRole('button', { name: /发送给 AI 团队/ }));
    await user.click(await screen.findByRole('button', { name: '确认生成' }));
    expect((await screen.findByRole('alert')).textContent).toContain('API Key 未保存');
    expect(screen.queryByTestId('run-panel')).toBeNull();
    expect(screen.queryByText('一座雾中灯塔')).toBeNull();
  });

  it('恢复时展示暂停说明，且不会自动开始付费生成', async () => {
    recoverCreation.mockResolvedValue(workbench({
      recoverable: true,
      blockingReason: 'cost_authorization',
      currentRun: { runId: 'run-stale', status: '', cancellable: true, publicStage: 'story_planning' },
    }));

    render(
      <MemoryRouter>
        <AppProvider>
          <Cockpit />
        </AppProvider>
      </MemoryRouter>,
    );

    expect(await screen.findByTestId('recovery-notice')).toBeTruthy();
    expect(recoverCreation).toHaveBeenCalled();
    expect(confirmCreationGenerate).not.toHaveBeenCalled();
    expect(previewCreationGenerate).not.toHaveBeenCalled();
  });

  it('后端灵感阶段零值 fieldEditor 不会把驾驶舱打成白屏', async () => {
    recoverCreation.mockResolvedValue(JSON.parse(JSON.stringify({
      ...workbench(),
      fieldEditor: { candidateId: '', version: 0, fields: null },
      revisions: null,
      assistant: { pendingActionCount: 0, messages: null, authorizations: null },
    })));

    render(
      <MemoryRouter>
        <AppContext.Provider value={contextValue({ hasCredential: true })}>
          <Cockpit />
        </AppContext.Provider>
      </MemoryRouter>,
    );

    expect(await screen.findByText('作品构建工作台')).toBeTruthy();
    expect(screen.getByLabelText('AI 共创对话')).toBeTruthy();
    expect(screen.getByRole('button', { name: '确认进入下一步' })).toBeTruthy();
  });
});

describe('档案页确认入口', () => {
  afterEach(() => {
    cleanup();
  });

  it('档案未确认时提示并提供返回驾驶舱入口', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter initialEntries={['/archive/outline']}>
        <AppContext.Provider value={contextValue({ archiveStatus: '' })}>
          <Routes>
            <Route path="/archive" element={<ArchiveLayout />}>
              <Route path="outline" element={<p>正式大纲</p>} />
            </Route>
            <Route path="/cockpit" element={<p>驾驶舱</p>} />
          </Routes>
        </AppContext.Provider>
      </MemoryRouter>,
    );

    expect(screen.getByTestId('archive-unconfirmed')).toBeTruthy();
    expect(screen.getByText('作品档案尚未确认')).toBeTruthy();
    expect(screen.getByText('正式大纲')).toBeTruthy();
    await user.click(screen.getByRole('button', { name: '返回驾驶舱' }));
    expect(screen.getByText('驾驶舱')).toBeTruthy();
  });

  it('档案确认后展示正式档案导航', () => {
    render(
      <MemoryRouter initialEntries={['/archive/outline']}>
        <AppContext.Provider value={contextValue({ archiveStatus: 'confirmed' })}>
          <Routes>
            <Route path="/archive" element={<ArchiveLayout />}>
              <Route path="outline" element={<p>正式大纲</p>} />
            </Route>
          </Routes>
        </AppContext.Provider>
      </MemoryRouter>,
    );

    expect(screen.queryByTestId('archive-unconfirmed')).toBeNull();
    expect(screen.getByText('作品档案')).toBeTruthy();
    expect(screen.getByText('正式大纲')).toBeTruthy();
  });
});

describe('已有方案工作台', () => {
  afterEach(() => { cleanup(); delete window.go; });
  it('解析使用已有方案阶段，明确跳过方向且只在确认费用后调用', async () => {
    const user = userEvent.setup();
    const view = workbench({ contentVersion: 2, runSlotVersion: 0, entry: { id: 'entry-1', version: 1, kind: 'author_plan', material: '作者自己的结局' }, stageRail: [{ id: 'intent', label: '已有方案', status: 'current' }, { id: 'directions', label: '故事方向', status: 'not_applicable' }] });
    const preview = vi.fn().mockResolvedValue({ stage: 'intent', workflowId: 'story.author_material', workflowVersion: 1, estimatedCalls: 2, maxRunTokens: 30000, remainingStages: [], confirmationDigest: 'source-bound' });
    const confirm = vi.fn().mockResolvedValue({ runId: 'material-run' });
    window.go = { main: { DesktopAPI: { RecoverCreation: vi.fn().mockResolvedValue(view), CreationWorkbench: vi.fn().mockResolvedValue(view), PreviewCreationGenerate: preview, ConfirmCreationGenerate: confirm } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '解析已有方案与检查缺口' }));
    expect(preview).toHaveBeenCalledWith('intent');
    expect(confirm).not.toHaveBeenCalled();
    expect(screen.queryByText('故事方向候选')).toBeNull();
    await user.click(await screen.findByRole('button', { name: '确认生成' }));
    await waitFor(() => expect(confirm).toHaveBeenCalledWith(expect.objectContaining({ stage: 'intent', confirmationDigest: 'source-bound' })));
  });

  it('路线图阶段展示故事弧规划目标', async () => {
    const view = workbench({
      stage: 'roadmap',
      peopleWorld: {candidateId:'old-people',version:1,status:'draft',entities:[{id:'character:old',kind:'character',title:'上阶段人物',summary:'旧人物',complete:true,accepted:false,rejected:false}]},
      fieldEditor: {candidateId:'roadmap-1',version:1,fields:[{path:'/arcs/@arc-1/summary',ownerRef:'arc:arc-1',label:'概要',group:'开关',value:'本弧内容',locked:false},{path:'/near_term_chapters/@空船/summary',ownerRef:'chapter:空船',label:'概要',group:'空船',value:'本章内容',locked:false}]},
      roadmap: {
        candidateId: 'roadmap-1', version: 1, status: 'draft',
        book: { id: 'book', title: '潮汐关', summary: '守关人必须选择', position: 1, accepted: false, rejected: false },
        volumes: [{ id: 'volume:第一卷', parentId:'book', title: '第一卷', summary: '建立关隘危机', position: 1, accepted: false, rejected: false }],
        arcs: [{
          id: 'arc:arc-1', parentId:'volume:第一卷', title: '开关', summary: '敌船靠岸导致必须抉择', position: 1,
          readerExpectation: '读者应感到守关人将被亲情撕裂',
          stagePayoff: '开城救人并失去军职',
          irreversibleChange: '阿绫不再以军令为最高准则',
          endingContribution: '为终局选择救人立下先例',
          commitments: [{ id: 'cmt-1', statement: '旧令牌兑现身份', detail: '本弧末章' }],
          retainedQuestions: [{ id: 'q-1', statement: '潮汐神谕是否被篡改', detail: '留给后续卷揭示' }],
          accepted: false, rejected: false,
        }],
        chapters: [{ id: 'chapter:空船', parentId:'arc:arc-1', title: '空船', summary: '发现异常', position: 1, accepted: false, rejected: false }],
      },
    });
    window.go = { main: { DesktopAPI: { RecoverCreation: vi.fn().mockResolvedValue(view), CreationWorkbench: vi.fn().mockResolvedValue(view) } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    expect(await screen.findByRole('button', { name: '开关' })).toBeTruthy();
    expect(screen.getByText('读者应感到守关人将被亲情撕裂')).toBeTruthy();
    expect(screen.getByText('待兑现承诺')).toBeTruthy();
    expect(screen.getByText('旧令牌兑现身份')).toBeTruthy();
    expect(screen.getByText('潮汐神谕是否被篡改')).toBeTruthy();
    expect(screen.getByText('当前对象：开关')).toBeTruthy();
    expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('本弧内容');
    await userEvent.click(screen.getByRole('button',{name:/近章 1 · 空船/}));
    expect(screen.getByText('当前对象：空船')).toBeTruthy();
    expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('本章内容');
    expect(Array.from((screen.getByLabelText('共创目标字段') as HTMLSelectElement).options).some(option => option.value === '/arcs/@arc-1/summary')).toBe(false);
  });
});

describe('试写准备工作台', () => {
 afterEach(()=>{cleanup();delete window.go;});
 it('只保存准备，不调用模型或推进阶段', async()=>{
  const user=userEvent.setup();
  const view=workbench({stage:'directions',canPrepareSample:true,contentVersion:4,runSlotVersion:1,directionSet:{candidateId:'c1',version:1,selectedId:'d1',status:'draft',cards:[]}});
  const save=vi.fn().mockResolvedValue({...view,contentVersion:5,sampleHandoff:{id:'sample',status:'prepared',directionCandidateId:'c1',directionCardId:'d1',pointOfView:'第三人称限知',tone:'自然、克制',constraints:[],targetRunes:1200}});
  const generate=vi.fn(),advance=vi.fn();
  window.go={main:{DesktopAPI:{RecoverCreation:vi.fn().mockResolvedValue(view),CreationWorkbench:vi.fn().mockResolvedValue(view),SaveCreationSample:save,ConfirmCreationGenerate:generate,AdvanceCreationStage:advance}}};
  render(<MemoryRouter><AppContext.Provider value={contextValue()}><Cockpit/></AppContext.Provider></MemoryRouter>);
  await user.click(await screen.findByRole('button',{name:'保存试写准备'}));
  await waitFor(()=>expect(save).toHaveBeenCalledWith(expect.objectContaining({projectId:'project-1',sessionId:'session-1',expectedVersion:4,expectedRunSlotVersion:1,directionCandidateId:'c1',directionCardId:'d1',targetRunes:1200})));
  expect(generate).not.toHaveBeenCalled();expect(advance).not.toHaveBeenCalled();
  expect((screen.getByRole('button',{name:'生成试写（尚未就绪）'}) as HTMLButtonElement).disabled).toBe(true);
 });
});

describe('作品策划逐项审阅', () => {
  const reviewCapabilities = ['SetReview', 'AcceptAllReview', 'UseCandidateVersion', 'CompareCandidateVersions', 'SaveReviewCursor'];
  const peopleEntities = [
    { id: 'character:char-1', kind: 'character', title: '阿绫', summary: '守关人的女儿', complete: true, accepted: false, rejected: false },
    { id: 'rule:潮汐律', kind: 'world_rule', title: '潮汐律', summary: '潮水每夜上涨', complete: true, accepted: false, rejected: false },
  ];
  const proposalVersions = [
    { candidateId: 'proposal-2', version: 2, kind: 'proposal', origin: 'author', status: 'draft', quality: 'passed', summary: '作者改写标题', createdAt: '2026-02-01T00:00:00Z', current: true },
    { candidateId: 'proposal-1', version: 1, kind: 'proposal', origin: 'model', status: 'superseded', quality: 'passed', summary: '模型初稿', createdAt: '2026-01-01T00:00:00Z', current: false },
  ];

  afterEach(() => { cleanup(); delete window.go; });

  it('接受人物世界实体时按候选、作用域与单位 ID 命令后端，并采用返回投影', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'people_world',
      capabilities: reviewCapabilities,
      peopleWorld: { candidateId: 'people-1', version: 1, status: 'draft', entities: peopleEntities },
      review: {
        candidateId: 'people-1', stage: 'people_world', reviewVersion: 1,
        accepted: [], rejected: [],
        unreviewed: [{ scope: 'entity', id: 'character:char-1' }, { scope: 'entity', id: 'rule:潮汐律' }],
        complete: false, updatedAt: '2026-01-01T00:00:00Z',
      },
    });
    const decided = workbench({
      ...base,
      peopleWorld: { ...base.peopleWorld, entities: peopleEntities.map((entity) => (entity.id === 'character:char-1' ? { ...entity, accepted: true } : entity)) },
      review: {
        ...base.review, reviewVersion: 2,
        accepted: [{ scope: 'entity', id: 'character:char-1' }],
        unreviewed: [{ scope: 'entity', id: 'rule:潮汐律' }],
      },
    });
    const setReview = vi.fn().mockResolvedValue(decided);
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      SetCreationReview: setReview,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '接受 阿绫' }));

    expect(setReview).toHaveBeenCalledWith({ candidateId: 'people-1', scope: 'entity', refId: 'character:char-1', decision: 'accept' });
    // 界面状态来自后端返回的投影，而不是本地猜测。
    await waitFor(() => expect((screen.getByRole('button', { name: '接受 阿绫' }) as HTMLButtonElement).disabled).toBe(true));
    expect(screen.getByText(/未审阅 1 项 · 已接受 1 项/)).toBeTruthy();
    expect((screen.getByRole('button', { name: '撤销决定 阿绫' }) as HTMLButtonElement).disabled).toBe(false);
  });

  it('全部接受按当前候选 ID 调用显式命令并刷新未审阅数量', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'people_world',
      capabilities: reviewCapabilities,
      peopleWorld: { candidateId: 'people-1', version: 1, status: 'draft', entities: peopleEntities },
      review: {
        candidateId: 'people-1', stage: 'people_world', reviewVersion: 1,
        accepted: [], rejected: [],
        unreviewed: [{ scope: 'entity', id: 'character:char-1' }, { scope: 'entity', id: 'rule:潮汐律' }],
        complete: false, updatedAt: '2026-01-01T00:00:00Z',
      },
    });
    const accepted = workbench({
      ...base,
      peopleWorld: { ...base.peopleWorld, entities: peopleEntities.map((entity) => ({ ...entity, accepted: true })) },
      review: {
        ...base.review, reviewVersion: 2, complete: true, unreviewed: [],
        accepted: [{ scope: 'entity', id: 'character:char-1' }, { scope: 'entity', id: 'rule:潮汐律' }],
      },
    });
    const acceptAll = vi.fn().mockResolvedValue(accepted);
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      AcceptAllCreationReview: acceptAll,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '全部接受（2 项）' }));

    expect(acceptAll).toHaveBeenCalledWith('people-1');
    expect(await screen.findByText(/未审阅 0 项 · 已接受 2 项/)).toBeTruthy();
  });

  it('非当前版本才提供用作工作基线，并按候选 ID 调用', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'proposal',
      capabilities: reviewCapabilities,
      proposal: { candidateId: 'proposal-2', version: 2, status: 'draft', title: '潮汐关', sections: [] },
      candidateVersions: proposalVersions,
    });
    const switched = workbench({
      ...base,
      proposal: { candidateId: 'proposal-1', version: 1, status: 'draft', title: '初稿', sections: [] },
      candidateVersions: [
        { ...proposalVersions[0], current: false, status: 'superseded' },
        { ...proposalVersions[1], current: true, status: 'draft' },
      ],
    });
    const useVersion = vi.fn().mockResolvedValue(switched);
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      UseCreationCandidateVersion: useVersion,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await screen.findByText('候选版本');

    expect(screen.queryByRole('button', { name: '用作工作基线 版本 2' })).toBeNull();
    await user.click(screen.getByRole('button', { name: '用作工作基线 版本 1' }));

    expect(useVersion).toHaveBeenCalledWith('proposal-1');
  });

  it('比较弹窗以当前工作版本为基线并只读渲染前后差异', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'proposal',
      capabilities: reviewCapabilities,
      proposal: { candidateId: 'proposal-2', version: 2, status: 'draft', title: '潮汐关', sections: [] },
      candidateVersions: proposalVersions,
    });
    const compare = vi.fn().mockResolvedValue({
      stage: 'proposal',
      baseCandidateId: 'proposal-2',
      versions: [
        { ...proposalVersions[0], diff: [] },
        { ...proposalVersions[1], diff: [{ path: '/title', label: '作品名', group: '基础设定', before: '雾港灯塔', after: '雾港灯塔·上' }] },
      ],
    });
    const useVersion = vi.fn();
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      CompareCreationCandidateVersions: compare,
      UseCreationCandidateVersion: useVersion,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '比较版本' }));

    expect(compare).toHaveBeenCalledWith('proposal', 'proposal-2');
    expect(await screen.findByText('基础设定')).toBeTruthy();
    expect(screen.getByText('作品名')).toBeTruthy();
    expect(screen.getByText('雾港灯塔')).toBeTruthy();
    expect(screen.getByText('雾港灯塔·上')).toBeTruthy();
    expect(screen.getByText('与基线一致')).toBeTruthy();
    expect(useVersion).not.toHaveBeenCalled();
  });

  it('选中实体时按实体 ID 保存审阅位置，保存失败只提示错误不打断选择', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'people_world',
      capabilities: reviewCapabilities,
      peopleWorld: { candidateId: 'people-1', version: 1, status: 'draft', entities: peopleEntities },
    });
    const saveCursor = vi.fn().mockRejectedValue(new Error('REVIEW_CURSOR_CONFLICT|审阅位置版本冲突，请重试。'));
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      SaveCreationReviewCursor: saveCursor,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '潮汐律' }));

    expect(saveCursor).toHaveBeenCalledWith({ stage: 'people_world', candidateId: 'people-1', entityId: 'rule:潮汐律', sectionId: '', comparisonRef: '' });
    expect((await screen.findByRole('alert')).textContent).toContain('审阅位置版本冲突');
    expect(screen.getByText('潮水每夜上涨')).toBeTruthy();
  });

  it('选中方案分节时按分节 ID 保存审阅位置', async () => {
    const user = userEvent.setup();
    const base = workbench({
      stage: 'proposal',
      capabilities: reviewCapabilities,
      proposal: {
        candidateId: 'proposal-2', version: 2, status: 'draft', title: '潮汐关',
        sections: [{ id: 'position', title: '定位', content: '悬疑 · 守关人视角', lockPath: '/positioning', locked: false, accepted: false, rejected: false }],
      },
    });
    const saveCursor = vi.fn().mockResolvedValue(base);
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
      SaveCreationReviewCursor: saveCursor,
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    await user.click(await screen.findByRole('button', { name: '选中分节 定位' }));

    expect(saveCursor).toHaveBeenCalledWith({ stage: 'proposal', candidateId: 'proposal-2', entityId: '', sectionId: 'position', comparisonRef: '' });
    expect(screen.getByRole('button', { name: '选中分节 定位' }).getAttribute('aria-current')).toBe('true');
  });

  it('加载工作台时恢复审阅游标指向的实体', async () => {
    const base = workbench({
      stage: 'people_world',
      capabilities: reviewCapabilities,
      peopleWorld: { candidateId: 'people-1', version: 1, status: 'draft', entities: peopleEntities },
      reviewCursor: {
        stage: 'people_world', candidateId: 'people-1', entityId: 'rule:潮汐律', sectionId: '',
        comparisonRef: '', viewVersion: 3, updatedAt: '2026-01-02T00:00:00Z',
      },
    });
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(base),
      CreationWorkbench: vi.fn().mockResolvedValue(base),
    } } };

    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);

    expect(await screen.findByText('潮水每夜上涨')).toBeTruthy();
    expect(screen.getByRole('button', { name: '潮汐律' }).getAttribute('aria-current')).toBe('true');
    expect(screen.queryByText('守关人的女儿')).toBeNull();
  });
});

describe('候选结构增删', () => {
  const capabilities = ['SetReview', 'SaveStructure'];
  const entities = [
    { id: 'character:char-1', kind: 'character', title: '阿绫', summary: '守关人的女儿', complete: true, accepted: false, rejected: false },
  ];
  const characterCollection: StructureCollectionView = {
    collection: 'characters', title: '人物', identityField: 'id',
    entities: [{ key: 'char-1', title: '阿绫' }, { key: 'char-2', title: '守塔人' }],
  };
  const relationshipCollection: StructureCollectionView = {
    collection: 'relationships', title: '人物关系', identityField: 'id',
    entities: [{ key: 'rel-1', title: '阿绫 → 守塔人' }],
  };

  const base = (overrides: Partial<CreationWorkbenchView> = {}) => workbench({
    stage: 'people_world',
    capabilities,
    structure: [characterCollection, relationshipCollection],
    peopleWorld: { candidateId: 'people-1', version: 1, status: 'draft', entities },
    ...overrides,
  });

  const mount = (view: CreationWorkbenchView, saveStructure: SaveStructure) => {
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(view),
      CreationWorkbench: vi.fn().mockResolvedValue(view),
      // 桌面桥按任意参数转发命令，测试里只需要按命令形状断言。
      SaveCreationStructure: saveStructure as unknown as (...args: unknown[]) => Promise<unknown>,
    } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
  };

  afterEach(() => { cleanup(); delete window.go; });

  it('删除实体时二次确认后按候选与标识调用命令，并采用返回投影刷新列表', async () => {
    const user = userEvent.setup();
    const saved = base({ structure: [{ ...characterCollection, entities: [{ key: 'char-2', title: '守塔人' }] }, relationshipCollection] });
    const saveStructure = vi.fn<SaveStructure>().mockResolvedValue(saved);
    mount(base(), saveStructure);

    await user.click(await screen.findByRole('button', { name: '删除 人物 阿绫' }));
    expect(screen.getByRole('dialog')).toBeTruthy();
    expect(saveStructure).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: '确认删除 人物 阿绫' }));

    await waitFor(() => expect(saveStructure).toHaveBeenCalledWith({
      candidateId: 'people-1', collection: 'characters', operation: 'remove', entityKey: 'char-1', value: '',
    }));
    // 列表来自后端返回的投影，本地不猜删除结果。
    await waitFor(() => expect(screen.queryByRole('button', { name: '删除 人物 阿绫' })).toBeNull());
    expect(screen.getByRole('button', { name: '删除 人物 守塔人' })).toBeTruthy();
  });

  it('新增实体时按集合标识与 JSON 字段调用命令，并用返回投影刷新列表', async () => {
    const user = userEvent.setup();
    const saved = base({
      structure: [
        { ...characterCollection, entities: [{ key: 'char-2', title: '守塔人' }, { key: 'char-3', title: '阿绫' }] },
        relationshipCollection,
      ],
    });
    const saveStructure = vi.fn<SaveStructure>().mockResolvedValue(saved);
    mount(base(), saveStructure);

    await user.click(await screen.findByRole('button', { name: '新增人物' }));
    await user.type(screen.getByLabelText('姓名 *'), '阿绫');
    await user.type(screen.getByLabelText('简介 *'), '守关人的女儿');
    await user.type(screen.getByLabelText('动机 *'), '找回失踪的父亲');
    await user.type(screen.getByLabelText('人物弧光 *'), '从被守护到守护他人');
    await user.selectOptions(screen.getByLabelText('角色定位 *'), 'protagonist');
    await user.click(screen.getByRole('button', { name: '提交新增人物' }));

    await waitFor(() => expect(saveStructure).toHaveBeenCalledTimes(1));
    const request = saveStructure.mock.calls[0][0];
    expect(request.candidateId).toBe('people-1');
    expect(request.collection).toBe('characters');
    expect(request.operation).toBe('add');
    expect(request.entityKey).toBe('char-3');
    expect(JSON.parse(request.value)).toEqual({
      id: 'char-3', name: '阿绫', role: 'protagonist',
      summary: '守关人的女儿', motivation: '找回失踪的父亲', arc: '从被守护到守护他人',
    });
    // 提交成功后表单收起，列表来自返回投影。
    expect(screen.queryByLabelText('角色定位 *')).toBeNull();
    expect(await screen.findByRole('button', { name: '删除 人物 阿绫' })).toBeTruthy();
  });

  it('必填字段缺失时本地拦截，不发请求并给出中文提示', async () => {
    const user = userEvent.setup();
    const saveStructure = vi.fn<SaveStructure>();
    mount(base(), saveStructure);

    await user.click(await screen.findByRole('button', { name: '新增人物' }));
    await user.type(screen.getByLabelText('姓名 *'), '阿绫');
    await user.click(screen.getByRole('button', { name: '提交新增人物' }));

    expect(saveStructure).not.toHaveBeenCalled();
    expect((await screen.findByRole('alert')).textContent).toContain('请先填写必填字段');
    expect((screen.getByLabelText('姓名 *') as HTMLInputElement).value).toBe('阿绫');
  });

  it('后端拒绝时展示错误信息并保留已填内容', async () => {
    const user = userEvent.setup();
    const saveStructure = vi.fn<SaveStructure>().mockRejectedValue(new Error('SCHEMA_REQUIRED|角色缺少必填字段 arc。'));
    mount(base(), saveStructure);

    await user.click(await screen.findByRole('button', { name: '新增人物' }));
    await user.type(screen.getByLabelText('姓名 *'), '阿绫');
    await user.type(screen.getByLabelText('简介 *'), '守关人的女儿');
    await user.type(screen.getByLabelText('动机 *'), '找回失踪的父亲');
    await user.type(screen.getByLabelText('人物弧光 *'), '从被守护到守护他人');
    await user.selectOptions(screen.getByLabelText('角色定位 *'), 'protagonist');
    await user.click(screen.getByRole('button', { name: '提交新增人物' }));

    const alerts = await screen.findAllByRole('alert');
    // call 包装把 CODE|message 拆成错误码与原始信息，界面展示后端返回的信息本身。
    expect(alerts.some((node) => node.textContent?.includes('角色缺少必填字段 arc。'))).toBe(true);
    expect((screen.getByLabelText('姓名 *') as HTMLInputElement).value).toBe('阿绫');
    expect((screen.getByLabelText('人物弧光 *') as HTMLTextAreaElement).value).toBe('从被守护到守护他人');
    expect(saveStructure).toHaveBeenCalledTimes(1);
  });

  it('无 SaveStructure capability 时不渲染结构增删入口', async () => {
    mount(base({ capabilities: ['SetReview'] }), vi.fn<SaveStructure>());

    await screen.findByText('守关人的女儿');
    expect(screen.queryByText('结构增删')).toBeNull();
    expect(screen.queryByRole('button', { name: '删除 人物 阿绫' })).toBeNull();
    expect(screen.queryByRole('button', { name: '新增人物' })).toBeNull();
  });

  it('有未保存的文本修改时禁用结构增删，避免新候选丢掉未保存输入', async () => {
    const user = userEvent.setup();
    const editor = {
      candidateId: 'people-1', version: 1,
      fields: [{ path: '/characters/@char-1/secret', label: '秘密', group: '人物 · 阿绫', value: '旧秘密', locked: false }],
    };
    mount(base({ fieldEditor: editor }), vi.fn<SaveStructure>());

    expect((await screen.findByRole('button', { name: '删除 人物 阿绫' }) as HTMLButtonElement).disabled).toBe(false);
    await user.type(screen.getByLabelText('候选字段内容'), '补充后的秘密');

    await waitFor(() => expect((screen.getByRole('button', { name: '删除 人物 阿绫' }) as HTMLButtonElement).disabled).toBe(true));
    expect((screen.getByRole('button', { name: '新增人物' }) as HTMLButtonElement).disabled).toBe(true);
  });
});

describe('修订结果的部分采用', () => {
  const revisionCard: RevisionCard = {
    actionId: 'rev-1', argumentsHash: 'hash-1', idempotencyKey: 'key-1', candidateId: 'proposal-1', version: 1,
    state: 'succeeded', instruction: '让标题与基调更克制', targets: [], preserved: [],
    cost: { estimatedCalls: 2, maxRunTokens: 20000, runBudgetUnlimited: false, roleModels: [] },
    runId: 'run-1', quality: 'passed', summary: '已通过独立审阅', stale: false, canApply: true, applied: false,
    diffs: [
      { path: '/title', label: '标题', group: '方案', kind: 'text', before: '旧标题', after: '新标题' },
      { path: '/logline', label: '一句话简介', group: '方案', kind: 'text', before: '旧简介', after: '新简介' },
    ],
  };

  const mount = (view: CreationWorkbenchView, apply: ApplyRevision) => {
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(view),
      CreationWorkbench: vi.fn().mockResolvedValue(view),
      ApplyCreationRevision: apply as unknown as (...args: unknown[]) => Promise<unknown>,
    } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
  };

  afterEach(() => { cleanup(); delete window.go; });

  it('部分采用只把勾选的差异路径交给后端命令', async () => {
    const user = userEvent.setup();
    const view = workbench({ revisions: [revisionCard] });
    const apply = vi.fn<ApplyRevision>().mockResolvedValue(workbench({ revisions: [{ ...revisionCard, applied: true, canApply: false }] }));
    mount(view, apply);

    await user.click(await screen.findByLabelText('方案 · 一句话简介'));
    await user.click(screen.getByRole('button', { name: '采用选中的 1 项修改' }));

    await waitFor(() => expect(apply).toHaveBeenCalledWith({
      actionId: 'rev-1', argumentsHash: 'hash-1', idempotencyKey: 'key-1', paths: ['/title'],
    }));
  });

  it('全部采用不携带路径，未勾选的差异也按整组提交', async () => {
    const user = userEvent.setup();
    const view = workbench({ revisions: [revisionCard] });
    const apply = vi.fn<ApplyRevision>().mockResolvedValue(workbench({ revisions: [{ ...revisionCard, applied: true, canApply: false }] }));
    mount(view, apply);

    await user.click(await screen.findByLabelText('方案 · 一句话简介'));
    await user.click(screen.getByRole('button', { name: '全部采用' }));

    await waitFor(() => expect(apply).toHaveBeenCalledTimes(1));
    expect(apply.mock.calls[0][0]).toEqual({ actionId: 'rev-1', argumentsHash: 'hash-1', idempotencyKey: 'key-1' });
    expect(apply.mock.calls[0][0].paths).toBeUndefined();
  });

  it('后端拒绝部分采用时展示错误并保留勾选状态', async () => {
    const user = userEvent.setup();
    const view = workbench({ revisions: [revisionCard] });
    const apply = vi.fn<ApplyRevision>().mockRejectedValue(new Error('基线已变化，无法采用这组修改。'));
    mount(view, apply);

    await user.click(await screen.findByLabelText('方案 · 一句话简介'));
    await user.click(screen.getByRole('button', { name: '采用选中的 1 项修改' }));

    const alerts = await screen.findAllByRole('alert');
    expect(alerts.some((node) => node.textContent?.includes('基线已变化'))).toBe(true);
    expect((screen.getByLabelText('方案 · 一句话简介') as HTMLInputElement).checked).toBe(false);
    expect((screen.getByLabelText('方案 · 标题') as HTMLInputElement).checked).toBe(true);
  });
});

describe('修订的结构授权', () => {
  // 预览请求的测试替身类型：断言点集中在请求携带的集合白名单上。
  type PreviewRevision = (request: { candidateId: string; paths: string[]; instruction: string; idempotencyKey: string; allowAdd?: string[]; allowRemove?: string[] }) => Promise<RevisionCard>;
  const editor = {
    candidateId: 'people-1', version: 1,
    fields: [{ path: '/characters/@char-1/summary', label: '简介', group: '人物 · 阿绫', value: '旧简介', locked: false }],
  };
  const structure: StructureCollectionView[] = [
    { collection: 'characters', title: '人物', identityField: 'id', entities: [{ key: 'char-1', title: '阿绫' }] },
    { collection: 'factions', title: '势力', identityField: 'name', entities: [{ key: '守军', title: '守军' }] },
  ];
  const revisionCard: RevisionCard = {
    actionId: 'rev-1', argumentsHash: 'hash-1', idempotencyKey: 'key-1', candidateId: 'people-1', version: 1,
    state: 'pending_confirmation', instruction: '补一个守塔人', targets: [], preserved: [],
    cost: { estimatedCalls: 2, maxRunTokens: 20000, runBudgetUnlimited: false, roleModels: [] },
    runId: '', quality: '', summary: '', stale: false, canApply: false, applied: false, diffs: [],
  };

  const mount = (preview: PreviewRevision) => {
    const view = workbench({ stage: 'people_world', fieldEditor: editor, structure });
    window.go = { main: { DesktopAPI: {
      RecoverCreation: vi.fn().mockResolvedValue(view),
      CreationWorkbench: vi.fn().mockResolvedValue(view),
      PreviewCreationRevision: preview as unknown as (...args: unknown[]) => Promise<unknown>,
    } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
  };

  afterEach(() => { cleanup(); delete window.go; });

  it('预览把勾选的集合白名单合并进请求，未勾选时请求不带授权字段', async () => {
    const user = userEvent.setup();
    const preview = vi.fn<PreviewRevision>().mockResolvedValue(revisionCard);
    mount(preview);

    await user.type(await screen.findByLabelText('修改要求'), '补一个守塔人');
    await user.click(screen.getByText('允许 AI 增删实体（可选）'));
    await user.click(screen.getByLabelText('允许新增 人物'));
    await user.click(screen.getByRole('button', { name: '查看修改提案与费用' }));

    await waitFor(() => expect(preview).toHaveBeenCalledTimes(1));
    const request = preview.mock.calls[0][0];
    expect(request.instruction).toBe('补一个守塔人');
    expect(request.paths).toEqual(['/characters/@char-1/summary']);
    expect(request.allowAdd).toEqual(['characters']);
    expect(request.allowRemove).toBeUndefined();
  });
});

it('入档先展示正式资产预览，确认后才写入', async () => {
  const preview = vi.fn().mockResolvedValue({confirmationDigest:'archive-digest',title:'潮汐关',characterCount:1,outlineCount:1,anchorCount:2,operations:[{kind:'character',action:'create',summary:'新增人物：江潮'}],excluded:[]});
  const confirm = vi.fn().mockResolvedValue({changeSetId:'archived'});
  const view = workbench({stage:'archive_confirm',capabilities:['Archive'],review:{candidateId:'c',stage:'archive_confirm',reviewVersion:1,accepted:[],rejected:[],unreviewed:[],complete:true,updatedAt:''}});
  window.go = {main:{DesktopAPI:{CreationWorkbench:vi.fn().mockResolvedValue(view),RecoverCreation:vi.fn().mockResolvedValue(view),PreviewCreationArchive:preview,ConfirmCreationArchive:confirm,ProjectWorkbench:vi.fn().mockResolvedValue({stage:'writing'})}}};
  render(<MemoryRouter><AppContext.Provider value={contextValue({hasCredential:true})}><Cockpit/></AppContext.Provider></MemoryRouter>);
  await userEvent.click(await screen.findByRole('button',{name:'确认作品档案'}));
  expect(await screen.findByText('新增人物：江潮',{exact:false})).toBeTruthy();
  expect(confirm).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'确认写入正式档案'}));
  await waitFor(()=>expect(confirm).toHaveBeenCalledWith(expect.objectContaining({confirmationDigest:'archive-digest',idempotencyKey:expect.any(String)})));
  cleanup();delete window.go;
});
