import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, BookOpenText, ChevronDown, ChevronRight, FilePlus2, Lock, Plus, Save, Search, Trash2, Unlock } from 'lucide-react'
import { Modal } from '../../components/Modal'
import type { CanonFact, Character, Faction, Hook, Location, OutlineNode, Project, StoryItem } from '../../domain/types'

type Change = (project: Project) => void
const nextLevel: Record<OutlineNode['level'], OutlineNode['level'] | null> = { book: 'volume', volume: 'arc', arc: 'chapter', chapter: 'scene', scene: null }
const levelLabel = { book: '全书', volume: '分卷', arc: '故事弧', chapter: '章节', scene: '场景' }

function PageHeader({ eyebrow, title, description, action }: { eyebrow: string; title: string; description?: string; action?: React.ReactNode }) {
  return <header className="page-header"><div><span className="eyebrow">{eyebrow}</span><h2>{title}</h2>{description && <p>{description}</p>}</div>{action}</header>
}

export function ManuscriptPage({ project, onChange, toast }: { project: Project; onChange: Change; toast: (s: string) => void }) {
  const [selectedId, setSelectedId] = useState(project.currentChapterId || project.chapters[0]?.id || '')
  const chapter = project.chapters.find((item) => item.id === selectedId)
  useEffect(() => { if (!chapter && project.chapters[0]) setSelectedId(project.chapters[0].id) }, [chapter, project.chapters])
  function update(patch: Partial<NonNullable<typeof chapter>>) {
    if (!chapter) return
    onChange({ ...project, currentChapterId: chapter.id, chapters: project.chapters.map((item) => item.id === chapter.id ? { ...item, ...patch, updatedAt: new Date().toISOString() } : item) })
  }
  function addChapter() {
    const number = Math.max(0, ...project.chapters.map((item) => item.number)) + 1
    const id = crypto.randomUUID()
    onChange({ ...project, currentChapterId: id, chapters: [...project.chapters, { id, number, title: '未命名章节', summary: '', content: '', status: 'draft', locked: false, lockedPassages: [], scenes: [], updatedAt: new Date().toISOString() }] })
    setSelectedId(id); toast(`已创建第 ${number} 章`)
  }
  function deleteChapter() {
    if (!chapter || !window.confirm(`删除第${chapter.number}章？`)) return
    const chapters = project.chapters.filter((item) => item.id !== chapter.id)
    onChange({ ...project, chapters, currentChapterId: chapters[0]?.id ?? '' }); setSelectedId(chapters[0]?.id ?? '')
  }
  function addScene() {
    if (!chapter) return
    const id = crypto.randomUUID()
    update({ scenes: [...chapter.scenes, { id, title: `场景 ${chapter.scenes.length + 1}`, summary: '补充场景目标', content: '', storyTime: '待定', participantIds: [], goal: '', conflict: '', outcome: '' }] })
  }
  return <div className="page fill-page">
    <PageHeader eyebrow="MANUSCRIPT" title="正文与场景" description="章节、场景、锁定段落与正文使用同一份自动保存草稿。" action={<button className="primary" onClick={addChapter}><FilePlus2 size={16} /> 新建章节</button>} />
    <div className="authoring-layout">
      <aside className="chapter-list panel"><div className="list-search"><Search size={15} /><input aria-label="搜索章节" placeholder="搜索章节" /></div>{project.chapters.map((item) => <button key={item.id} className={selectedId === item.id ? 'selected' : ''} onClick={() => { setSelectedId(item.id); onChange({ ...project, currentChapterId: item.id }) }}><span>{String(item.number).padStart(2, '0')}</span><div><strong>{item.title}</strong><small>{item.content.length} 字 · {item.status}</small></div>{item.locked && <Lock size={12} />}</button>)}{!project.chapters.length && <div className="mini-empty">尚无章节</div>}</aside>
      <section className="editor-panel panel">
        {chapter ? <>
          <div className="editor-toolbar"><span className={`status-dot ${chapter.status}`} /> <select aria-label="章节状态" value={chapter.status} onChange={(e) => update({ status: e.target.value as typeof chapter.status })}><option value="planned">规划</option><option value="draft">草稿</option><option value="review">审稿中</option><option value="approved">已通过</option><option value="published">已发布</option></select><span className="toolbar-spacer" /><button className="secondary compact" onClick={() => update({ locked: !chapter.locked })}>{chapter.locked ? <Unlock size={15} /> : <Lock size={15} />}{chapter.locked ? '解锁章节' : '锁定章节'}</button><button className="icon-button danger" aria-label="删除当前章节" onClick={deleteChapter}><Trash2 size={16} /></button></div>
          <input className="chapter-title-input" aria-label="章节标题" value={chapter.title} disabled={chapter.locked} onChange={(e) => update({ title: e.target.value })} />
          <textarea className="manuscript-editor" aria-label="章节正文" value={chapter.content} disabled={chapter.locked} onChange={(e) => update({ content: e.target.value })} placeholder="在这里开始写作……" />
          <div className="editor-footer"><span>{chapter.content.length.toLocaleString()} 字</span><span>预计阅读 {Math.max(1, Math.ceil(chapter.content.length / 400))} 分钟</span><span className="saved"><Save size={13} /> 已保存</span></div>
        </> : <div className="empty-state"><BookOpenText /><h3>新建第一章</h3><button className="primary" onClick={addChapter}>新建章节</button></div>}
      </section>
      <aside className="scene-panel panel"><div className="panel-heading"><h3>场景</h3><button className="icon-button" aria-label="新增场景" onClick={addScene}><Plus size={16} /></button></div>{chapter?.scenes.map((scene, index) => <article key={scene.id}><small>SCENE {index + 1}</small><input aria-label={`${scene.title}标题`} value={scene.title} onChange={(e) => update({ scenes: chapter.scenes.map((item) => item.id === scene.id ? { ...item, title: e.target.value } : item) })} /><textarea aria-label={`${scene.title}摘要`} value={scene.summary} onChange={(e) => update({ scenes: chapter.scenes.map((item) => item.id === scene.id ? { ...item, summary: e.target.value } : item) })} /></article>)}{!chapter?.scenes.length && <div className="mini-empty">将章节拆成场景，控制 POV、地点、目标与冲突。</div>}</aside>
    </div>
  </div>
}

