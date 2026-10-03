import {cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import {afterEach,expect,it,vi} from 'vitest';
import {ConstraintChanges,type ConstraintAPI} from './ConstraintChanges';
import type {ConstraintPreview,ConstraintRequest} from '../api/studio';
afterEach(cleanup);
function api():ConstraintAPI {
 return {
  constraints:vi.fn().mockResolvedValue([{assetId:'a',kind:'world_rule',name:'潮汐',original:'原规则',current:'原规则',revision:1,locked:false,changes:[]}]),
  listChapters:vi.fn().mockResolvedValue([{id:'c',title:'后续章节',position:2,body:'',revision:1}]),
  previewConstraintChange:vi.fn(async(request:ConstraintRequest):Promise<ConstraintPreview>=>({request:{...request,projectId:'p'},digest:'frozen',before:'原规则',revision:2,position:2,protectedChapters:1,futureChapters:1,canApply:true,blockedReason:''})),
  applyConstraintChange:vi.fn().mockRejectedValue(new Error('已有正文，重新预览')),
 };
}
async function edit(){
 fireEvent.click(await screen.findByRole('button',{name:'修订生效范围'}));
 fireEvent.change(screen.getByLabelText('新语句'),{target:{value:'后续规则'}});
 fireEvent.change(screen.getByLabelText('修订理由'),{target:{value:'路线调整'}});
 fireEvent.change(screen.getByLabelText('生效章节'),{target:{value:'c'}});
 fireEvent.click(screen.getByRole('button',{name:'预览影响'}));
 await waitFor(()=>expect((screen.getByRole('button',{name:'确认从该章起生效'}) as HTMLButtonElement).disabled).toBe(false));
}
it('失败保留输入和离开保护，修改表单使预览失效',async()=>{
 const calls=api();let guard:(()=>Promise<void>)|undefined;
 render(<ConstraintChanges api={calls} registerBeforeLeave={fn=>{guard=fn;return ()=>{guard=undefined;};}}/>);
 await edit();await expect(guard?.()).rejects.toThrow('尚未确认');
 fireEvent.click(screen.getByRole('button',{name:'确认从该章起生效'}));await screen.findByRole('alert');
 expect((screen.getByLabelText('新语句') as HTMLTextAreaElement).value).toBe('后续规则');
 expect(calls.applyConstraintChange).toHaveBeenCalledWith(expect.objectContaining({digest:'frozen',request:expect.objectContaining({projectId:'p',text:'后续规则'})}));
 fireEvent.change(screen.getByLabelText('新语句'),{target:{value:'再次修改'}});
 expect((screen.getByRole('button',{name:'确认从该章起生效'}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.keyDown(window,{key:'Escape'});expect(screen.getByRole('dialog')).not.toBeNull();expect((screen.getByLabelText('新语句') as HTMLTextAreaElement).value).toBe('再次修改');
 fireEvent.click(screen.getByRole('button',{name:'取消修订'}));await waitFor(()=>expect(guard).toBeUndefined());
});
it('边界有正文时展示阻止原因且不能确认',async()=>{
 const calls=api();calls.previewConstraintChange=vi.fn(async(request)=>({request,digest:'x',before:'原规则',revision:2,position:2,protectedChapters:1,futureChapters:1,canApply:false,blockedReason:'该章已有恢复草稿'}));
 render(<ConstraintChanges api={calls}/>);
 fireEvent.click(await screen.findByRole('button',{name:'修订生效范围'}));fireEvent.change(screen.getByLabelText('新语句'),{target:{value:'新规则'}});fireEvent.change(screen.getByLabelText('修订理由'),{target:{value:'说明'}});fireEvent.change(screen.getByLabelText('生效章节'),{target:{value:'c'}});fireEvent.click(screen.getByRole('button',{name:'预览影响'}));
 await screen.findByText('该章已有恢复草稿');expect((screen.getByRole('button',{name:'确认从该章起生效'}) as HTMLButtonElement).disabled).toBe(true);expect(calls.applyConstraintChange).not.toHaveBeenCalled();
});
it('确认成功展示回执，保留原版且解除离开保护',async()=>{
 const calls=api();calls.applyConstraintChange=vi.fn().mockResolvedValue({});let registered=false;
 render(<ConstraintChanges api={calls} registerBeforeLeave={()=>{registered=true;return ()=>{registered=false;};}}/>);await edit();fireEvent.click(screen.getByRole('button',{name:'确认从该章起生效'}));await screen.findByRole('status');await waitFor(()=>expect(registered).toBe(false));expect(screen.queryByRole('dialog')).toBeNull();
});
