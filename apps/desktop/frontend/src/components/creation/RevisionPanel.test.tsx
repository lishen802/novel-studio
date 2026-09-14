import {cleanup,render,screen,fireEvent,waitFor} from '@testing-library/react';
import {afterEach,it,vi,expect} from 'vitest';
import {RevisionPanel,type RevisionCard} from './RevisionPanel';
afterEach(cleanup);
const editor={candidateId:'c1',version:1,fields:[{path:'/title',label:'标题',group:'方案',value:'旧标题',locked:false}]};
const card:RevisionCard={actionId:'a',argumentsHash:'hash',idempotencyKey:'key',candidateId:'c1',version:1,state:'pending_confirmation',instruction:'简洁一些',targets:editor.fields,preserved:[],cost:{estimatedCalls:2,maxRunTokens:30000,runBudgetUnlimited:false,roleModels:[]},runId:'',quality:'',summary:'',stale:false,canApply:false,applied:false,diffs:[]};
it('预览不触发付费确认，作者须明确确认费用',async()=>{
 const onPreview=vi.fn().mockResolvedValue(undefined),onConfirm=vi.fn().mockResolvedValue(undefined);
 const props={editor,actions:[] as RevisionCard[],onPreview,onConfirm,onReject:vi.fn(),onApply:vi.fn()};
 const {rerender}=render(<RevisionPanel {...props}/>);
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'简洁一些'}});
 fireEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
 await waitFor(()=>expect(onPreview).toHaveBeenCalledWith('c1','/title','简洁一些',expect.any(String)));
 expect(onConfirm).not.toHaveBeenCalled();
 rerender(<RevisionPanel {...props} actions={[card]}/>);
 await waitFor(()=>expect((screen.getByRole('button',{name:'确认费用并开始修改'}) as HTMLButtonElement).disabled).toBe(false));
 fireEvent.click(screen.getByRole('button',{name:'确认费用并开始修改'}));await waitFor(()=>expect(onConfirm).toHaveBeenCalledWith(card));
});
it('候选变化保留输入，过期基线与未通过结果不能确认或采用',()=>{
 const props={editor,actions:[{...card,stale:true},{...card,actionId:'b',state:'succeeded',quality:'blocked'}],onPreview:vi.fn(),onConfirm:vi.fn(),onReject:vi.fn(),onApply:vi.fn()};
 const {rerender}=render(<RevisionPanel {...props}/>);
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'保留这份输入'}});
 rerender(<RevisionPanel {...props} editor={{...editor,candidateId:'c2'}}/>);
 expect((screen.getByLabelText('修改要求') as HTMLTextAreaElement).value).toBe('保留这份输入');
 expect(screen.getByRole('alert').textContent).toContain('候选已更新');
 expect((screen.getByRole('button',{name:'确认费用并开始修改'}) as HTMLButtonElement).disabled).toBe(true);
 expect((screen.getByRole('button',{name:'审阅未通过，暂不可采用'}) as HTMLButtonElement).disabled).toBe(true);
});
