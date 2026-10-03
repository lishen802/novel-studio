import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { FormalCharacterAI, type FormalCharacterAPI } from './FormalCharacterAI';
import type { ConstraintView } from '../api/studio';
import type { RevisionCard } from './creation/RevisionPanel';
const item: ConstraintView = { assetId:'a',typedId:'hero',kind:'character',name:'旧姓名',original:'{}',current:JSON.stringify({name:'新姓名',motivation:'作者最新动机'}),revision:3,locked:false,changes:[] };
const action: RevisionCard = { formalAssetId:'a',formalRevision:3,actionId:'action',argumentsHash:'digest',idempotencyKey:'key',candidateId:'',version:3,state:'pending_confirmation',instruction:'补足代价',targets:[{path:'/characters/@hero/motivation',ownerRef:'hero',label:'动机',group:'人物',value:'作者最新动机',locked:false}],preserved:[],cost:{estimatedCalls:2,maxRunTokens:30000,runBudgetUnlimited:false,roleModels:[{role:'planner',provider:'P1',model:'writer'},{role:'continuity_reviewer',provider:'P2',model:'review'}]},runId:'',quality:'',summary:'',stale:false,canApply:false,applied:false,diffs:[] };
function setup():FormalCharacterAPI {return {previewCharacterRevision:vi.fn().mockResolvedValue(action),characterRevisionActions:vi.fn().mockResolvedValue([]),confirmCreationRevision:vi.fn().mockResolvedValue({...action,state:'executing'}),rejectCreationRevision:vi.fn().mockResolvedValue({})};}
function open(api:FormalCharacterAPI,props:Partial<Parameters<typeof FormalCharacterAI>[0]>={}) { render(<FormalCharacterAI api={api} items={[item]} readOnly={false} onReady={vi.fn().mockResolvedValue(undefined)} {...props}/>);fireEvent.click(screen.getByText('让 AI 修改正式人物')); }
afterEach(cleanup);
it('从最新正式版本预览费用，确认后才调用生成',async()=>{
 const api=setup();open(api);expect(screen.getByText('当前正式原文：作者最新动机')).toBeTruthy();
 fireEvent.change(screen.getByLabelText('正式人物 AI 修改要求'),{target:{value:'补足代价'}});fireEvent.click(screen.getByText('预览人物 AI 修改费用'));
 await screen.findByText('待确认费用 · 基准版本 3');expect(api.confirmCreationRevision).not.toHaveBeenCalled();
 expect(vi.mocked(api.previewCharacterRevision!).mock.calls[0][0]).toMatchObject({assetId:'a',expectedRevision:3,field:'motivation',instruction:'补足代价'});
 expect(screen.getByText('独立审阅：P2 / review')).toBeTruthy();fireEvent.click(screen.getByText('确认费用并生成候选'));
 await screen.findByText('正在修改与独立审阅 · 基准版本 3');expect(api.confirmCreationRevision).toHaveBeenCalledWith({actionId:'action',argumentsHash:'digest',idempotencyKey:'key'});
});
it('失败保留要求与预览幂等键，重试不换参数',async()=>{
 const api=setup();vi.mocked(api.previewCharacterRevision!).mockRejectedValueOnce(new Error('连接中断')).mockResolvedValueOnce(action);open(api);
 fireEvent.change(screen.getByLabelText('正式人物 AI 修改要求'),{target:{value:'补足代价'}});fireEvent.click(screen.getByText('预览人物 AI 修改费用'));await screen.findByText('连接中断');
 expect((screen.getByLabelText('正式人物 AI 修改要求') as HTMLTextAreaElement).value).toBe('补足代价');fireEvent.click(screen.getByText('预览人物 AI 修改费用'));await screen.findByText('待确认费用 · 基准版本 3');
 const calls=vi.mocked(api.previewCharacterRevision!).mock.calls;expect(calls[0][0]).toEqual(calls[1][0]);
});
it('未处理要求阻止 Esc 和导航，明确取消解除保护',async()=>{
 const api=setup();let guard:(()=>Promise<void>)|undefined;const unregister=vi.fn();open(api,{registerBeforeLeave:(handler)=>{guard=handler;return unregister;}});
 fireEvent.change(screen.getByLabelText('正式人物 AI 修改要求'),{target:{value:'保留人物性格'}});fireEvent.keyDown(window,{key:'Escape'});
 expect(screen.getByRole('dialog')).toBeTruthy();await expect(guard!()).rejects.toThrow('人物 AI 修改要求尚未处理');fireEvent.click(screen.getByText('取消输入并关闭'));expect(screen.queryByRole('dialog')).toBeNull();expect(unregister).toHaveBeenCalled();
});
it('恢复过期基线不允许付费确认，只读不能启动',async()=>{
 const api=setup();vi.mocked(api.characterRevisionActions!).mockResolvedValue([{...action,stale:true}]);open(api);await screen.findByText('正式版本或保护输入已变化，此提案/结果不再适用，请重新提出修改。');
 expect((screen.getByText('确认费用并生成候选') as HTMLButtonElement).disabled).toBe(true);cleanup();render(<FormalCharacterAI api={api} items={[item]} readOnly onReady={vi.fn()}/>);
 await waitFor(()=>expect((screen.getByText('让 AI 修改正式人物') as HTMLButtonElement).disabled).toBe(true));expect(api.confirmCreationRevision).not.toHaveBeenCalled();
});
