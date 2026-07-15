import { AlertTriangle, ArrowRight, BookOpenText, Bot, GitBranch, Network, Route, Sparkles, Users } from 'lucide-react'
import type { Project } from '../../domain/types'

export function DashboardPage({ project, onNavigate }: { project: Project; onNavigate: (page: string) => void }) {
  const words = project.chapters.reduce((sum, chapter) => sum + chapter.content.length, 0)
  const openReviews = project.reviews.filter((review) => review.disposition === 'open').length
  const conflicted = project.facts.filter((fact) => fact.status === 'conflicted').length
  const current = project.chapters.find((chapter) => chapter.id === project.currentChapterId) ?? project.chapters.at(-1)
  return <div className="page dashboard-page">
    <header className="page-header"><div><span className="eyebrow">创作驾驶舱</span><h2>{project.title}</h2><p>{project.summary}</p></div><button className="primary" onClick={() => onNavigate('workflow')}><Sparkles size={17} /> 运行章节工作流</button></header>
    <section className="metric-grid">
      <article><span>正文进度</span><strong>{words.toLocaleString()}</strong><small>/ {project.targetWords.toLocaleString()} 字</small></article>
      <article><span>章节</span><strong>{project.chapters.length}</strong><small>{project.chapters.filter((c) => c.status === 'approved').length} 章已通过</small></article>
      <article><span>故事资产</span><strong>{project.characters.length + project.factions.length + project.locations.length}</strong><small>角色 · 势力 · 地点</small></article>
      <article className={openReviews ? 'attention' : ''}><span>待处理审稿</span><strong>{openReviews}</strong><small>{conflicted} 个事实冲突</small></article>
    </section>
    <div className="dashboard-grid">
      <section className="panel focus-panel"><div className="panel-heading"><div><span className="eyebrow">CURRENT FOCUS</span><h3>{current ? `第${current.number}章 · ${current.title}` : '开始第一章'}</h3></div><span className="status-pill">{current?.status ?? 'planned'}</span></div><p>{current?.summary || '为当前章节补充目标、冲突和结果。'}</p><div className="focus-actions"><button className="primary" onClick={() => onNavigate('manuscript')}><BookOpenText size={16} /> 继续写作</button><button className="secondary" onClick={() => onNavigate('outline')}>查看章节规划</button></div></section>
      <section className="panel"><div className="panel-heading"><h3>质量雷达</h3><button className="text-button" onClick={() => onNavigate('review')}>查看全部 <ArrowRight size={14} /></button></div><div className="quality-list"><div><AlertTriangle size={18} /><span>时间连续性</span><b>{openReviews ? '需关注' : '正常'}</b></div><div><Users size={18} /><span>角色戏份偏差</span><b>+3%</b></div><div><GitBranch size={18} /><span>未回收伏笔</span><b>{project.hooks.filter((hook) => hook.status !== 'paid-off').length}</b></div></div></section>
      <section className="panel span-2"><div className="panel-heading"><h3>故事系统</h3><span className="muted">所有视图共享同一套事实</span></div><div className="system-links"><button onClick={() => onNavigate('characters')}><Users /><strong>角色与势力</strong><span>{project.characters.length} 位角色</span></button><button onClick={() => onNavigate('timeline')}><Route /><strong>多轨时间线</strong><span>{project.events.length} 个事件</span></button><button onClick={() => onNavigate('graph')}><Network /><strong>关系拓扑</strong><span>{project.relationships.length} 条关系</span></button><button onClick={() => onNavigate('agents')}><Bot /><strong>智能体团队</strong><span>{project.agents.filter((a) => a.enabled).length} 个启用</span></button></div></section>
    </div>
  </div>
}
