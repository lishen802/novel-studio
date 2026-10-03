import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { PlanningRearchive, type PlanningRearchiveAPI } from './PlanningRearchive';
import type { PlanningRearchivePreview } from '../api/studio';

const preview:PlanningRearchivePreview={request:{projectId:'project',branchId:'branch',chapterId:'future',reason:'调整后续选择'},digest:'digest',position:4,protectedChapters:3,futureChapters:1,changes:[{assetId:'book',kind:'outline_node',before:JSON.stringify({'/title':'潮汐关','/goal':'旧目标'}),after:JSON.stringify({'/title':'潮汐关','/goal':'新目标'})}]};
const api=():PlanningRearchiveAPI=>({listChapters:vi.fn().mockResolvedValue([{id:'future',projectId:'project',title:'第四章',position:4,body:'',revision:1,updatedAt:'',hasRecoveryDraft:false}]),previewPlanningRearchive:vi.fn().mockResolvedValue(preview),confirmPlanningRearchive:vi.fn().mockResolvedValue(preview)});
afterEach(cleanup);

it('先展示真实差异，再用冻结摘要确认生效章节',async()=>{
 const client=api();render(<PlanningRearchive api={client} readOnly={false}/>);fireEvent.click(screen.getByText('预览再次入档'));
 await screen.findByRole('option',{name:'第 4 章 · 第四章'});fireEvent.change(screen.getByLabelText('修改理由'),{target:{value:'调整后续选择'}});fireEvent.click(screen.getByText('预览整组差异'));
 await screen.findByText('目标：新目标');expect(client.confirmPlanningRearchive).not.toHaveBeenCalled();
 fireEvent.click(screen.getByText('确认从该章起入档'));await waitFor(()=>expect(client.confirmPlanningRearchive).toHaveBeenCalledTimes(1));
 expect(vi.mocked(client.confirmPlanningRearchive).mock.calls[0][0]).toMatchObject({request:preview.request,digest:'digest'});
 await screen.findByText('规划候选已从第 4 章起入档，更新 1 项。');
});

it('失败和导航保留理由；明确取消后才能关闭',async()=>{
 const client=api();vi.mocked(client.previewPlanningRearchive).mockRejectedValueOnce(new Error('版本冲突'));
 let guard:(()=>Promise<void>)|undefined;render(<PlanningRearchive api={client} readOnly={false} registerBeforeLeave={handler=>{guard=handler;return ()=>{guard=undefined};}}/>);
 fireEvent.click(screen.getByText('预览再次入档'));await screen.findByRole('option',{name:'第 4 章 · 第四章'});
 fireEvent.change(screen.getByLabelText('修改理由'),{target:{value:'调整后续选择'}});fireEvent.click(screen.getByText('预览整组差异'));await screen.findByText('版本冲突');
 expect((screen.getByLabelText('修改理由') as HTMLTextAreaElement).value).toBe('调整后续选择');await expect(guard!()).rejects.toThrow('尚未确认');
 fireEvent.keyDown(window,{key:'Escape'});expect(screen.getByRole('dialog')).toBeTruthy();fireEvent.click(screen.getByText('取消输入并关闭'));expect(screen.queryByRole('dialog')).toBeNull();
});
