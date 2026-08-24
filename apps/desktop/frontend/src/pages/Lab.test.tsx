import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Lab from './Lab';
import { AppProvider } from '../store/AppContext';

const run = { run: { id: 'run-1', status: 'completed', case_ids: ['case-1'], binding: { variant_id: 'minimal' }, started_at: '' }, completedCases: 1, completedCaseIds: ['case-1'], totalCases: 1 };
const blind = {
  case: { case_id: 'case-1', genre: '东方幻想', task_kind: 'chapter', rating_dimensions: ['人物动机'], chapter_goal: '守住灯塔' },
  candidates: [{ slot: '候选-1', content: '匿名正文', metrics: {} }],
};

describe('智能体实验室盲评', () => {
  beforeEach(() => {
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: vi.fn().mockResolvedValue({ preferences: { theme: 'light', defaultCreativeProfileId: 'profile-1' }, recentProjects: [], providers: [], profiles: [{ profileId: 'profile-1', version: 1, name: '参考方案', roles: [], maxRunTokens: 1000, maxContextChars: 1000, createdAt: '' }] }),
      ListCreativeHarnessDatasets: vi.fn().mockResolvedValue({ dataset: { dataset_id: 'set', version: '1', sha256: 'x', originality_note: '原创', cases: [{}] }, variants: [{ id: 'minimal', workflow_id: 'quality.chapter', workflow_version: 1, default_enabled: true }] }),
      ListCreativeHarnessRuns: vi.fn().mockResolvedValue([run]),
      GetCreativeHarnessCaseResult: vi.fn().mockResolvedValue(blind),
      SaveCreativeHarnessRating: vi.fn().mockResolvedValue({ ...blind, rating: { id: 'rating-1' }, revealed: { '候选-1': 'revised' }, binding: { model: 'writer-model' } }),
    } } };
  });
  afterEach(() => { delete window.go; });

  it('评分前不泄露身份，整组提交后才揭示来源，并明确不修改正式内容', async () => {
    const user = userEvent.setup();
    render(<MemoryRouter><AppProvider><Lab /></AppProvider></MemoryRouter>);

    expect(await screen.findByText(/实验结果不会写入作品正文/)).toBeTruthy();
    await user.click(await screen.findByRole('button', { name: 'case-1' }));
    expect(await screen.findByText('匿名正文')).toBeTruthy();
    expect(screen.queryByText(/revised/)).toBeNull();

    await user.click(screen.getByRole('button', { name: '提交整组评分并揭示来源' }));
    expect(await screen.findByText(/已揭示：revised/)).toBeTruthy();
    expect(screen.getByText(/重复提交会被拒绝/)).toBeTruthy();
  });

  it('Bridge 加载失败时提供可聚焦的错误与重试入口', async () => {
    window.go!.main!.DesktopAPI!.ListCreativeHarnessDatasets = vi.fn().mockRejectedValue(new Error('Bridge 暂不可用'));
    render(<MemoryRouter><AppProvider><Lab /></AppProvider></MemoryRouter>);
    expect((await screen.findByRole('alert')).textContent).toContain('Bridge 暂不可用');
    expect(screen.getByRole('button', { name: '重试加载' })).toBeTruthy();
  });
});