export function OutlinePage({ project, onChange, toast }: { project: Project; onChange: Change; toast: (s: string) => void }) {
  const [selectedId, setSelectedId] = useState(project.outlineNodes[0]?.id ?? '')
  const [collapsed, setCollapsed] = useState<string[]>([])
  const selected = project.outlineNodes.find((item) => item.id === selectedId)
  const visible = project.outlineNodes.filter((node) => {
    let parent = node.parentId
    while (parent) { if (collapsed.includes(parent)) return false; parent = project.outlineNodes.find((item) => item.id === parent)?.parentId ?? null }
    return true
  })
  function updateNode(patch: Partial<OutlineNode>) { if (selected) onChange({ ...project, outlineNodes: project.outlineNodes.map((node) => node.id === selected.id ? { ...node, ...patch } : node) }) }
  function addChild() {
    const parent = selected
    const level = parent ? nextLevel[parent.level] : 'book'
    if (!level) { toast('场景层级不能继续添加子级'); return }
    const title = window.prompt(`新增${levelLabel[level]}名称`, `新${levelLabel[level]}`)
    if (!title) return
    const node: OutlineNode = { id: crypto.randomUUID(), parentId: parent?.id ?? null, level, title, summary: '', order: project.outlineNodes.filter((item) => item.parentId === parent?.id).length + 1, status: 'planned', linkedEntityIds: [] }
    onChange({ ...project, outlineNodes: [...project.outlineNodes, node] }); setSelectedId(node.id); toast('已添加大纲节点')
  }
  function deleteNode() {
    if (!selected || !window.confirm('删除该节点及其全部子节点？')) return
    const deleting = new Set([selected.id]); let changed = true
    while (changed) { changed = false; project.outlineNodes.forEach((node) => { if (node.parentId && deleting.has(node.parentId) && !deleting.has(node.id)) { deleting.add(node.id); changed = true } }) }
    onChange({ ...project, outlineNodes: project.outlineNodes.filter((node) => !deleting.has(node.id)) }); setSelectedId('')
  }
  return <div className="page"><PageHeader eyebrow="STORY ARCHITECTURE" title="五级故事架构" description="全书 → 分卷 → 故事弧 → 章节 → 场景，结构修改会提示潜在影响。" action={<button className="primary" onClick={addChild}><Plus size={16} /> {selected ? '添加子级' : '添加全书'}</button>} />
    <div className="split-view"><section className="panel outline-tree">{visible.sort((a,b)=>a.order-b.order).map((node) => { const depth = ['book','volume','arc','chapter','scene'].indexOf(node.level); const hasChildren = project.outlineNodes.some((item)=>item.parentId===node.id); return <button key={node.id} className={selectedId === node.id ? 'selected' : ''} style={{ paddingLeft: 12 + depth * 22 }} onClick={() => setSelectedId(node.id)}>{hasChildren ? <span onClick={(e)=>{e.stopPropagation();setCollapsed(collapsed.includes(node.id)?collapsed.filter((id)=>id!==node.id):[...collapsed,node.id])}}>{collapsed.includes(node.id)?<ChevronRight size={14}/>:<ChevronDown size={14}/>}</span> : <i />}<em>{levelLabel[node.level]}</em><strong>{node.title}</strong><small>{node.status}</small></button>})}</section>
      <section className="panel detail-form">{selected ? <><div className="panel-heading"><div><span className="eyebrow">{levelLabel[selected.level]}</span><h3>节点详情</h3></div><button className="icon-button danger" aria-label="删除大纲节点" onClick={deleteNode}><Trash2 size={16}/></button></div><label>名称<input value={selected.title} onChange={(e)=>updateNode({title:e.target.value})}/></label><label>内容摘要<textarea value={selected.summary} onChange={(e)=>updateNode({summary:e.target.value})}/></label><label>状态<select value={selected.status} onChange={(e)=>updateNode({status:e.target.value as OutlineNode['status']})}><option value="planned">规划</option><option value="active">进行中</option><option value="completed">已完成</option></select></label><div className="impact-hint"><AlertTriangle size={17}/><div><strong>影响分析</strong><p>该节点关联 {selected.linkedEntityIds.length} 个故事实体。修改仅影响未来生成，不自动改写现有正文。</p></div></div></> : <div className="empty-state">选择一个大纲节点查看详情</div>}</section>
    </div>
  </div>
}

