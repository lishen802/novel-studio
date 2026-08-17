import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { Layout } from './Layout';
import { AppProvider } from '../store/AppContext';
import { currentProject } from '../mock/data';

const activeProjectStorageKey = 'story-studio.active-project';

Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
  value: () => undefined,
  writable: true,
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
});

describe('应用与项目工作区边界', () => {
  it('应用设置不渲染当前作品侧栏或项目级 AI 面板', () => {
    render(
      <MemoryRouter initialEntries={['/settings']}>
        <AppProvider>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route path="settings" element={<h1>应用设置内容</h1>} />
            </Route>
          </Routes>
        </AppProvider>
      </MemoryRouter>,
    );

    expect(screen.getByRole('heading', { name: '应用设置内容' })).toBeTruthy();
    expect(screen.getByTestId('route-viewport').className).toContain('h-full');
    expect(screen.queryByText('当前作品')).toBeNull();
    expect(screen.queryByTitle('打开 AI 面板')).toBeNull();
  });

  it('刷新正文页面会恢复最近打开的作品并显示一级项目导航', () => {
    window.localStorage.setItem(activeProjectStorageKey, JSON.stringify(currentProject));

    render(
      <MemoryRouter initialEntries={['/chapters']}>
        <AppProvider>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route path="chapters" element={<h1>正文编辑</h1>} />
            </Route>
          </Routes>
        </AppProvider>
      </MemoryRouter>,
    );

    expect(screen.getByText('当前作品')).toBeTruthy();
    expect(screen.getByText(currentProject.title)).toBeTruthy();
    expect(screen.getByRole('link', { name: '正文与章节' })).toBeTruthy();
  });

  it('没有可恢复作品时，刷新项目地址会返回首页', () => {
    render(
      <MemoryRouter initialEntries={['/chapters']}>
        <AppProvider>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<h1>首页</h1>} />
              <Route path="chapters" element={<h1>正文编辑</h1>} />
            </Route>
          </Routes>
        </AppProvider>
      </MemoryRouter>,
    );

    expect(screen.getByRole('heading', { name: '首页' })).toBeTruthy();
    expect(screen.queryByRole('heading', { name: '正文编辑' })).toBeNull();
  });
});
