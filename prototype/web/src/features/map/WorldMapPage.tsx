import { BookOpen, ChevronRight, Compass, Flag, ListTree, Map as MapIcon, MapPin, Plus, Trash2, Users } from 'lucide-react'
import { useState } from 'react'
import type { Project, WorldRegion } from '../../domain/types'
import { canDeleteRegion, canMoveRegion, getRegionChildren, getRegionPath } from './worldRegionModel'

type Props = {
  project: Project
  onChange: (project: Project) => void
  toast: (message: string) => void
}

const typeLabels: Record<WorldRegion['type'], string> = {
  world: '世界', continent: '大陆', sea: '海域', state: '州域', island: '群岛', city: '城池',
  stronghold: '势力据点', terrain: '地貌', 'secret-realm': '秘境 / 禁区', resource: '资源点',
}

const directionLabels: Record<WorldRegion['direction'], string> = {
  north: '北', 'north-east': '东北', east: '东', 'south-east': '东南', south: '南', 'south-west': '西南', west: '西', 'north-west': '西北', center: '中部', surrounding: '环绕',
}

function RegionTree({ regions, parentId, selectedId, onSelect }: { regions: WorldRegion[]; parentId: string | null; selectedId: string; onSelect: (id: string) => void }) {
  return <>{getRegionChildren(regions, parentId).map((region) => {
    const children = getRegionChildren(regions, region.id)
    return <div className="world-tree-branch" key={region.id}>
      <button type="button" role="treeitem" aria-label={`${region.name} ${typeLabels[region.type]}`} aria-level={getRegionPath(regions, region.id).length} aria-expanded={children.length ? true : undefined} aria-selected={selectedId === region.id} className={selectedId === region.id ? 'selected' : ''} onClick={() => onSelect(region.id)}>
        <span className={`region-type-icon ${region.type}`}>{region.name[0]}</span><span><strong>{region.name}</strong><small>{typeLabels[region.type]}</small></span>{children.length > 0 && <ChevronRight size={14} />}
      </button>
      {children.length > 0 && <div role="group"><RegionTree regions={regions} parentId={region.id} selectedId={selectedId} onSelect={onSelect} /></div>}
    </div>
  })}</>
}

function LinkedGroup({ icon, title, items }: { icon: React.ReactNode; title: string; items: string[] }) {
  if (!items.length) return null
  return <section className="region-linked-group"><h4>{icon}{title}<span>{items.length}</span></h4><div className="chips">{items.map((item) => <i key={item}>{item}</i>)}</div></section>
}