export function CharactersPage({ project, onChange, toast }: { project: Project; onChange: Change; toast: (s: string) => void }) {
  const [tab, setTab] = useState<'characters'|'factions'>('characters')
  const [selectedId, setSelectedId] = useState(project.characters[0]?.id ?? '')
  const items = tab === 'characters' ? project.characters : project.factions
  const selected = items.find((item)=>item.id===selectedId)
  function add() {
    if (tab === 'characters') { const character: Character = { id: crypto.randomUUID(), name:'新角色', role:'supporting', summary:'', personality:[], motivation:'', secrets:[], currentStatus:'待登场', factionIds:[], targetShare:10, arc:[], knowledge:[] }; onChange({...project,characters:[...project.characters,character]}); setSelectedId(character.id) }
    else { const faction: Faction={id:crypto.randomUUID(),name:'新势力',goal:'',resources:[],stance:'中立',memberIds:[]};onChange({...project,factions:[...project.factions,faction]});setSelectedId(faction.id) }
  }
  function update(patch: Record<string, unknown>) { onChange(tab==='characters'?{...project,characters:project.characters.map((item)=>item.id===selectedId?{...item,...patch}:item)}:{...project,factions:project.factions.map((item)=>item.id===selectedId?{...item,...patch}:item)}) }
  function remove() { if (!selected||!window.confirm(`删除“${selected.name}”？`))return; onChange(tab==='characters'?{...project,characters:project.characters.filter((item)=>item.id!==selectedId)}:{...project,factions:project.factions.filter((item)=>item.id!==selectedId)});setSelectedId('');toast('已删除并保留相关引用供后续处理') }
  return <div className="page"><PageHeader eyebrow="CAST & FACTIONS" title="角色与势力" description="管理角色弧、戏份目标、势力归属、秘密与知识边界。" action={<button className="primary" onClick={add}><Plus size={16}/>新增{tab==='characters'?'角色':'势力'}</button>}/><div className="tabs"><button className={tab==='characters'?'active':''} onClick={()=>{setTab('characters');setSelectedId(project.characters[0]?.id??'')}}>角色 {project.characters.length}</button><button className={tab==='factions'?'active':''} onClick={()=>{setTab('factions');setSelectedId(project.factions[0]?.id??'')}}>势力 {project.factions.length}</button></div><div className="split-view entity-view"><section className="panel card-list">{items.map((item)=><button key={item.id} className={selectedId===item.id?'selected':''} onClick={()=>setSelectedId(item.id)}><span className="entity-avatar">{item.name[0]}</span><div><strong>{item.name}</strong><small>{'role' in item ? item.role : item.stance}</small></div></button>)}</section><section className="panel detail-form">{selected ? <><div className="panel-heading"><h3>编辑{tab==='characters'?'角色':'势力'}</h3><button className="icon-button danger" aria-label="删除实体" onClick={remove}><Trash2 size={16}/></button></div><label>名称<input value={selected.name} onChange={(e)=>update({name:e.target.value})}/></label>{'role' in selected ? <><label>角色定位<select value={selected.role} onChange={(e)=>update({role:e.target.value})}><option value="protagonist">主角</option><option value="supporting">主要配角</option><option value="antagonist">对手</option><option value="minor">次要角色</option></select></label><label>人物摘要<textarea value={selected.summary} onChange={(e)=>update({summary:e.target.value})}/></label><label>核心动机<textarea value={selected.motivation} onChange={(e)=>update({motivation:e.target.value})}/></label><label>目标戏份占比<input type="range" min="0" max="100" value={selected.targetShare} onChange={(e)=>update({targetShare:Number(e.target.value)})}/><span>{selected.targetShare}%</span></label><label>势力归属<select multiple value={selected.factionIds} onChange={(e)=>update({factionIds:Array.from(e.target.selectedOptions).map((option)=>option.value)})}>{project.factions.map((faction)=><option key={faction.id} value={faction.id}>{faction.name}</option>)}</select></label></> : <><label>目标<textarea value={selected.goal} onChange={(e)=>update({goal:e.target.value})}/></label><label>立场<input value={selected.stance} onChange={(e)=>update({stance:e.target.value})}/></label><label>资源<input value={selected.resources.join('、')} onChange={(e)=>update({resources:e.target.value.split(/[、,，]/).filter(Boolean)})}/></label></>}</> : <div className="empty-state">选择实体查看详情</div>}</section></div></div>
}

