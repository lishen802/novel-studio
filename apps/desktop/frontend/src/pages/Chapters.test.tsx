import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import Chapters from './Chapters';
import { AppProvider } from '../store/AppContext';

describe('正文编辑器模型提示', () => {
  beforeEach(() => {
    window.go = { main: { DesktopAPI: {
      ApplicationBootstrap: vi.fn().mockResolvedValue({ preferences: {}, recentProjects: [], providers: [], profiles: [] }),
      ListChapters: vi.fn().mockResolvedValue([{ id: 'chapter-1', projectId: 'project-1', title: '第一章', position: 1, body: '', revision: 1, updatedAt: '', hasRecoveryDraft: false }]),
      OpenChapter: vi.fn().mockResolvedValue({ id: 'chapter-1', projectId: 'project-1', title: '第一章', position: 1, body: '', revision: 1, updatedAt: '', hasRecoveryDraft: false }),
    } } };
  });
  afterEach(() => { delete window.go; });

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
