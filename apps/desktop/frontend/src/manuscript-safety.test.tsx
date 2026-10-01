import { useEffect } from 'react';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, createMemoryRouter, RouterProvider, Link, Outlet } from 'react-router-dom';
import { EditorNavigationGuard } from './components/EditorNavigationGuard';
import { afterEach, expect, it, vi } from 'vitest';
import Chapters from './pages/Chapters';
import { AppProvider } from './store/AppContext';
import { useApp } from './store/useApp';
import type { ChapterView } from './api/studio';

const one: ChapterView = { id: 'c1', projectId: 'p1', title: '第一章', position: 1, body: '原有正文', revision: 1, hasRecoveryDraft: false, updatedAt: '' };
const two = { ...one, id: 'c2', title: '第二章', position: 2, body: '第二章正文' };
const bootstrap = { preferences: {}, recentProjects: [], providers: [], profiles: [] };
afterEach(() => { cleanup(); delete window.go; delete (window as Window & {runtime?:unknown}).runtime; });
function setup(first = one) {
 const save = vi.fn().mockResolvedValue(undefined);
 window.go = { main: { DesktopAPI: {
  ApplicationBootstrap: vi.fn().mockResolvedValue(bootstrap),
  ListChapters: vi.fn().mockResolvedValue([first, two]),
  OpenChapter: vi.fn(async (id: unknown) => id === 'c1' ? first : two),
  AgentRuns: vi.fn().mockResolvedValue([]), SaveDraft: save,
 } } };
 const view = render(<MemoryRouter><AppProvider><Chapters /></AppProvider></MemoryRouter>);
 return {save, view};
}
it('快速切章仍须保存刚输入的正文', async () => {
 const {save} = setup();
 const editor = await screen.findByPlaceholderText('从这里开始书写……');
 await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe('原有正文'));
 fireEvent.change(editor, { target: { value: '作者的新内容' } });
 fireEvent.click(screen.getByRole('button', { name: /第二章/ }));
 await waitFor(() => expect((editor as HTMLTextAreaElement).value).toBe('第二章正文'));
 await act(async () => { await new Promise(resolve => setTimeout(resolve, 950)); });
 expect(save).toHaveBeenCalledWith('c1', '作者的新内容', 1);
});
it('空白恢复草稿不能被原正文替代', async () => {
 setup({ ...one, hasRecoveryDraft: true, recoveryBody: '' });
 const editor = await screen.findByPlaceholderText('从这里开始书写……');
 await waitFor(() => expect((screen.getByPlaceholderText('章节标题') as HTMLInputElement).value).toBe('第一章'));
 expect((editor as HTMLTextAreaElement).value).toBe('');
});
function CloseProbe() {
 const {registerBeforeClose} = useApp();
 useEffect(() => registerBeforeClose(async () => { throw new Error('磁盘满'); }), [registerBeforeClose]);
 return null;
}
it('草稿保存失败必须阻止退出', async () => {
 let close: (()=>void) | undefined;
 (window as Window & {runtime?:unknown}).runtime = {EventsOn: (name: string, cb: ()=>void) => { if(name === 'app:before-close') close = cb; return ()=>{}; }};
 const complete = vi.fn().mockResolvedValue(undefined);
 window.go = {main:{DesktopAPI:{ApplicationBootstrap: vi.fn().mockResolvedValue(bootstrap),CompleteClose: complete}}};
 render(<AppProvider><CloseProbe /></AppProvider>);
 await act(async () => {close?.();});
 expect(complete).not.toHaveBeenCalled();
});

