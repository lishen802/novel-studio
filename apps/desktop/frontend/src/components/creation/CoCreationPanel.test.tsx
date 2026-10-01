import {cleanup, render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach, expect, it, vi} from 'vitest';
import {CoCreationPanel} from './CoCreationPanel';

afterEach(cleanup);

const editor = {candidateId:'candidate-2',version:2,fields:[
  {path:'/title',label:'标题',group:'定位',value:'旧标题',locked:false},
	{path:'/protagonist_arc',label:'主角成长',group:'结构',value:'成长',locked:false},
  {path:'/ending',label:'结局',group:'结构',value:'结局',locked:true},
]};

it('字段列表为空或缺失时仍渲染对话区', () => {
  render(<CoCreationPanel editor={{candidateId:'',version:0,fields:null as unknown as []}} messages={[]} onSend={vi.fn()} authorizations={null as unknown as []}/>);
  expect(screen.getByLabelText('AI 共创对话')).toBeTruthy();
});

it('提出修改时冻结当前候选、字段和实体引用', async () => {
  const onSend = vi.fn().mockResolvedValue(undefined);
  render(<CoCreationPanel editor={editor} entityRef="arc:3" messages={[]} onSend={onSend}/>);
  await userEvent.type(screen.getByRole('textbox',{name:'共创消息'}),'标题更含蓄一些');
  await userEvent.click(screen.getByRole('button',{name:'提出修改'}));
  expect(onSend).toHaveBeenCalledWith(expect.objectContaining({candidateId:'candidate-2',entityRef:'arc:3',fieldPath:'/title',text:'标题更含蓄一些'}));
  expect(screen.getByRole('option',{name:/结局.*已锁定，可讨论/})).toBeTruthy();
});

it('输入后切换字段必须重新核对草稿目标', async () => {
  const onSend = vi.fn().mockResolvedValue(undefined);
  render(<CoCreationPanel editor={editor} messages={[]} onSend={onSend}/>);
  await userEvent.type(screen.getByRole('textbox',{name:'共创消息'}),'只讨论标题');
  await userEvent.selectOptions(screen.getByRole('combobox',{name:'共创目标字段'}),'/protagonist_arc');
  expect(screen.getByRole('alert').textContent).toContain('字段已变化');
  expect((screen.getByRole('button',{name:'提出修改'}) as HTMLButtonElement).disabled).toBe(true);
});

it('刷新后恢复待确认讨论和原始输入', () => {
  render(<CoCreationPanel
    editor={editor}
    messages={[]}
    onSend={vi.fn()}
    pendingDiscussion={{
      actionId:'pending-a', argumentsHash:'h', idempotencyKey:'k', state:'pending_confirmation', draftMessage:'标题是否准确？',
      authorization:{actionId:'pending-a',state:'pending_confirmation',candidateId:'candidate-2',candidateVersion:2,remainingCalls:8,remainingTokens:16000,maxCalls:8,maxTotalTokens:16000,maxTurnTokens:4000,expiresAt:'2026-09-15T12:00:00Z',provider:'openai',model:'gpt-test',fieldPaths:['/title']},
    }}
  />);
  expect(screen.getByText(/确认讨论授权/)).toBeTruthy();
  expect((screen.getByRole('textbox',{name:'共创消息'}) as HTMLTextAreaElement).value).toBe('标题是否准确？');
});

