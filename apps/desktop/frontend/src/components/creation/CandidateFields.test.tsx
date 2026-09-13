import {cleanup,render,screen,fireEvent,waitFor} from '@testing-library/react';
import {afterEach,it,vi,expect} from 'vitest';
import {CandidateFields} from './CandidateFields';
afterEach(cleanup);
const editor={candidateId:'c1',version:1,fields:[{path:'/characters/@a/secret',label:'秘密',group:'人物 · 阿青',value:'旧秘密',locked:false}]};
it('新版候选到达时保留未保存输入和旧基线，保存冲突不丢文本',async()=>{
 const save=vi.fn().mockRejectedValue(new Error('候选已更新'));const props={editor,onSave:save,onLock:vi.fn()};
 const {rerender}=render(<CandidateFields {...props}/>);fireEvent.change(screen.getByLabelText('候选字段内容'),{target:{value:'作者新秘密'}});
 rerender(<CandidateFields {...props} editor={{...editor,candidateId:'c2',version:2}}/>);
 fireEvent.click(screen.getByRole('button',{name:'保存文本修改',hidden:true}));await waitFor(()=>expect(save).toHaveBeenCalledWith('c1',{'/characters/@a/secret':'作者新秘密'}));await screen.findByRole('alert');expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).value).toBe('作者新秘密');
});
it('锁定与保存独立，锁定文本不可改写',()=>{render(<CandidateFields editor={{...editor,fields:[{...editor.fields[0],locked:true}]}} onSave={vi.fn()} onLock={vi.fn()}/>);expect((screen.getByLabelText('候选字段内容') as HTMLTextAreaElement).disabled).toBe(true);});
