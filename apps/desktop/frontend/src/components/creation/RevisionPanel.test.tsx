import {cleanup,render,screen,fireEvent,waitFor} from '@testing-library/react';
import {afterEach,it,vi,expect} from 'vitest';
import {RevisionPanel,type RevisionAllow,type RevisionCard} from './RevisionPanel';
import type {StructureCollectionView} from '../../api/studio';
afterEach(cleanup);
const editor={candidateId:'c1',version:1,fields:[{path:'/title',label:'标题',group:'方案',value:'旧标题',locked:false}]};
const card:RevisionCard={actionId:'a',argumentsHash:'hash',idempotencyKey:'key',candidateId:'c1',version:1,state:'pending_confirmation',instruction:'简洁一些',targets:editor.fields,preserved:[],cost:{estimatedCalls:2,maxRunTokens:30000,runBudgetUnlimited:false,roleModels:[]},runId:'',quality:'',summary:'',stale:false,canApply:false,applied:false,diffs:[]};
it('预览不触发付费确认，作者须明确确认费用',async()=>{
 const onPreview=vi.fn().mockResolvedValue(undefined),onConfirm=vi.fn().mockResolvedValue(undefined);
 const props={editor,actions:[] as RevisionCard[],onPreview,onConfirm,onReject:vi.fn(),onApply:vi.fn()};
 const {rerender}=render(<RevisionPanel {...props}/>);
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'简洁一些'}});
 fireEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
 await waitFor(()=>expect(onPreview).toHaveBeenCalledWith('c1','/title','简洁一些',expect.any(String),{}));
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
 {path:'/title',label:'标题',group:'方案',kind:'text',before:'旧标题',after:'新标题'},
 {path:'/logline',label:'一句话简介',group:'方案',kind:'text',before:'旧简介',after:'新简介'},
 {path:'/tone',label:'基调',group:'文风',kind:'text',before:'旧基调',after:'新基调'},
]};
const renderSucceeded=(onApply:(action:RevisionCard,paths?:string[])=>Promise<void>,action:RevisionCard=succeeded,collections?:StructureCollectionView[])=>render(<RevisionPanel editor={editor} actions={[action]} structure={collections} onPreview={vi.fn()} onConfirm={vi.fn()} onReject={vi.fn()} onApply={onApply}/>);
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
// 结构授权：只有被勾选的集合才允许 AI 在该次修订内增删实体，未勾选的集合一律不带出。
const structure:StructureCollectionView[]=[
 {collection:'characters',title:'人物',identityField:'id',entities:[{key:'char-1',title:'阿绫'}]},
 {collection:'factions',title:'势力',identityField:'name',entities:[{key:'守军',title:'守军'}]},
];
const renderDraft=(onPreview:(candidateId:string,path:string,instruction:string,key:string,allow:RevisionAllow)=>Promise<void>)=>render(<RevisionPanel editor={editor} actions={[]} structure={structure} onPreview={onPreview} onConfirm={vi.fn()} onReject={vi.fn()} onApply={vi.fn()}/>);
it('全部未勾选时不带结构授权，勾选允许新增后只带该集合',async()=>{
 const onPreview=vi.fn().mockResolvedValue(undefined);
 renderDraft(onPreview);
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'补一个守塔人'}});
 fireEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
 await waitFor(()=>expect(onPreview).toHaveBeenCalledWith('c1','/title','补一个守塔人',expect.any(String),{}));
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'补一个守塔人'}});
 fireEvent.click(screen.getByLabelText('允许新增 人物'));
 fireEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
 await waitFor(()=>expect(onPreview).toHaveBeenCalledWith('c1','/title','补一个守塔人',expect.any(String),{allowAdd:['characters']}));
});
it('勾选允许删除的集合后预览只带 allowRemove',async()=>{
 const onPreview=vi.fn().mockResolvedValue(undefined);
 renderDraft(onPreview);
 fireEvent.change(screen.getByLabelText('修改要求'),{target:{value:'删掉守军'}});
 fireEvent.click(screen.getByLabelText('允许删除 势力'));
 fireEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
 await waitFor(()=>expect(onPreview).toHaveBeenCalledWith('c1','/title','删掉守军',expect.any(String),{allowRemove:['factions']}));
});
it('候选不支持结构增删时不渲染授权小节',()=>{
 render(<RevisionPanel editor={editor} actions={[]} onPreview={vi.fn()} onConfirm={vi.fn()} onReject={vi.fn()} onApply={vi.fn()}/>);
 expect(screen.queryByText('允许 AI 增删实体（可选）')).toBeNull();
 expect(screen.queryByText('允许新增')).toBeNull();
 expect(screen.queryByLabelText('允许新增 人物')).toBeNull();
});
// 结构差异整组结算：界面用标记与实体名称呈现，不做文本前后对比，也不允许只取其中一部分。
// 后端只给文本字段路径标注 label/group，结构差异按集合与实体自行拼出标题。
const withStructure:RevisionCard={...succeeded,diffs:[
 {path:'/characters/@char-2',label:'',group:'',kind:'add_entity',before:'',after:'{"id":"char-2","name":"守塔人","summary":"守塔人接替了守关人的位置"}'},
 {path:'/factions/@守军',label:'',group:'',kind:'remove_entity',before:'{"name":"守军","goal":"守住关隘"}',after:''},
 {path:'/title',label:'标题',group:'方案',kind:'text',before:'旧标题',after:'新标题'},
]};
it('结构差异以新增/删除实体标记与实体名称呈现，不做文本前后对比',()=>{
 renderSucceeded(vi.fn(),withStructure,structure);
 expect(screen.getByText('人物 · 新增 守塔人')).toBeTruthy();
 expect(screen.getByText('势力 · 删除 守军')).toBeTruthy();
 expect(screen.getByText('新增实体')).toBeTruthy();
 expect(screen.getByText('守塔人')).toBeTruthy();
 expect(screen.getByText('删除实体')).toBeTruthy();
 expect(screen.getByText('守军')).toBeTruthy();
 // 三条差异里只有文本差异有前后对比，结构差异按实体内容展示。
 expect(screen.getAllByText('修改前')).toHaveLength(1);
 expect(screen.getAllByText('修改后')).toHaveLength(1);
 expect(screen.getByText(/"summary": "守塔人接替了守关人的位置"/)).toBeTruthy();
 expect(screen.getByText(/"goal": "守住关隘"/)).toBeTruthy();
});
it('勾选集合不含全部结构差异时不能部分采用，必须整组采用',async()=>{
 const onApply=vi.fn().mockResolvedValue(undefined);
 renderSucceeded(onApply,withStructure,structure);
 fireEvent.click(screen.getByLabelText('人物 · 新增 守塔人'));
 expect(screen.getByText('结构增删必须整组采用')).toBeTruthy();
 const partial=screen.getByRole('button',{name:'采用选中的 2 项修改'}) as HTMLButtonElement;
 expect(partial.disabled).toBe(true);
 fireEvent.click(partial);
 expect(onApply).not.toHaveBeenCalled();
 // 整组采用按钮在存在结构差异时始终保持可用。
 expect((screen.getByRole('button',{name:'全部采用'}) as HTMLButtonElement).disabled).toBe(false);
 fireEvent.click(screen.getByLabelText('人物 · 新增 守塔人'));
 expect(screen.queryByText('结构增删必须整组采用')).toBeNull();
 expect((screen.getByRole('button',{name:'采用选中的 3 项修改'}) as HTMLButtonElement).disabled).toBe(false);
});
