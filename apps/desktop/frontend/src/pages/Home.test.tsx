import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import Home from './Home';
import { AppProvider } from '../store/AppContext';

describe('首页项目入口', () => {
  it('从一个想法开始会先打开原创作品创建卡，而不是直接进入预置作品', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AppProvider>
          <Home />
        </AppProvider>
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: /从一个想法开始/ }));

    expect(await screen.findByRole('heading', { name: '创建原创作品' })).toBeTruthy();
  });
});
