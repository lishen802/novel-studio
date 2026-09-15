import {render,screen,fireEvent,cleanup} from '@testing-library/react';
import {afterEach,expect,it,vi} from 'vitest';
import {roadmapNavItems,RoadmapNavigation} from './RoadmapNavigation';
afterEach(cleanup);
it('同名卷和章节以稳定 ID 选择，并按真实父子顺序排列',()=>{
 const n=(id:string,parentId:string,title:string,position=1)=>({id,parentId,title,position});
 const items=roadmapNavItems({book:n('b','','全书'),volumes:[n('v2','b','同名卷',2),n('v1','b','同名卷')],arcs:[n('a2','v2','第二弧'),n('a1','v1','第一弧')],chapters:[n('c2','a2','同名章节'),n('c1','a1','同名章节')]});
 expect(items.map(i=>i.id)).toEqual(['b','v1','a1','c1','v2','a2','c2']);
 const select=vi.fn();render(<RoadmapNavigation items={items} selectedId="v2" onSelect={select}/>);
 fireEvent.click(screen.getAllByRole('button',{name:'同名卷'})[1]);expect(select).toHaveBeenCalledWith('v2');
 fireEvent.click(screen.getAllByRole('button',{name:'近章 1 · 同名章节'})[1]);expect(select).toHaveBeenLastCalledWith('c2');
});
