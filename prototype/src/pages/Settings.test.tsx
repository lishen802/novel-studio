import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import Settings from './Settings';
import { AppProvider } from '../store/AppContext';

describe('应用级创作方案', () => {
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
