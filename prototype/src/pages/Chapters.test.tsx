import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import Chapters from './Chapters';
import { AppProvider } from '../store/AppContext';

describe('正文编辑器模型提示', () => {
  it('保持暖灰提示底色，不随正文画布切换', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AppProvider>
          <Chapters />
        </AppProvider>
      </MemoryRouter>,
    );

    const notice = screen.getByTestId('model-configuration-notice');
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