function RegionInspector({ project, selected, onEnter, onUpdate, onDelete, toast }: { project: Project; selected?: WorldRegion; onEnter: (region: WorldRegion) => void; onUpdate: (patch: Partial<WorldRegion>) => void; onDelete: () => void; toast: (message: string) => void }) {
  if (!selected) return <aside className="panel world-region-inspector"><div className="mini-empty">选择区域查看设定</div></aside>
  const children = getRegionChildren(project.worldRegions, selected.id)
  const parentOptions = project.worldRegions.filter((region) => canMoveRegion(project.worldRegions, selected.id, region.id))
  const siblingDuplicate = project.worldRegions.some((region) => region.id !== selected.id && region.parentId === selected.parentId && region.name.trim() === selected.name.trim())
  const locations = project.locations.filter((item) => selected.locationIds.includes(item.id)).map((item) => item.name)
  const factions = project.factions.filter((item) => selected.factionIds.includes(item.id)).map((item) => item.name)
  const characters = project.characters.filter((item) => selected.characterIds.includes(item.id)).map((item) => item.name)
  const events = project.events.filter((item) => selected.eventIds.includes(item.id)).map((item) => item.title)
  const chapters = project.chapters.filter((item) => selected.chapterIds.includes(item.id)).map((item) => `第${item.number}章 ${item.title}`)
  const facts = project.facts.filter((item) => selected.factIds.includes(item.id)).map((item) => item.text)
  return <aside className="panel world-region-inspector">
    <div className="region-inspector-title"><span className={`region-type-icon ${selected.type}`}>{selected.name[0]}</span><div><span className="eyebrow">{typeLabels[selected.type]}</span><h3>{selected.name}</h3></div></div>
    <div className="region-status-row"><span className={`status-pill ${selected.canonStatus}`}>{selected.canonStatus}</span><span>{directionLabels[selected.direction]}</span></div>
    <div className="region-edit-grid">
      <label>区域名称<input value={selected.name} onChange={(event) => onUpdate({ name: event.target.value })} onBlur={() => { if (!selected.name.trim()) { onUpdate({ name: '未命名区域' }); toast('区域名称不能为空') } }} /></label>
      <label>区域类型<select value={selected.type} onChange={(event) => onUpdate({ type: event.target.value as WorldRegion['type'] })}>{Object.entries(typeLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label>相对方位<select value={selected.direction} onChange={(event) => onUpdate({ direction: event.target.value as WorldRegion['direction'] })}>{Object.entries(directionLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      <label>上级区域<select value={selected.parentId ?? ''} onChange={(event) => onUpdate({ parentId: event.target.value || null })}><option value="">无上级</option>{parentOptions.map((region) => <option key={region.id} value={region.id}>{getRegionPath(project.worldRegions, region.id).map((item) => item.name).join(' / ')}</option>)}</select></label>
      <label className="span-2">区域简介<textarea value={selected.summary} onChange={(event) => onUpdate({ summary: event.target.value })} /></label>
      <label>地形<input value={selected.terrain} onChange={(event) => onUpdate({ terrain: event.target.value })} /></label>
      <label>气候<input value={selected.climate} onChange={(event) => onUpdate({ climate: event.target.value })} /></label>
      <label className="span-2">特殊规则<textarea value={selected.rules} onChange={(event) => onUpdate({ rules: event.target.value })} /></label>
      <label className="span-2">通行说明<textarea value={selected.travelNotes} onChange={(event) => onUpdate({ travelNotes: event.target.value })} /></label>
      <label>信息来源<select value={selected.canonStatus} onChange={(event) => onUpdate({ canonStatus: event.target.value as WorldRegion['canonStatus'] })}><option value="text-evidenced">正文已确认</option><option value="author-confirmed">作者设定</option><option value="ai-inferred">AI 推测</option><option value="conflicted">存在冲突</option></select></label>
    </div>
    {siblingDuplicate && <p className="inline-warning">同一上级下存在同名区域，请确认是否为不同时代或政权。</p>}
    {children.length > 0 && <><div className="panel-heading compact-heading"><h4>下级区域</h4><span>{children.length}</span></div><div className="region-child-list">{children.map((child) => <button type="button" key={child.id} onClick={() => onEnter(child)}><span>{child.name}</span><small>{typeLabels[child.type]}</small><ChevronRight size={14} /></button>)}</div><button className="primary enter-region" type="button" aria-label={`进入${selected.name}`} onClick={() => onEnter(selected)}>进入区域<ChevronRight size={15} /></button></>}
    <div className="region-linked-sections"><LinkedGroup icon={<MapPin size={14} />} title="主要地点" items={locations} /><LinkedGroup icon={<Flag size={14} />} title="相关势力" items={factions} /><LinkedGroup icon={<Users size={14} />} title="活跃角色" items={characters} /><LinkedGroup icon={<Compass size={14} />} title="相关事件" items={events} /><LinkedGroup icon={<BookOpen size={14} />} title="关联章节" items={chapters} /><LinkedGroup icon={<BookOpen size={14} />} title="设定事实" items={facts} /></div>
    <button type="button" className="secondary danger region-delete" onClick={onDelete}><Trash2 size={14} />删除区域</button>
  </aside>
}

export function WorldMapPage({ project, onChange, toast }: Props) {
  const root = project.worldRegions.find((region) => region.type === 'world')
  const validFocus = project.worldRegions.find((region) => region.id === project.worldMapState.focusRegionId)
  const focus = validFocus ?? root
  const [selectedId, setSelectedId] = useState(focus?.id ?? '')
  const selected = project.worldRegions.find((region) => region.id === selectedId) ?? focus
  const visibleRegions = focus ? getRegionChildren(project.worldRegions, focus.id) : []
  const breadcrumbs = focus ? getRegionPath(project.worldRegions, focus.id) : []

  const setMode = (mode: 'map' | 'tree') => onChange({ ...project, worldMapState: { ...project.worldMapState, mode } })
  const enter = (region: WorldRegion) => {
    if (!getRegionChildren(project.worldRegions, region.id).length) return
    setSelectedId(region.id)
    onChange({ ...project, worldMapState: { ...project.worldMapState, focusRegionId: region.id } })
  }
  const navigate = (region: WorldRegion) => {
    setSelectedId(region.id)
    onChange({ ...project, worldMapState: { ...project.worldMapState, focusRegionId: region.id } })
  }
  const updateSelected = (patch: Partial<WorldRegion>) => {
    if (!selected) return
    if (patch.parentId !== undefined && !canMoveRegion(project.worldRegions, selected.id, patch.parentId)) {
      toast('不能把区域移动到自身或下级区域中')
      return
    }
    onChange({ ...project, worldRegions: project.worldRegions.map((region) => region.id === selected.id ? { ...region, ...patch } : region) })
  }
  const addRegion = () => {
    if (!focus) return
    const name = window.prompt('区域名称', '新区域')?.trim()
    if (!name) return
    const siblings = getRegionChildren(project.worldRegions, focus.id)
    const inferredType: WorldRegion['type'] = focus.type === 'world' ? 'continent' : focus.type === 'sea' ? 'island' : focus.type === 'continent' ? 'state' : 'city'
    const id = crypto.randomUUID()
    const column = siblings.length % 3
    const row = Math.floor(siblings.length / 3)
    const region: WorldRegion = {
      id, parentId: focus.id, name, type: inferredType, direction: 'center', order: siblings.length + 1,
      summary: '', terrain: '', climate: '', rules: '', canonStatus: 'author-confirmed', adjacentRegionIds: [], travelNotes: '',
      locationIds: [], factionIds: [], characterIds: [], eventIds: [], chapterIds: [], factIds: [],
      layout: { x: 80 + column * 300, y: 100 + row * 240, width: 250, height: 180 },
    }
    setSelectedId(id)
    onChange({ ...project, worldRegions: [...project.worldRegions, region] })
    toast(`已在${focus.name}下创建${name}`)
  }
  const deleteSelected = () => {
    if (!selected) return
    if (!canDeleteRegion(project.worldRegions, selected.id)) {
      toast('请先移动或删除下级区域')
      return
    }
    if (!window.confirm(`确认删除“${selected.name}”？`)) return
    const nextSelection = selected.parentId ?? root?.id ?? ''
    setSelectedId(nextSelection)
    onChange({ ...project, worldRegions: project.worldRegions.filter((region) => region.id !== selected.id) })
    toast('区域已删除')
  }
  const createWorldRoot = () => {
    const id = crypto.randomUUID()
    const world: WorldRegion = {
      id, parentId: null, name: '新世界', type: 'world', direction: 'center', order: 1,
      summary: '从这里开始构建大陆、海域与故事地点。', terrain: '', climate: '', rules: '', canonStatus: 'author-confirmed', adjacentRegionIds: [], travelNotes: '',
      locationIds: [], factionIds: [], characterIds: [], eventIds: [], chapterIds: [], factIds: [],
      layout: { x: 0, y: 0, width: 1000, height: 700 },
    }
    setSelectedId(id)
    onChange({ ...project, worldRegions: [world], worldMapState: { mode: 'map', focusRegionId: id } })
    toast('世界根节点已创建')
  }

  return <div className="page visualization-page">
    <header className="page-header"><div><span className="eyebrow">WORLD STRUCTURE</span><h2>层级方位世界地图</h2><p>用层级、方位和故事关联描述世界，不依赖真实地图或图片底图。</p></div><div className="toolbar"><button type="button" className={project.worldMapState.mode === 'map' ? 'primary' : 'secondary'} onClick={() => setMode('map')}><MapIcon size={16} />方位地图</button><button type="button" className={project.worldMapState.mode === 'tree' ? 'primary' : 'secondary'} onClick={() => setMode('tree')}><ListTree size={16} />层级结构</button><button type="button" className="secondary" onClick={addRegion}><Plus size={16} />新增区域</button></div></header>
    {root ? <>
      <nav className="world-map-breadcrumbs" aria-label="地图层级">{breadcrumbs.map((region, index) => <span key={region.id}><button type="button" onClick={() => navigate(region)}>{region.name}</button>{index < breadcrumbs.length - 1 && <ChevronRight size={13} />}</span>)}</nav>
      <div className="world-map-shell">
        {project.worldMapState.mode === 'map' ? <section className="panel world-map-surface" aria-label={`${focus?.name ?? '世界'}方位地图`}>
          <div className="world-map-compass"><Compass size={17} /><span>北</span></div>
          <div className="world-map-caption"><strong>{focus?.name}</strong><span>{visibleRegions.length} 个下级区域 · 示意方位，非真实比例</span></div>
          {visibleRegions.map((region) => <button type="button" aria-label={region.name} key={region.id} className={`world-region-block ${region.type} ${selected?.id === region.id ? 'selected' : ''}`} style={{ left: `${region.layout.x / 10}%`, top: `${region.layout.y / 7}%`, width: `${region.layout.width / 10}%`, height: `${region.layout.height / 7}%` }} onClick={() => setSelectedId(region.id)} onDoubleClick={() => enter(region)}><span className="region-kind">{typeLabels[region.type]}</span><strong>{region.name}</strong><small>{directionLabels[region.direction]}</small></button>)}
          {!visibleRegions.length && <div className="mini-empty map-empty">当前区域没有下级划分</div>}
        </section> : <section className="panel world-tree" role="tree" aria-label="世界层级结构"><RegionTree regions={project.worldRegions} parentId={null} selectedId={selected?.id ?? ''} onSelect={setSelectedId} /></section>}
        <RegionInspector project={project} selected={selected} onEnter={enter} onUpdate={updateSelected} onDelete={deleteSelected} toast={toast} />
      </div>
    </> : <div className="empty-state"><Compass size={42} /><h3>还没有世界结构</h3><p>创建世界根节点后，可以继续划分大陆、海域、州域与地点。</p><button type="button" className="primary" onClick={createWorldRoot}>创建世界根节点</button></div>}
  </div>
}
