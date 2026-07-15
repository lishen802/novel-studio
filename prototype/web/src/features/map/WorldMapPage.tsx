import { ChevronRight, Compass, ListTree, Map as MapIcon, Plus } from 'lucide-react'
import { useState } from 'react'
import type { Project, WorldRegion } from '../../domain/types'
import { getRegionChildren, getRegionPath } from './worldRegionModel'

type Props = {
  project: Project
  onChange: (project: Project) => void
  toast: (message: string) => void
}

const typeLabels: Record<WorldRegion['type'], string> = {
  world: '世界', continent: '大陆', sea: '海域', state: '州域', island: '群岛', city: '城池',
  stronghold: '势力据点', terrain: '地貌', 'secret-realm': '秘境 / 禁区', resource: '资源点',
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

function RegionInspector({ project, selected, onEnter }: { project: Project; selected?: WorldRegion; onEnter: (region: WorldRegion) => void }) {
  if (!selected) return <aside className="panel world-region-inspector"><div className="mini-empty">选择区域查看设定</div></aside>
  const children = getRegionChildren(project.worldRegions, selected.id)
  return <aside className="panel world-region-inspector">
    <div className="region-inspector-title"><span className={`region-type-icon ${selected.type}`}>{selected.name[0]}</span><div><span className="eyebrow">{typeLabels[selected.type]}</span><h3>{selected.name}</h3></div></div>
    <div className="region-status-row"><span className={`status-pill ${selected.canonStatus}`}>{selected.canonStatus}</span><span>{selected.direction}</span></div>
    <p className="region-summary">{selected.summary || '尚未补充区域简介。'}</p>
    <dl className="region-facts"><dt>地形</dt><dd>{selected.terrain || '未设定'}</dd><dt>气候</dt><dd>{selected.climate || '未设定'}</dd><dt>规则</dt><dd>{selected.rules || '无特殊规则'}</dd><dt>通行</dt><dd>{selected.travelNotes || '未记录'}</dd></dl>
    {children.length > 0 && <><div className="panel-heading compact-heading"><h4>下级区域</h4><span>{children.length}</span></div><div className="region-child-list">{children.map((child) => <button type="button" key={child.id} onClick={() => onEnter(child)}><span>{child.name}</span><small>{typeLabels[child.type]}</small><ChevronRight size={14} /></button>)}</div><button className="primary enter-region" type="button" aria-label={`进入${selected.name}`} onClick={() => onEnter(selected)}>进入区域<ChevronRight size={15} /></button></>}
  </aside>
}

export function WorldMapPage({ project, onChange }: Props) {
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

  return <div className="page visualization-page">
    <header className="page-header"><div><span className="eyebrow">WORLD STRUCTURE</span><h2>层级方位世界地图</h2><p>用层级、方位和故事关联描述世界，不依赖真实地图或图片底图。</p></div><div className="toolbar"><button type="button" className={project.worldMapState.mode === 'map' ? 'primary' : 'secondary'} onClick={() => setMode('map')}><MapIcon size={16} />方位地图</button><button type="button" className={project.worldMapState.mode === 'tree' ? 'primary' : 'secondary'} onClick={() => setMode('tree')}><ListTree size={16} />层级结构</button><button type="button" className="secondary"><Plus size={16} />新增区域</button></div></header>
    {root ? <>
      <nav className="world-map-breadcrumbs" aria-label="地图层级">{breadcrumbs.map((region, index) => <span key={region.id}><button type="button" onClick={() => navigate(region)}>{region.name}</button>{index < breadcrumbs.length - 1 && <ChevronRight size={13} />}</span>)}</nav>
      <div className="world-map-shell">
        {project.worldMapState.mode === 'map' ? <section className="panel world-map-surface" aria-label={`${focus?.name ?? '世界'}方位地图`}>
          <div className="world-map-compass"><Compass size={17} /><span>北</span></div>
          <div className="world-map-caption"><strong>{focus?.name}</strong><span>{visibleRegions.length} 个下级区域 · 示意方位，非真实比例</span></div>
          {visibleRegions.map((region) => <button type="button" aria-label={region.name} key={region.id} className={`world-region-block ${region.type} ${selected?.id === region.id ? 'selected' : ''}`} style={{ left: `${region.layout.x / 10}%`, top: `${region.layout.y / 7}%`, width: `${region.layout.width / 10}%`, height: `${region.layout.height / 7}%` }} onClick={() => setSelectedId(region.id)} onDoubleClick={() => enter(region)}><span className="region-kind">{typeLabels[region.type]}</span><strong>{region.name}</strong><small>{region.direction}</small></button>)}
          {!visibleRegions.length && <div className="mini-empty map-empty">当前区域没有下级划分</div>}
        </section> : <section className="panel world-tree" role="tree" aria-label="世界层级结构"><RegionTree regions={project.worldRegions} parentId={null} selectedId={selected?.id ?? ''} onSelect={setSelectedId} /></section>}
        <RegionInspector project={project} selected={selected} onEnter={enter} />
      </div>
    </> : <div className="empty-state"><Compass size={42} /><h3>还没有世界结构</h3><p>创建世界根节点后，可以继续划分大陆、海域、州域与地点。</p><button type="button" className="primary">创建世界根节点</button></div>}
  </div>
}
