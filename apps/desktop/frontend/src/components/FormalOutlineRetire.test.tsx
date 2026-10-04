import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { FormalOutlineRetire, type FormalOutlineRetireAPI } from './FormalOutlineRetire';
import type { ConstraintView, FormalOutlineRetirePreview } from '../api/studio';

const request={projectId:'project',branchId:'main',assetId:'new-asset',chapterId:'later',reason:'后续目标调整'};
const preview:FormalOutlineRetirePreview={request,digest:'frozen',nodeId:'new',title:'旧目标',introducedAt:2,position:3};
const api=():FormalOutlineRetireAPI=>({listChapters:vi.fn().mockResolvedValue([{id:'later',projectId:'project',title:'第三章',position:3,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}]),planningRevisions:vi.fn().mockResolvedValue([{assetId:'new-asset',kind:'outline_node',typedId:'new'} as ConstraintView]),formalOutlineReferences:vi.fn().mockResolvedValue({assetId:'new-asset',nodeId:'new',title:'旧目标',references:[]}),previewFormalOutlineRetire:vi.fn().mockResolvedValue(preview),confirmFormalOutlineRetire:vi.fn().mockResolvedValue(preview)});
afterEach(cleanup);

it('冻结停用边界，确认后刷新正式树',async()=>{
 const client=api();const refreshed=vi.fn().mockResolvedValue(undefined);
 render(<FormalOutlineRetire api={client} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:2,retiredAt:0}]} readOnly={false} onApplied={refreshed}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 第 2 章起'});
 fireEvent.change(screen.getByLabelText('待停用节点'),{target:{value:'new-asset'}});
 await screen.findByText('未发现直接结构引用；仍须核对未来章节和正式版本。');
 fireEvent.change(screen.getByLabelText('停用生效章节'),{target:{value:'later'}});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:request.reason}});
 fireEvent.click(screen.getByText('预览停用范围'));
 await screen.findByText('“旧目标”从第 2 章起加入，拟从第 3 章起停用。');
 expect(client.confirmFormalOutlineRetire).not.toHaveBeenCalled();
 fireEvent.click(screen.getByText('确认从该章起停用'));
 await waitFor(()=>expect(client.confirmFormalOutlineRetire).toHaveBeenCalledWith({request,digest:'frozen',idempotencyKey:expect.any(String)}));
 await waitFor(()=>expect(refreshed).toHaveBeenCalledTimes(1));
});

it('未确认输入阻止离开，明确取消才关闭',async()=>{
 let guard:(()=>Promise<void>)|undefined;
 render(<FormalOutlineRetire api={api()} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:2,retiredAt:0}]} readOnly={false} registerBeforeLeave={handler=>{guard=handler;return ()=>{guard=undefined};}}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 第 2 章起'});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:'还没确认'}});
 await expect(guard!()).rejects.toThrow('尚未确认');
 fireEvent.keyDown(window,{key:'Escape'});
 expect(screen.getByRole('dialog')).toBeTruthy();
 fireEvent.click(screen.getByText('取消输入并关闭'));
 expect(screen.queryByRole('dialog')).toBeNull();
});

it('首次入档章节点可选择，并清楚显示原始规划边界',async()=>{
 const client=api();
 (client.previewFormalOutlineRetire as ReturnType<typeof vi.fn>).mockResolvedValue({...preview,introducedAt:0});
 render(<FormalOutlineRetire api={client} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:0,retiredAt:0}]} readOnly={false}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 首次入档'});
 fireEvent.change(screen.getByLabelText('待停用节点'),{target:{value:'new-asset'}});
 await screen.findByText('未发现直接结构引用；仍须核对未来章节和正式版本。');
 fireEvent.change(screen.getByLabelText('停用生效章节'),{target:{value:'later'}});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:request.reason}});
 fireEvent.click(screen.getByText('预览停用范围'));
 await screen.findByText('“旧目标”属于首次入档规划，拟从第 3 章起停用。');
});