export function WorldPage({ project, onChange }: { project: Project; onChange: Change }) {
  const [tab,setTab]=useState<'locations'|'items'|'hooks'>('locations')
  const [selectedId,setSelectedId]=useState(project.locations[0]?.id??'')
  const items = tab==='locations'?project.locations:tab==='items'?project.items:project.hooks
  const selected=items.find((item)=>item.id===selectedId)
  function switchTab(next: typeof tab){setTab(next);const list=next==='locations'?project.locations:next==='items'?project.items:project.hooks;setSelectedId(list[0]?.id??'')}
  function update(patch:Record<string,unknown>){if(tab==='locations')onChange({...project,locations:project.locations.map((item)=>item.id===selectedId?{...item,...patch}:item)});else if(tab==='items')onChange({...project,items:project.items.map((item)=>item.id===selectedId?{...item,...patch}:item)});else onChange({...project,hooks:project.hooks.map((item)=>item.id===selectedId?{...item,...patch}:item)})}
  function add(){if(tab==='locations'){const item:Location={id:crypto.randomUUID(),name:'新地点',summary:'',region:'未分类',x:500,y:350,visitChapterIds:[],eventIds:[]};onChange({...project,locations:[...project.locations,item]});setSelectedId(item.id)}else if(tab==='items'){const item:StoryItem={id:crypto.randomUUID(),name:'新物品',status:'未知',origin:'',transfers:[]};onChange({...project,items:[...project.items,item]});setSelectedId(item.id)}else{const item:Hook={id:crypto.randomUUID(),title:'新伏笔',plantedChapterId:project.chapters[0]?.id??'',progressChapterIds:[],status:'planted',evidence:[]};onChange({...project,hooks:[...project.hooks,item]});setSelectedId(item.id)}}
  return <div className="page"><PageHeader eyebrow="WORLD BIBLE" title="世界设定" description="地点、物品流转、伏笔回收和世界规则共同组成可追溯的设定库。" action={<button className="primary" onClick={add}><Plus size={16}/>新增</button>}/><div className="tabs"><button className={tab==='locations'?'active':''} onClick={()=>switchTab('locations')}>地点</button><button className={tab==='items'?'active':''} onClick={()=>switchTab('items')}>物品</button><button className={tab==='hooks'?'active':''} onClick={()=>switchTab('hooks')}>伏笔</button></div><div className="split-view"><section className="panel card-list">{items.map((item)=><button key={item.id} className={selectedId===item.id?'selected':''} onClick={()=>setSelectedId(item.id)}><span className="entity-avatar">{'name' in item?item.name[0]:item.title[0]}</span><div><strong>{'name' in item?item.name:item.title}</strong><small>{'region' in item?item.region:'status' in item?item.status:''}</small></div></button>)}</section><section className="panel detail-form">{selected && 'region' in selected ? <><h3>地点设定</h3><label>名称<input value={selected.name} onChange={(e)=>update({name:e.target.value})}/></label><label>区域<input value={selected.region} onChange={(e)=>update({region:e.target.value})}/></label><label>说明<textarea value={selected.summary} onChange={(e)=>update({summary:e.target.value})}/></label><p className="muted">地图坐标：{selected.x}, {selected.y}</p></> : selected && 'origin' in selected ? <><h3>物品设定</h3><label>名称<input value={selected.name} onChange={(e)=>update({name:e.target.value})}/></label><label>状态<input value={selected.status} onChange={(e)=>update({status:e.target.value})}/></label><label>来源<textarea value={selected.origin} onChange={(e)=>update({origin:e.target.value})}/></label><button className="secondary" onClick={()=>update({transfers:[...selected.transfers,{chapterId:project.currentChapterId,toEntityId:project.characters[0]?.id}]})}>新增流转记录</button></> : selected ? <><h3>伏笔管理</h3><label>伏笔<input value={selected.title} onChange={(e)=>update({title:e.target.value})}/></label><label>状态<select value={selected.status} onChange={(e)=>update({status:e.target.value})}><option value="planted">已埋设</option><option value="developing">推进中</option><option value="paid-off">已回收</option><option value="overdue">逾期</option></select></label><label>回收截止章节<input type="number" value={selected.deadlineChapter??''} onChange={(e)=>update({deadlineChapter:Number(e.target.value)||undefined})}/></label><label>证据<textarea value={selected.evidence.join('\n')} onChange={(e)=>update({evidence:e.target.value.split('\n')})}/></label></> : <div className="empty-state">选择一项世界资产</div>}</section></div></div>
}

