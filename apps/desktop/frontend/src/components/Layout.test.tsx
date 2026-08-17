import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Layout } from './Layout';
import { AppProvider } from '../store/AppContext';

Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
  value: () => undefined,
  writable: true,
});

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  vi.restoreAllMocks();
});

describe('应用与项目工作区边界', () => {
  it('macOS 窗口标题栏不重复显示软件名称', () => {
    vi.spyOn(window.navigator, 'userAgent', 'get').mockReturnValue('Macintosh');

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

    expect(screen.queryByText('织境 Story Studio')).toBeNull();
    expect(screen.getByRole('button', { name: '未配置模型' })).toBeTruthy();
  });

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

  it('不会把浏览器 localStorage 中的原型项目当作真实项目恢复', () => {
    window.localStorage.setItem('story-studio.active-project', JSON.stringify({ id: 'mock-project', title: '原型项目' }));

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
    expect(screen.queryByText('原型项目')).toBeNull();
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