it('列明直接引用并阻止确认，重新检查后允许预览',async()=>{
 const client=api();let blocked=true;
 (client.formalOutlineReferences as ReturnType<typeof vi.fn>).mockImplementation(async()=>({assetId:'new-asset',nodeId:'new',title:'旧目标',references:blocked?[{kind:'anchor',id:'anchor-1',label:'保持城门目标'}]:[]}));
 render(<FormalOutlineRetire api={client} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:2,retiredAt:0}]} readOnly={false}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 第 2 章起'});
 fireEvent.change(screen.getByLabelText('待停用节点'),{target:{value:'new-asset'}});
 await screen.findByText('发现 1 项引用：');
 expect(screen.getByText('锚点 · 保持城门目标')).toBeTruthy();
 fireEvent.change(screen.getByLabelText('停用生效章节'),{target:{value:'later'}});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:request.reason}});
 expect((screen.getByText('预览停用范围') as HTMLButtonElement).disabled).toBe(true);
 blocked=false;fireEvent.click(screen.getByText('重新检查引用'));
 await screen.findByText('未发现直接结构引用；仍须核对未来章节和正式版本。');
 expect((screen.getByText('预览停用范围') as HTMLButtonElement).disabled).toBe(false);
});

it('唯一故事线链接须明确选择解除，并显示冻结的解除范围',async()=>{
 const client=api();
 (client.formalOutlineReferences as ReturnType<typeof vi.fn>).mockResolvedValue({assetId:'new-asset',nodeId:'new',title:'旧目标',references:[{kind:'story_line',id:'linked-line',label:'主线'}]});
 (client.previewFormalOutlineRetire as ReturnType<typeof vi.fn>).mockResolvedValue({...preview,request:{...request,unlinkStoryLineId:'linked-line'},unlinkedStoryLine:{storyLineId:'linked-line',name:'主线',position:1,createdAt:'2026-01-01T00:00:00Z'}});
 render(<FormalOutlineRetire api={client} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:2,retiredAt:0}]} readOnly={false}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 第 2 章起'});
 fireEvent.change(screen.getByLabelText('待停用节点'),{target:{value:'new-asset'}});
 await screen.findByText('故事线 · 主线');
 fireEvent.change(screen.getByLabelText('停用生效章节'),{target:{value:'later'}});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:request.reason}});
 expect((screen.getByText('预览停用范围') as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByLabelText('同时解除故事线关联'));
 fireEvent.click(screen.getByText('预览停用范围'));
 await screen.findByText('将同时解除“主线”故事线与此节点的关联。');
 expect(client.previewFormalOutlineRetire).toHaveBeenCalledWith('new-asset','later',request.reason,'linked-line',[]);
});

it('多条故事线必须全部选择，预览列明每条解除项',async()=>{
 const client=api();
 (client.formalOutlineReferences as ReturnType<typeof vi.fn>).mockResolvedValue({assetId:'new-asset',nodeId:'new',title:'旧目标',references:[{kind:'story_line',id:'line-a',label:'主线'},{kind:'story_line',id:'line-b',label:'支线'}]});
 (client.previewFormalOutlineRetire as ReturnType<typeof vi.fn>).mockResolvedValue({...preview,request:{...request,unlinkStoryLineIds:['line-a','line-b']},unlinkedStoryLines:[{storyLineId:'line-a',name:'主线',position:1,createdAt:'2026-01-01T00:00:00Z'},{storyLineId:'line-b',name:'支线',position:1,createdAt:'2026-01-01T00:00:00Z'}]});
 render(<FormalOutlineRetire api={client} nodes={[{id:'new',title:'旧目标',level:'chapter',introducedAt:2,retiredAt:0}]} readOnly={false}/>);
 fireEvent.click(screen.getByText('预览未来停用'));
 await screen.findByRole('option',{name:'旧目标 · 第 2 章起'});
 fireEvent.change(screen.getByLabelText('待停用节点'),{target:{value:'new-asset'}});
 await screen.findByText('故事线 · 支线');
 fireEvent.change(screen.getByLabelText('停用生效章节'),{target:{value:'later'}});
 fireEvent.change(screen.getByLabelText('节点停用理由'),{target:{value:request.reason}});
 expect((screen.getByText('预览停用范围') as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByLabelText('同时解除故事线关联'));
 fireEvent.click(screen.getByText('预览停用范围'));
 await screen.findByText('将同时解除“主线”、“支线”故事线与此节点的关联。');
 expect(client.previewFormalOutlineRetire).toHaveBeenCalledWith('new-asset','later',request.reason,'',['line-a','line-b']);
});