it('未授权讨论先展示费用预览且关闭后保留输入', async () => {
  const onPreview = vi.fn().mockResolvedValue({actionId:'a1',argumentsHash:'h',idempotencyKey:'k',state:'pending_confirmation',authorization:{actionId:'a1',state:'pending_confirmation',candidateId:'candidate-2',candidateVersion:2,remainingCalls:8,remainingTokens:16000,maxCalls:8,maxTotalTokens:16000,maxTurnTokens:4000,expiresAt:'2026-09-15T12:00:00Z',provider:'openai',model:'gpt-test'}});
  const onConfirm = vi.fn();
  const view = render(<CoCreationPanel editor={editor} messages={[]} onSend={vi.fn()} onPreviewDiscussion={onPreview} onConfirmDiscussion={onConfirm}/>);
  const panel = within(view.container);
  await userEvent.type(panel.getByRole('textbox',{name:'共创消息'}),'动机哪里不够可信？');
  await userEvent.click(panel.getByRole('button',{name:'讨论这个问题'}));
  expect(await screen.findByText(/确认讨论授权/)).toBeTruthy();
  expect(onConfirm).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'取消'}));
  expect((panel.getByRole('textbox',{name:'共创消息'}) as HTMLTextAreaElement).value).toBe('动机哪里不够可信？');
});

it('授权后显示余额，修改建议不会自动执行', async () => {
  const onPreviewSuggestion = vi.fn();
  render(<CoCreationPanel editor={editor} authorization={{id:'auth-1',actionId:'a1',state:'active',candidateId:'candidate-2',candidateVersion:2,remainingCalls:5,remainingTokens:12000,maxCalls:8,maxTotalTokens:16000,maxTurnTokens:4000,expiresAt:'2026-09-15T12:00:00Z',provider:'openai',model:'gpt-test',fieldPaths:['/title']}} messages={[{id:'m1',role:'assistant',kind:'discussion',status:'answered',text:'公开动机缺少代价',stage:'proposal',candidateId:'c1',entityRef:'',fieldPath:'/title',actionId:'a1',actionState:'succeeded',createdAt:'',suggestions:[{id:'s1',paths:['/title'],instruction:'标题更冷',rationale:'定位已成立'}]}]} onSend={vi.fn()} onPreviewDiscussion={vi.fn()} onSendDiscussion={vi.fn()} onPreviewSuggestion={onPreviewSuggestion}/>);
  expect(screen.getByText(/剩余 5 次调用/)).toBeTruthy();
  expect(screen.getByText('公开动机缺少代价')).toBeTruthy();
  expect(onPreviewSuggestion).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'查看修改提案与费用'}));
  expect(onPreviewSuggestion).toHaveBeenCalledWith('m1','s1');
});

it('锁定字段可以讨论，但不能直接提出修改',async()=>{
 const preview=vi.fn().mockResolvedValue({actionId:'a',argumentsHash:'h',idempotencyKey:'k',state:'pending_confirmation'});
 render(<CoCreationPanel editor={editor} messages={[]} onSend={vi.fn()} onPreviewDiscussion={preview}/>);
 await userEvent.selectOptions(screen.getByRole('combobox',{name:'共创目标字段'}),'/ending');
 await userEvent.type(screen.getByRole('textbox',{name:'共创消息'}),'结局是否合适');
 expect((screen.getByRole('button',{name:'提出修改'}) as HTMLButtonElement).disabled).toBe(true);
 await userEvent.click(screen.getByRole('button',{name:'讨论这个问题'}));
 expect(preview).toHaveBeenCalledWith(expect.objectContaining({fieldPaths:['/ending']}));
});
it('整个候选讨论冻结全部读取路径，不扩展为修改授权',async()=>{
 const preview=vi.fn().mockResolvedValue({actionId:'a',argumentsHash:'h',idempotencyKey:'k',state:'pending_confirmation'});
 render(<CoCreationPanel editor={editor} entityRef="arc:3" messages={[]} onSend={vi.fn()} onPreviewDiscussion={preview}/>);
 await userEvent.selectOptions(screen.getByRole('combobox',{name:'共创目标字段'}),'*');
 await userEvent.type(screen.getByRole('textbox',{name:'共创消息'}),'整体节奏是否合理');
 await userEvent.click(screen.getByRole('button',{name:'讨论这个问题'}));
 expect(preview).toHaveBeenCalledWith(expect.objectContaining({entityRef:'',fieldPaths:['/title','/protagonist_arc','/ending']}));
 expect((screen.getByRole('button',{name:'提出修改'}) as HTMLButtonElement).disabled).toBe(true);
});
