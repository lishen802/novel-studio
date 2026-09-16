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
// 修订结果的部分采用：勾选只影响回传的路径子集，未勾选的差异由后端保持基线原文。
const succeeded:RevisionCard={...card,state:'succeeded',quality:'passed',canApply:true,diffs:[
 {path:'/title',label:'标题',group:'方案',before:'旧标题',after:'新标题'},
 {path:'/logline',label:'一句话简介',group:'方案',before:'旧简介',after:'新简介'},
 {path:'/tone',label:'基调',group:'文风',before:'旧基调',after:'新基调'},
]};
const renderSucceeded=(onApply:(action:RevisionCard,paths?:string[])=>Promise<void>,action:RevisionCard=succeeded)=>render(<RevisionPanel editor={editor} actions={[action]} onPreview={vi.fn()} onConfirm={vi.fn()} onReject={vi.fn()} onApply={onApply}/>);
it('只采用勾选的差异时按界面顺序回传所选路径',async()=>{
 const onApply=vi.fn().mockResolvedValue(undefined);
 renderSucceeded(onApply);
 fireEvent.click(screen.getByLabelText('方案 · 一句话简介'));
 expect(screen.getByText('只采用选中的修改会生成待复核的新候选，其余字段保持基线原文。')).toBeTruthy();
 fireEvent.click(screen.getByRole('button',{name:'采用选中的 2 项修改'}));
 await waitFor(()=>expect(onApply).toHaveBeenCalledWith(succeeded,['/title','/tone']));
});
it('全部采用不带路径，未勾选的差异也按整组提交',async()=>{
 const onApply=vi.fn().mockResolvedValue(undefined);
 renderSucceeded(onApply);
 fireEvent.click(screen.getByLabelText('文风 · 基调'));
 fireEvent.click(screen.getByRole('button',{name:'全部采用'}));
 await waitFor(()=>expect(onApply).toHaveBeenCalledWith(succeeded));
 expect(onApply.mock.calls[0][1]).toBeUndefined();
});
it('取消全部勾选后部分采用不可用，也不会发起调用',()=>{
 const onApply=vi.fn().mockResolvedValue(undefined);
 renderSucceeded(onApply);
 ['方案 · 标题','方案 · 一句话简介','文风 · 基调'].forEach(name=>fireEvent.click(screen.getByLabelText(name)));
 expect(screen.queryAllByRole('checkbox').every(node=>(node as HTMLInputElement).checked)).toBe(false);
 const button=screen.getByRole('button',{name:'采用选中的 0 项修改'}) as HTMLButtonElement;
 expect(button.disabled).toBe(true);
 fireEvent.click(button);
 expect(onApply).not.toHaveBeenCalled();
});
it('部分采用被拒绝时展示错误并保留勾选状态',async()=>{
 const onApply=vi.fn().mockRejectedValue(new Error('基线已变化，无法采用这组修改。'));
 renderSucceeded(onApply);
 fireEvent.click(screen.getByLabelText('方案 · 一句话简介'));
 fireEvent.click(screen.getByRole('button',{name:'采用选中的 2 项修改'}));
 await waitFor(()=>expect(screen.getByRole('alert').textContent).toContain('基线已变化'));
 expect((screen.getByLabelText('方案 · 一句话简介') as HTMLInputElement).checked).toBe(false);
 expect((screen.getByLabelText('方案 · 标题') as HTMLInputElement).checked).toBe(true);
 expect((screen.getByRole('button',{name:'采用选中的 2 项修改'}) as HTMLButtonElement).disabled).toBe(false);
});
it('审阅未通过时保留禁用入口，不提供可用的采用动作',()=>{
 const onApply=vi.fn().mockResolvedValue(undefined);
 renderSucceeded(onApply,{...succeeded,quality:'blocked',canApply:false});
 expect((screen.getByRole('button',{name:'审阅未通过，暂不可采用'}) as HTMLButtonElement).disabled).toBe(true);
 expect(screen.queryByRole('button',{name:'全部采用'})).toBeNull();
 expect(screen.queryByRole('button',{name:/采用选中的/})).toBeNull();
 expect(screen.queryAllByRole('checkbox').every(node=>(node as HTMLInputElement).disabled)).toBe(true);
});
