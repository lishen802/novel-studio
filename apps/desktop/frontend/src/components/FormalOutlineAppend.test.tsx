import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { FormalOutlineAppend, type FormalOutlineAppendAPI } from './FormalOutlineAppend';
import type { ConstraintView, FormalOutlineAppendPreview } from '../api/studio';

const request={projectId:'project',branchId:'branch',parentAssetId:'arc-asset',chapterId:'future',title:'下一章目标',summary:'',goal:'',conflict:'',outcome:'',reason:'补足后续章节'};
const preview:FormalOutlineAppendPreview={request,digest:'frozen',position:4,nodePosition:3,parentTitle:'守关'};
const api=():FormalOutlineAppendAPI=>({listChapters:vi.fn().mockResolvedValue([{id:'future',projectId:'project',title:'第四章',position:4,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}]),planningRevisions:vi.fn().mockResolvedValue([{assetId:'arc-asset',kind:'outline_node',typedId:'arc'} as ConstraintView]),previewFormalOutlineAppend:vi.fn().mockResolvedValue(preview),confirmFormalOutlineAppend:vi.fn().mockResolvedValue({...preview,nodeId:'new',assetId:'new-asset'})});
afterEach(cleanup);

it('预览冻结故事弧和章节边界，确认后刷新正式树',async()=>{
 const client=api();const refreshed=vi.fn().mockResolvedValue(undefined);
 render(<FormalOutlineAppend api={client} arcs={[{id:'arc',title:'守关'}]} readOnly={false} onApplied={refreshed}/>);
 fireEvent.click(screen.getByText('新增未来章节目标'));
 await screen.findByRole('option',{name:'守关'});
 fireEvent.change(screen.getByLabelText('所属故事弧'),{target:{value:'arc-asset'}});
 fireEvent.change(screen.getByLabelText('未来节点标题'),{target:{value:'下一章目标'}});
 fireEvent.change(screen.getByLabelText('新增节点理由'),{target:{value:'补足后续章节'}});
 fireEvent.click(screen.getByText('预览新增范围'));
 await screen.findByText('在“守关”末尾新增第 3 项：下一章目标');
 expect(client.confirmFormalOutlineAppend).not.toHaveBeenCalled();
 fireEvent.click(screen.getByText('确认新增正式节点'));
 await waitFor(()=>expect(client.confirmFormalOutlineAppend).toHaveBeenCalledWith({request,digest:'frozen',idempotencyKey:expect.any(String)}));
 await waitFor(()=>expect(refreshed).toHaveBeenCalledTimes(1));
 await screen.findByText('“下一章目标”已从第 4 章起加入正式规划。');
});

it('未确认输入阻止离开，明确取消才关闭',async()=>{
 const client=api();let guard:(()=>Promise<void>)|undefined;
 render(<FormalOutlineAppend api={client} arcs={[{id:'arc',title:'守关'}]} readOnly={false} registerBeforeLeave={handler=>{guard=handler;return ()=>{guard=undefined};}}/>);
 fireEvent.click(screen.getByText('新增未来章节目标'));
 await screen.findByRole('option',{name:'守关'});
 fireEvent.change(screen.getByLabelText('未来节点标题'),{target:{value:'尚未保存'}});
 await expect(guard!()).rejects.toThrow('尚未确认');
 fireEvent.keyDown(window,{key:'Escape'});
 expect(screen.getByRole('dialog')).toBeTruthy();
 fireEvent.click(screen.getByText('取消输入并关闭'));
 expect(screen.queryByRole('dialog')).toBeNull();
});