export function CanonPage({ project,onChange,toast}:{project:Project;onChange:Change;toast:(s:string)=>void}){
  const [filter,setFilter]=useState('all'); const facts=project.facts.filter((fact)=>filter==='all'||fact.status===filter)
  function setStatus(id:string,status:CanonFact['status']){onChange({...project,facts:project.facts.map((fact)=>fact.id===id?{...fact,status}:fact)});toast(status==='author-confirmed'?'已提升为作者确认事实':'事实状态已更新')}
  function add(){const text=window.prompt('输入新事实');if(!text)return;onChange({...project,facts:[...project.facts,{id:crypto.randomUUID(),text,category:'world-rule',status:'author-confirmed',confidence:1,evidence:[],scopeEntityIds:[]}]})}
  return <div className="page"><PageHeader eyebrow="CANON & KNOWLEDGE" title="事实与知识边界" description="AI 推断不会自动成为正式事实；每条事实都保留来源证据和适用范围。" action={<button className="primary" onClick={add}><Plus size={16}/>作者新增事实</button>}/><div className="filter-bar"><select aria-label="事实状态筛选" value={filter} onChange={(e)=>setFilter(e.target.value)}><option value="all">全部状态</option><option value="author-confirmed">作者确认</option><option value="text-evidenced">文本证据</option><option value="ai-inferred">AI 推断</option><option value="conflicted">冲突</option><option value="rejected">已拒绝</option></select><span>{facts.length} 条事实</span></div><div className="fact-grid">{facts.map((fact)=><article className={`fact-card ${fact.status}`} key={fact.id}><header><span className="status-pill">{fact.status}</span><b>{Math.round(fact.confidence*100)}%</b></header><h3>{fact.text}</h3><small>{fact.category}</small>{fact.evidence.map((evidence)=><blockquote key={evidence.chapterId}>“{evidence.quote}”</blockquote>)}<footer>{fact.status!=='author-confirmed'&&<button className="primary compact" onClick={()=>setStatus(fact.id,'author-confirmed')}>确认为正式事实</button>}{fact.status!=='rejected'&&<button className="secondary compact" onClick={()=>setStatus(fact.id,'rejected')}>拒绝</button>}<button className="text-button" onClick={()=>setStatus(fact.id,'conflicted')}>标记冲突</button></footer></article>)}</div></div>
}

