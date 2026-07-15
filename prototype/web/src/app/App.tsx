import { useState } from 'react'
import { Bell, BookOpen, Boxes, Command, Download, Library, Menu, PanelLeftClose, Search, Settings2 } from 'lucide-react'
import { DashboardPage } from '../features/dashboard/DashboardPage'
import { StudioPageRouter } from '../features/pages/StudioPageRouter'
import { WorkspaceLibrary } from '../features/workspace/WorkspaceLibrary'
import { useStudioStore } from '../store/useStudioStore'
import { projectNavigation, type PageId } from './navigation'

export function App() {
  const [page, setPage] = useState<PageId>('dashboard')
  const [sidebarOpen, setSidebarOpen] = useState(() => window.innerWidth > 980)
  const [toast, setToast] = useState('')
  const projects = useStudioStore((state) => state.projects)
  const selectedProjectId = useStudioStore((state) => state.selectedProjectId)
  const selectProject = useStudioStore((state) => state.selectProject)
  const replaceProject = useStudioStore((state) => state.replaceProject)
  const createSnapshot = useStudioStore((state) => state.createSnapshot)
  const restoreSnapshot = useStudioStore((state) => state.restoreSnapshot)
  const project = projects.find((item) => item.id === selectedProjectId)
  const showToast = (message: string) => { setToast(message); window.setTimeout(() => setToast(''), 2600) }

  return <div className="studio-app">
    <aside className="global-rail">
      <div className="brand-mark">织</div>
      <nav aria-label="全局导航">
        <button className={page === 'library' ? 'active' : ''} aria-label="作品库" onClick={() => setPage('library')}><Library /></button>
        <button aria-label="模型中心" onClick={() => { setPage('agents'); showToast('已进入模型与智能体配置') }}><Boxes /></button>
        <button aria-label="导出中心" onClick={() => { setPage('versions'); showToast('可从版本页导出当前作品') }}><Download /></button>
      </nav>
      <button aria-label="全局设置" onClick={() => setPage('settings')}><Settings2 /></button>
    </aside>
    {page === 'library' || !project ? <main className="library-shell"><header className="library-topbar"><h1>织境 Story Studio</h1><div><button className="icon-button" aria-label="通知"><Bell size={18} /></button><span className="avatar">SL</span></div></header><WorkspaceLibrary onOpen={() => setPage('dashboard')} /></main> : <>
      <aside className={`project-sidebar ${sidebarOpen ? '' : 'collapsed'}`}>
        <div className="sidebar-brand"><BookOpen size={22} /><div><h1>织境 Story Studio</h1><span>LONGFORM FICTION OS</span></div></div>
        <button className="project-switcher" onClick={() => setPage('library')}><div><small>当前作品</small><strong>{project.title}</strong></div><span>⌄</span></button>
        <nav aria-label="作品导航">{projectNavigation.map(([id, label, Icon]) => <button key={id} className={page === id ? 'active' : ''} onClick={() => setPage(id)}><Icon size={17} /><span>{label}</span>{id === 'review' && project.reviews.filter((item) => item.disposition === 'open').length > 0 && <b>{project.reviews.filter((item) => item.disposition === 'open').length}</b>}</button>)}</nav>
        <div className="sidebar-projects"><span>快速切换</span>{projects.slice(0, 3).map((item) => <button key={item.id} onClick={() => { selectProject(item.id); setPage('dashboard') }}><i>{item.title[0]}</i><span>{item.title}</span></button>)}</div>
      </aside>
      <main className="workspace-shell">
        <header className="workspace-topbar"><button className="icon-button" aria-label="切换侧栏" onClick={() => setSidebarOpen(!sidebarOpen)}>{sidebarOpen ? <PanelLeftClose size={19} /> : <Menu size={19} />}</button><button className="command-search" onClick={() => showToast('命令面板：可使用左侧导航快速跳转')}><Search size={17} /><span>搜索角色、章节、事实或命令</span><kbd><Command size={12} /> K</kbd></button><div className="topbar-actions"><span className="autosave">● 已自动保存</span><button className="icon-button" aria-label="通知" onClick={() => showToast('没有新的系统通知')}><Bell size={18} /></button><span className="avatar">SL</span></div></header>
        {page === 'dashboard' ? <DashboardPage project={project} onNavigate={(next) => setPage(next as PageId)} /> : <StudioPageRouter page={page} project={project} onChange={replaceProject} toast={showToast} createSnapshot={createSnapshot} restoreSnapshot={restoreSnapshot} />}
      </main>
    </>}
    {toast && <div className="toast" role="status">{toast}</div>}
  </div>
}
