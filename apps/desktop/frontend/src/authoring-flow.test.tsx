import {cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react';
import {afterEach, expect, it, vi} from 'vitest';
import {createMemoryRouter, Link, Outlet, RouterProvider} from 'react-router-dom';
import Chapters from './pages/Chapters';
import Review from './pages/Review';
import {AppProvider} from './store/AppContext';
import {EditorNavigationGuard} from './components/EditorNavigationGuard';
import type {ChapterView, ChangeSetView} from './api/studio';

afterEach(()=>{cleanup();delete window.go;});

// 有状态的绑定替身只验证跨页面操作合同；实际落库与版本冲突另由 SQLite 集成测试验证。
it('自动保存后切章、审阅冲突、重新采用与历史恢复始终保留作者版本',async()=>{
 let chapter:ChapterView={id:'c1',projectId:'p1',title:'第一章',position:1,body:'初始正文',revision:1,hasRecoveryDraft:false,updatedAt:''};
 const second={...chapter,id:'c2',title:'第二章',position:2,body:'第二章内容'};
 const versions=new Map<number,string>([[1,chapter.body]]);
 let draft:string|undefined;
 let baseRevision=1;
 const candidate:ChangeSetView={id:'candidate',title:'正文候选',summary:'修订动机',createdBy:'agent',status:'pending',createdAt:'',updatedAt:'',operations:[{id:'op',kind:'replace_chapter_body',targetId:'c1',beforeValue:chapter.body,afterValue:'AI 候选正文',reason:'让动机更清晰',status:'pending'}]};
 const save=vi.fn(async(id:unknown,body:unknown,revision:unknown)=>{
  expect(id).toBe('c1');expect(revision).toBe(chapter.revision);draft=String(body);
 });
 const commit=vi.fn(async(id:unknown,body:unknown,revision:unknown)=>{
  expect(id).toBe('c1');expect(revision).toBe(chapter.revision);
  chapter={...chapter,body:String(body),revision:chapter.revision+1};versions.set(chapter.revision,chapter.body);draft=undefined;return {...chapter};
 });
 const apply=vi.fn(async()=>{
  if(draft!==undefined && draft!==chapter.body)throw new Error('请先保存工作稿为版本，候选已保留');
  if(baseRevision!==chapter.revision)throw new Error('候选基准已变化');
  chapter={...chapter,body:candidate.operations[0].afterValue,revision:chapter.revision+1};versions.set(chapter.revision,chapter.body);draft=undefined;
  candidate.status='applied';candidate.operations[0].status='applied';return {};
 });
 const restore=vi.fn(async(id:unknown,target:unknown,revision:unknown)=>{
  expect(id).toBe('c1');expect(revision).toBe(chapter.revision);
  chapter={...chapter,body:versions.get(Number(target))!,revision:chapter.revision+1};versions.set(chapter.revision,chapter.body);draft=undefined;return {...chapter};
 });
 window.go={main:{DesktopAPI:{
  ApplicationBootstrap:vi.fn().mockResolvedValue({preferences:{},recentProjects:[],providers:[],profiles:[]}),
  ListChapters:vi.fn(async()=>[{...chapter},second]),
  OpenChapter:vi.fn(async(id:unknown)=>id==='c1'?{...chapter,hasRecoveryDraft:draft!==undefined,recoveryBody:draft}:second),
  AgentRuns:vi.fn().mockResolvedValue([]),SaveDraft:save,CommitChapter:commit,
  ChangeSets:vi.fn(async()=>[structuredClone(candidate)]),ApplyChangeSet:apply,
  ChapterHistory:vi.fn(async()=>Array.from(versions,([revision,body])=>({chapterId:'c1',revision,body,title:'第一章',createdAt:'测试时间',isCurrent:revision===chapter.revision}))),RestoreChapterRevision:restore,
 }}};
 const router=createMemoryRouter([{path:'/',element:<AppProvider><EditorNavigationGuard/><Link to="/">正文</Link><Link to="/review">审阅</Link><Outlet/></AppProvider>,children:[{index:true,element:<Chapters/>},{path:'review',element:<Review/>}]}]);
 render(<RouterProvider router={router}/>);
 const body=()=>screen.getByPlaceholderText('从这里开始书写……') as HTMLTextAreaElement;
 await waitFor(()=>expect(body().value).toBe('初始正文'));
 fireEvent.change(body(),{target:{value:'作者自动保存稿'}});
 await waitFor(()=>expect(save).toHaveBeenCalledWith('c1','作者自动保存稿',1),{timeout:2000});
 fireEvent.click(screen.getByRole('button',{name:/第二章/}));await waitFor(()=>expect(body().value).toBe('第二章内容'));
 fireEvent.click(screen.getByRole('button',{name:/第一章/}));await waitFor(()=>expect(body().value).toBe('作者自动保存稿'));
 fireEvent.click(screen.getByRole('link',{name:'审阅'}));fireEvent.click(await screen.findByRole('button',{name:'应用整组'}));
 expect(await screen.findByText(/请先保存工作稿为版本/)).toBeTruthy();expect(candidate.status).toBe('pending');expect(draft).toBe('作者自动保存稿');
 fireEvent.click(screen.getByRole('link',{name:'正文'}));await waitFor(()=>expect(body().value).toBe('作者自动保存稿'));
 fireEvent.click(screen.getByRole('button',{name:'保存版本'}));await waitFor(()=>expect(chapter.revision).toBe(2));
 // 模拟在作者确认的新基线上重新产出的候选，不将旧候选静默换绑。
 baseRevision=chapter.revision;candidate.id='candidate-new';candidate.operations[0].id='op-new';candidate.operations[0].beforeValue=chapter.body;
 fireEvent.click(screen.getByRole('link',{name:'审阅'}));fireEvent.click(await screen.findByRole('button',{name:'应用整组'}));
 await waitFor(()=>expect(candidate.status).toBe('applied'));
 fireEvent.click(screen.getByRole('link',{name:'正文'}));await waitFor(()=>expect(body().value).toBe('AI 候选正文'));
 fireEvent.change(body(),{target:{value:'采用后的作者修改'}});fireEvent.click(screen.getByRole('button',{name:'版本'}));
 const history=await screen.findByLabelText('历史版本');await waitFor(()=>expect((history as HTMLSelectElement).options.length).toBe(3));
 fireEvent.change(history,{target:{value:'2'}});fireEvent.click(screen.getByRole('button',{name:'保存当前版本并恢复此版'}));
 await waitFor(()=>expect(body().value).toBe('作者自动保存稿'));
 expect(versions.get(4)).toBe('采用后的作者修改');expect(versions.get(3)).toBe('AI 候选正文');expect(chapter.revision).toBe(5);
 expect(restore).toHaveBeenCalledWith('c1',2,4);
 fireEvent.click(screen.getByRole('link',{name:'审阅'}));await screen.findByRole('heading',{name:'审阅与候选'});
 fireEvent.click(screen.getByRole('link',{name:'正文'}));await waitFor(()=>expect(body().value).toBe('作者自动保存稿'));
});
