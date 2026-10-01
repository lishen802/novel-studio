import {cleanup,render,screen,fireEvent,waitFor} from '@testing-library/react';
import {afterEach,it,vi,expect} from 'vitest';
import {CandidateFields} from './CandidateFields';
afterEach(cleanup);
const editor={candidateId:'c1',version:1,fields:[{path:'/characters/@a/secret',label:'秘密',group:'人物 · 阿青',value:'旧秘密',locked:false}]};
it('新版候选到达时保留未保存输入和旧基线，保存冲突不丢文本',async()=>{
 const save=vi.fn().mockRejectedValue(new Error('候选已更新'));const props={editor,onSave:save,onLock:vi.fn()};
 const {rerender}=render(<CandidateFields {...props}/>);fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'作者新秘密'}});
 rerender(<CandidateFields {...props} editor={{...editor,candidateId:'c2',version:2}}/>);
 fireEvent.click(screen.getByRole('button',{name:'保存文本修改',hidden:true}));await waitFor(()=>expect(save).toHaveBeenCalledWith('c1',{'/characters/@a/secret':'作者新秘密'}));await screen.findByText(/候选已更新/);expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('作者新秘密');
});
it('锁定与保存独立，锁定文本不可改写',()=>{render(<CandidateFields editor={{...editor,fields:[{...editor.fields[0],locked:true}]}} onSave={vi.fn()} onLock={vi.fn()}/>);expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).disabled).toBe(true);});
it('切换同名对象保留各自修改，只提交发生变化的字段',async()=>{
 const fields=[{path:'/characters/@a/secret',ownerRef:'character:a',label:'秘密',group:'人物 · 同名',value:'甲',locked:false},{path:'/characters/@b/secret',ownerRef:'character:b',label:'秘密',group:'人物 · 同名',value:'乙',locked:false}];
 const save=vi.fn().mockResolvedValue(undefined);const props={editor:{...editor,fields},onSave:save,onLock:vi.fn(),targetLabel:'同名'};
 const {rerender}=render(<CandidateFields {...props} ownerRef="character:a"/>);
 fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'甲修改'}});
 rerender(<CandidateFields {...props} ownerRef="character:b"/>);
 expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('乙');
 expect(screen.getByText(/另有 1 处修改/)).toBeTruthy();
 fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'乙修改'}});
 rerender(<CandidateFields {...props} ownerRef="character:a"/>);
 expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('甲修改');
 fireEvent.click(screen.getByRole('button',{name:'保存文本修改'}));
 await waitFor(()=>expect(save).toHaveBeenCalledWith('c1',{'/characters/@a/secret':'甲修改','/characters/@b/secret':'乙修改'}));
});
it('改回原值不再产生无意义新版本',()=>{
 const dirty=vi.fn();render(<CandidateFields editor={editor} onSave={vi.fn()} onLock={vi.fn()} onDirtyChange={dirty}/>);
 fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'修改'}});fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'旧秘密'}});
 expect((screen.getByRole('button',{name:'保存文本修改'}) as HTMLButtonElement).disabled).toBe(true);expect(dirty).toHaveBeenLastCalledWith(false);
});
it('离开前保存复用正在进行的请求，失败后保留输入并阻止离开',async()=>{
 let flush:()=>Promise<void>=async()=>{};
 let reject!:(reason:Error)=>void;
 const save=vi.fn(()=>new Promise<void>((_,fail)=>{reject=fail;}));
 render(<CandidateFields editor={editor} onSave={save} onLock={vi.fn()} registerBeforeLeave={handler=>{flush=handler;return ()=>{};}}/>);
 fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'不能丢失'}});
 fireEvent.click(screen.getByRole('button',{name:'保存文本修改'}));
 const leaving=flush();const failure=expect(leaving).rejects.toThrow('磁盘满');
 await waitFor(()=>expect(save).toHaveBeenCalledTimes(1));reject(new Error('磁盘满'));await failure;
 await screen.findByText(/磁盘满/);expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('不能丢失');
});