export function SettingsPage({project,onChange,toast}:{project:Project;onChange:Change;toast:(s:string)=>void}){
  const [draft,setDraft]=useState(project.writingPreferences); const [impact,setImpact]=useState(false)
  useEffect(()=>setDraft(project.writingPreferences),[project.id,project.writingPreferences])
  function apply(mode:'future'|'branch'|'all'){const branches=mode==='branch'?[...project.branches,{id:crypto.randomUUID(),name:`文风调整 ${project.branches.length}`}]:project.branches;onChange({...project,writingPreferences:draft,writingPreferenceRevisionId:crypto.randomUUID(),branches});setImpact(false);toast(mode==='future'?'新文风仅用于未来生成':mode==='branch'?'已创建分支并应用文风':'已更新全部规划约束')}
  return <div className="page"><PageHeader eyebrow="PROJECT SETTINGS" title="作品设置与作者控制" description="调整题材、视角、文风、节奏与禁用表达；不会自动改写已有正文。"/><div className="settings-grid"><section className="panel detail-form"><h3>创作简报</h3><label>目标读者<input value={draft.targetAudience} onChange={(e)=>setDraft({...draft,targetAudience:e.target.value})}/></label><label>叙事人称<select value={draft.narrativePerson} onChange={(e)=>setDraft({...draft,narrativePerson:e.target.value as typeof draft.narrativePerson})}><option value="first">第一人称</option><option value="third-limited">第三人称限知</option><option value="third-omniscient">第三人称全知</option></select></label><label>POV 策略<input value={draft.povStrategy} onChange={(e)=>setDraft({...draft,povStrategy:e.target.value})}/></label><label>语气<input value={draft.tone} onChange={(e)=>setDraft({...draft,tone:e.target.value})}/></label><label>节奏<select value={draft.pacing} onChange={(e)=>setDraft({...draft,pacing:e.target.value as typeof draft.pacing})}><option value="slow">舒缓</option><option value="balanced">均衡</option><option value="fast">快速</option></select></label><label>文风说明<textarea value={draft.styleGuide} onChange={(e)=>setDraft({...draft,styleGuide:e.target.value})}/></label><label>禁用表达<textarea value={draft.forbiddenPatterns.join('\n')} onChange={(e)=>setDraft({...draft,forbiddenPatterns:e.target.value.split('\n').filter(Boolean)})}/></label><button className="primary" onClick={()=>setImpact(true)}>分析并保存</button></section><section className="panel brief-preview"><span className="eyebrow">GENERATION BRIEF</span><h3>给智能体的结构化约束</h3><dl><dt>受众</dt><dd>{draft.targetAudience}</dd><dt>视角</dt><dd>{draft.narrativePerson} · {draft.povStrategy}</dd><dt>语气与节奏</dt><dd>{draft.tone} · {draft.pacing}</dd><dt>文风</dt><dd>{draft.styleGuide}</dd><dt>避免</dt><dd>{draft.forbiddenPatterns.join('、')}</dd></dl><p className="impact-hint">当前修订：{project.writingPreferenceRevisionId.slice(0,8)}</p></section></div><Modal title="修改影响" open={impact} onClose={()=>setImpact(false)}><div className="impact-dialog"><AlertTriangle/><div><h3>文风与 POV 发生变化</h3><p>可能影响未来章节规划、人物语气和节奏审查。已有正文不会被自动改写。</p></div></div><div className="choice-list"><button onClick={()=>apply('future')}><strong>仅影响未来章节</strong><span>保留已有正文和规划</span></button><button onClick={()=>apply('branch')}><strong>创建新分支</strong><span>保留当前方案用于比较</span></button><button onClick={()=>apply('all')}><strong>应用到全部规划</strong><span>更新未完成章节约束</span></button></div></Modal></div>
}