it('切页保存失败时保留编辑器与输入，重试成功后才能离开', async () => {
 const save=vi.fn().mockRejectedValueOnce(new Error('磁盘满')).mockResolvedValue(undefined);
 window.go={main:{DesktopAPI:{ApplicationBootstrap:vi.fn().mockResolvedValue(bootstrap),ListChapters:vi.fn().mockResolvedValue([one]),OpenChapter:vi.fn().mockResolvedValue(one),AgentRuns:vi.fn().mockResolvedValue([]),SaveDraft:save}}};
 const router=createMemoryRouter([{path:'/',element:<AppProvider><EditorNavigationGuard/><Link to="/review">切到审阅</Link><Outlet/></AppProvider>,children:[{index:true,element:<Chapters/>},{path:'review',element:<p>审阅页面</p>}]}]);
 render(<RouterProvider router={router}/>);
 const editor=await screen.findByPlaceholderText('从这里开始书写……');
 await waitFor(()=>expect((editor as HTMLTextAreaElement).value).toBe('原有正文'));
 fireEvent.change(editor,{target:{value:'切页前的新稿'}});
 fireEvent.click(screen.getByRole('link',{name:'切到审阅'}));
 await waitFor(()=>expect(save).toHaveBeenCalledTimes(1));
 expect(router.state.location.pathname).toBe('/');
 expect((editor as HTMLTextAreaElement).value).toBe('切页前的新稿');
 fireEvent.click(screen.getByRole('link',{name:'切到审阅'}));
 expect(await screen.findByText('审阅页面')).toBeTruthy();
 expect(save).toHaveBeenLastCalledWith('c1','切页前的新稿',1);
});

it('保存失败时不能切章，编辑输入仍在',async()=>{
 const {save}=setup();save.mockRejectedValue(new Error('磁盘满'));
 const editor=await screen.findByPlaceholderText('从这里开始书写……');
 await waitFor(()=>expect((editor as HTMLTextAreaElement).value).toBe('原有正文'));
 fireEvent.change(editor,{target:{value:'必须保留'}});fireEvent.click(screen.getByRole('button',{name:/第二章/}));
 await waitFor(()=>expect(screen.getByRole('alert').textContent).toContain('磁盘满'));
 expect((editor as HTMLTextAreaElement).value).toBe('必须保留');
});

it('本章历史显示旧正文，恢复前保存当前输入为版本',async()=>{
 setup();
 const commit=vi.fn().mockResolvedValue({...one,body:'当前工作稿',revision:2});
 const restore=vi.fn().mockResolvedValue({...one,body:'历史正文',revision:3});
 Object.assign(window.go!.main!.DesktopAPI!,{ChapterHistory:vi.fn().mockResolvedValue([{chapterId:'c1',revision:0,title:'第一章',body:'历史正文',createdAt:'过去',isCurrent:false}]),CommitChapter:commit,RestoreChapterRevision:restore});
 const editor=await screen.findByPlaceholderText('从这里开始书写……');
 await waitFor(()=>expect((editor as HTMLTextAreaElement).value).toBe('原有正文'));
 fireEvent.change(editor,{target:{value:'当前工作稿'}});fireEvent.click(screen.getByRole('button',{name:'版本'}));
 expect(await screen.findByText('历史正文')).toBeTruthy();expect(restore).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole('button',{name:'保存当前版本并恢复此版'}));
 await waitFor(()=>expect((editor as HTMLTextAreaElement).value).toBe('历史正文'));
 expect(commit).toHaveBeenCalledWith('c1','当前工作稿',1);expect(restore).toHaveBeenCalledWith('c1',0,2);
});

it('选区操作未就绪时明确提示，不借用整章生成',async()=>{
 setup();const create=vi.fn();Object.assign(window.go!.main!.DesktopAPI!,{CreateChapterAgentRun:create});
 const editor=await screen.findByPlaceholderText('从这里开始书写……');await waitFor(()=>expect((editor as HTMLTextAreaElement).value).toBe('原有正文'));
 (editor as HTMLTextAreaElement).setSelectionRange(0,2);fireEvent.mouseUp(editor);
 const explain=screen.queryByRole('button',{name:'解释'});
 if (!explain) { fireEvent.select(editor); }
 fireEvent.click(screen.getByRole('button',{name:'解释'}));
 expect(await screen.findByText(/解释这段尚未接入/)).toBeTruthy();expect(create).not.toHaveBeenCalled();
});
