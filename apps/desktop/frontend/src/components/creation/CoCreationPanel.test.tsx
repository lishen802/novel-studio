import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { CoCreationPanel } from './CoCreationPanel';

it('发送时冻结当前候选、字段和实体引用', async () => {
  const onSend = vi.fn().mockResolvedValue(undefined);
  render(<CoCreationPanel editor={{candidateId:'candidate-2',version:2,fields:[
    {path:'/title',label:'标题',group:'定位',value:'旧标题',locked:false},
    {path:'/ending',label:'结局',group:'结构',value:'结局',locked:true},
  ]}} entityRef="arc:3" messages={[]} onSend={onSend}/>);
  await userEvent.type(screen.getByRole('textbox',{name:'共创消息'}),'标题更含蓄一些');
  await userEvent.click(screen.getByRole('button',{name:'发送并查看修改提案'}));
  expect(onSend).toHaveBeenCalledWith(expect.objectContaining({candidateId:'candidate-2',entityRef:'arc:3',fieldPath:'/title',text:'标题更含蓄一些'}));
  expect(screen.queryByRole('option',{name:/结局/})).toBeNull();
});

it('恢复消息及其关联动作状态', () => {
  render(<CoCreationPanel messages={[{id:'m1',role:'assistant',text:'提案已整理',stage:'proposal',candidateId:'c1',entityRef:'',fieldPath:'/title',actionId:'a1',actionState:'pending_confirmation',createdAt:''}]} onSend={vi.fn()}/>);
  expect(screen.getByText('提案已整理')).toBeTruthy();
  expect(screen.getByText(/修改提案待确认/)).toBeTruthy();
});
