import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Cockpit from './Cockpit';
import ArchiveLayout from './archive/ArchiveLayout';
import { AppProvider } from '../store/AppContext';
import { AppContext, type AppCtxValue } from '../store/appContextValue';
import { defaultSettings } from '../store/defaultSettings';
import type { CreationWorkbenchView } from '../api/studio';

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
      book: { id: '', title: '', summary: '', position: 0 },
      volumes: [], arcs: [], chapters: [],
    },
    currentRun: { runId: '', status: '', cancellable: false, publicStage: '' },
    assistant: { pendingActionCount: 0 },
    impact: {
      items: [], touchesLocked: false, touchesConfirmed: false, canAdvance: true,
      regenerateStages: [], estimatedCalls: 0, evidence: '',
    },
    capabilities: [],
    ...overrides,
  };
}

function contextValue(options: { archiveStatus?: string; hasCredential?: boolean } = {}): AppCtxValue {
  return {
    state: {
      currentProject: null,
      backendProject: {
        id: 'project-1',
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
      roadmap: {
        candidateId: 'roadmap-1', version: 1, status: 'draft',
        book: { id: 'book', title: '潮汐关', summary: '守关人必须选择', position: 1 },
        volumes: [{ id: 'volume:第一卷', title: '第一卷', summary: '建立关隘危机', position: 1 }],
        arcs: [{
          id: 'arc:arc-1', title: '开关', summary: '敌船靠岸导致必须抉择', position: 1,
          readerExpectation: '读者应感到守关人将被亲情撕裂',
          stagePayoff: '开城救人并失去军职',
          irreversibleChange: '阿绫不再以军令为最高准则',
          endingContribution: '为终局选择救人立下先例',
          commitments: [{ id: 'cmt-1', statement: '旧令牌兑现身份', detail: '本弧末章' }],
          retainedQuestions: [{ id: 'q-1', statement: '潮汐神谕是否被篡改', detail: '留给后续卷揭示' }],
        }],
        chapters: [{ id: 'chapter:空船', title: '空船', summary: '发现异常', position: 1 }],
      },
    });
    window.go = { main: { DesktopAPI: { RecoverCreation: vi.fn().mockResolvedValue(view), CreationWorkbench: vi.fn().mockResolvedValue(view) } } };
    render(<MemoryRouter><AppContext.Provider value={contextValue({ hasCredential: true })}><Cockpit /></AppContext.Provider></MemoryRouter>);
    expect(await screen.findByRole('button', { name: '开关' })).toBeTruthy();
    expect(screen.getByText('读者应感到守关人将被亲情撕裂')).toBeTruthy();
    expect(screen.getByText('待兑现承诺')).toBeTruthy();
    expect(screen.getByText('旧令牌兑现身份')).toBeTruthy();
    expect(screen.getByText('潮汐神谕是否被篡改')).toBeTruthy();
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
