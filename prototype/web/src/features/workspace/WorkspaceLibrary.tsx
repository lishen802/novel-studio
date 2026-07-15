import { useRef, useState } from 'react'
import { BookOpen, Copy, Download, FileUp, Plus, Trash2 } from 'lucide-react'
import { Modal } from '../../components/Modal'
import { createDemoProject } from '../../domain/demoData'
import { splitImportedChapters, analyzeImportedText } from '../import/importText'
import { buildProjectJson, downloadText, parseProjectJson } from '../../services/exportService'
import { useStudioStore } from '../../store/useStudioStore'

export function WorkspaceLibrary({ onOpen }: { onOpen: () => void }) {
  const projects = useStudioStore((state) => state.projects)
  const createProject = useStudioStore((state) => state.createProject)
  const duplicateProject = useStudioStore((state) => state.duplicateProject)
  const deleteProject = useStudioStore((state) => state.deleteProject)
  const selectProject = useStudioStore((state) => state.selectProject)
  const replaceProject = useStudioStore((state) => state.replaceProject)
  const [creating, setCreating] = useState(false)
  const [form, setForm] = useState({ title: '', genre: '', summary: '', targetAudience: '18–35 岁类型小说读者', style: '克制、具象、有画面感' })
  const [importing, setImporting] = useState(false)
  const [progress, setProgress] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  function openProject(id: string) { selectProject(id); onOpen() }
  function submitNew(event: React.FormEvent) {
    event.preventDefault()
    if (!form.title.trim()) return
    createProject({ title: form.title.trim(), genre: form.genre.trim(), summary: form.summary.trim() })
    setCreating(false)
    onOpen()
  }
  async function importFile(file?: File) {
    if (!file) return
    if (file.size > 5 * 1024 * 1024) { window.alert('文件不能超过 5MB'); return }
    setImporting(true)
    const text = await file.text()
    if (file.name.endsWith('.json')) {
      const parsed = parseProjectJson(text)
      if (!parsed.success) { setProgress(parsed.error); return }
      const imported = { ...parsed.data, id: crypto.randomUUID(), title: `${parsed.data.title}（导入）` }
      useStudioStore.setState((state) => ({ projects: [...state.projects, imported], selectedProjectId: imported.id }))
      setProgress('项目 JSON 已完整导入')
      return
    }
    const stages = ['识别人物', '分析世界', '重建时间线', '反推大纲', '生成关系']
    const chapters = splitImportedChapters(text)
    if (!chapters.length) { setProgress('没有识别到正文'); return }
    for (const stage of stages) { setProgress(stage); await new Promise((resolve) => setTimeout(resolve, 120)) }
    const analysis = analyzeImportedText(chapters)
    const id = crypto.randomUUID()
    const project = createDemoProject({
      id, title: file.name.replace(/\.(txt|md)$/i, ''), summary: `从 ${file.name} 导入并重建的作品`,
      chapters: chapters.map((chapter, index) => ({ id: `import-chapter-${index + 1}`, number: index + 1, title: chapter.title, summary: '', content: chapter.content, status: 'draft', locked: false, lockedPassages: [], scenes: [], updatedAt: new Date().toISOString() })),
      currentChapterId: 'import-chapter-1', ...analysis, snapshots: [], runs: [], reviews: [], graphPositions: {},
    })
    useStudioStore.setState((state) => ({ projects: [...state.projects, project], selectedProjectId: id }))
    setProgress(`完成：${chapters.length} 章，${analysis.characters.length} 个待确认角色`)
  }

  return (
    <div className="library-page">
      <div className="library-hero">
        <div><span className="eyebrow">PROJECT LIBRARY</span><h2>作品库</h2><p>从一个故事种子开始，或导入已有小说重建它的世界。</p></div>
        <div className="toolbar"><button className="secondary" onClick={() => fileRef.current?.click()}><FileUp size={17} /> 导入小说</button><button className="primary" onClick={() => setCreating(true)}><Plus size={17} /> 新建作品</button></div>
        <input ref={fileRef} hidden type="file" accept=".txt,.md,.json,text/plain,text/markdown,application/json" onChange={(event) => void importFile(event.target.files?.[0])} />
      </div>
      <div className="project-grid">
        {projects.map((project) => {
          const words = project.chapters.reduce((sum, chapter) => sum + chapter.content.length, 0)
          return <article className="project-card" key={project.id}>
            <div className="book-cover"><span>{project.genre.split('/')[0]}</span><BookOpen size={32} /></div>
            <div className="project-card-body"><div className="project-meta"><span>{project.status === 'writing' ? '创作中' : '规划中'}</span><span>{project.chapters.length} 章</span></div><h3>{project.title}</h3><p>{project.summary || '尚未填写作品简介'}</p><div className="progress-track"><i style={{ width: `${Math.min(100, words / Math.max(project.targetWords, 1) * 100)}%` }} /></div><small>{words.toLocaleString()} / {project.targetWords.toLocaleString()} 字</small></div>
            <footer><button className="primary compact" onClick={() => openProject(project.id)}>打开作品</button><button className="icon-button" aria-label={`复制${project.title}`} onClick={() => duplicateProject(project.id)}><Copy size={17} /></button><button className="icon-button" aria-label={`导出${project.title}`} onClick={() => downloadText(`${project.title}.json`, buildProjectJson(project), 'application/json')}><Download size={17} /></button><button className="icon-button danger" aria-label={`删除${project.title}`} onClick={() => window.confirm(`确认删除《${project.title}》？`) && deleteProject(project.id)}><Trash2 size={17} /></button></footer>
          </article>
        })}
        {!projects.length && <div className="empty-state"><BookOpen size={40} /><h3>还没有作品</h3><p>新建作品或导入 TXT、Markdown、项目 JSON。</p><button className="primary" onClick={() => setCreating(true)}>新建第一部作品</button></div>}
      </div>
      <Modal title="创建新作品" open={creating} onClose={() => setCreating(false)}>
        <form className="form-grid" onSubmit={submitNew}>
          <label>书名<input required value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} placeholder="例如：星海遗民" /></label>
          <label>题材<input value={form.genre} onChange={(e) => setForm({ ...form, genre: e.target.value })} placeholder="科幻 / 悬疑" /></label>
          <label className="span-2">故事简介<textarea value={form.summary} onChange={(e) => setForm({ ...form, summary: e.target.value })} placeholder="核心冲突与故事承诺" /></label>
          <label>目标读者<input value={form.targetAudience} onChange={(e) => setForm({ ...form, targetAudience: e.target.value })} /></label>
          <label>默认文风<input value={form.style} onChange={(e) => setForm({ ...form, style: e.target.value })} /></label>
          <div className="modal-actions span-2"><button type="button" className="secondary" onClick={() => setCreating(false)}>取消</button><button className="primary" type="submit">创建并进入</button></div>
        </form>
      </Modal>
      <Modal title="导入与重建" open={importing} onClose={() => { setImporting(false); if (progress.startsWith('完成') || progress.includes('已完整导入')) onOpen() }}>
        <div className="analysis-progress"><div className="spinner" /><h3>{progress || '准备读取文件'}</h3><p>模拟分析只用于验证交互，所有推断都会标记为“待作者确认”。</p></div>
        <div className="modal-actions"><button className="primary" onClick={() => { setImporting(false); if (progress.startsWith('完成') || progress.includes('已完整导入')) onOpen() }}>完成</button></div>
      </Modal>
    </div>
  )
}
