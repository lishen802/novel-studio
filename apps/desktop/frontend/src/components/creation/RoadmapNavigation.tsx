import { clsx } from 'clsx';

interface Node { id: string; parentId?: string; title: string; position: number }
export interface RoadmapItem<T extends Node> { id: string; title: string; kind: string; depth: number; node: T }

// 按真实父引用及同父顺序展开，不以标题判断节点身份或归属。
export function roadmapNavItems<T extends Node>(roadmap: {book:T;volumes:T[];arcs:T[];chapters:T[]}): RoadmapItem<T>[] {
  const items: RoadmapItem<T>[] = [];
  if (!roadmap.book.id) return items;
  const children = new Map<string, Array<{node:T;kind:string}>>();
  for (const [kind, nodes] of [['volume',roadmap.volumes],['arc',roadmap.arcs],['chapter',roadmap.chapters]] as const) {
    for (const node of nodes) {
      const parent = node.parentId ?? '';
      children.set(parent,[...(children.get(parent) ?? []),{node,kind}]);
    }
  }
  const visited = new Set<string>();
  function visit(node:T,kind:string,depth:number) {
    if (visited.has(node.id)) return;
    visited.add(node.id);
    items.push({id:node.id,title:kind==='chapter'?`近章 ${node.position} · ${node.title}`:node.title,kind,depth,node});
    for (const child of [...(children.get(node.id) ?? [])].sort((a,b)=>a.node.position-b.node.position || a.node.id.localeCompare(b.node.id))) visit(child.node,child.kind,depth+1);
  }
  visit(roadmap.book,'book',0);
  return items;
}

export function RoadmapNavigation<T extends Node>({items,selectedId,onSelect}:{items:RoadmapItem<T>[];selectedId?:string;onSelect:(id:string)=>void}) {
 return <nav aria-label="路线图层级">{items.map(item=><button key={item.id} aria-current={selectedId===item.id?'true':undefined}
   className={clsx('w-full text-left px-2 py-1.5 rounded-md text-sm mb-1',selectedId===item.id?'bg-primary-subtle text-primary':'text-text-sub hover:bg-surface-hover')}
   style={{paddingLeft:8+item.depth*12}} onClick={()=>onSelect(item.id)}>{item.title}</button>)}</nav>;
}
