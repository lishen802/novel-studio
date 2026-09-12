import { cleanup,render,screen,fireEvent,waitFor } from '@testing-library/react';
import { afterEach,it,expect,vi } from 'vitest';
import { SamplePanel } from './SamplePanel';
afterEach(cleanup);
it('保留编辑开始时的来源版本，冲突不丢输入，生成始终未就绪',async()=>{
 const save=vi.fn().mockRejectedValue(new Error('来源已变化'));
 const props={version:2,runVersion:0,directionCandidateId:'c1',directionCardId:'d1',onSave:save};
 const {rerender}=render(<SamplePanel {...props}/>);
 fireEvent.change(screen.getByLabelText('试写语气'),{target:{value:'明快而诙谐'}});
 rerender(<SamplePanel {...props} version={3} directionCandidateId="c2"/>);
 fireEvent.click(screen.getByRole('button',{name:'保存试写准备'}));
 await waitFor(()=>expect(save).toHaveBeenCalledWith(expect.objectContaining({expectedVersion:2,directionCandidateId:'c1',tone:'明快而诙谐',targetRunes:1200})));
 await screen.findByRole('alert');
 expect((screen.getByLabelText('试写语气') as HTMLTextAreaElement).value).toBe('明快而诙谐');
 expect((screen.getByRole('button',{name:'生成试写（尚未就绪）'}) as HTMLButtonElement).disabled).toBe(true);
});
